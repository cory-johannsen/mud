package gameserver

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"

	"github.com/cory-johannsen/mud/internal/game/inventory"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var merchantRuntimeMu sync.RWMutex

// initMerchantRuntimeState initialises runtime state for a merchant instance if absent.
//
// Precondition: inst must be non-nil.
// Postcondition: merchantRuntimeStates[inst.ID] is set iff inst.NPCType == "merchant" and template is found.
func (s *GameServiceServer) initMerchantRuntimeState(inst *npc.Instance) {
	if inst.NPCType != "merchant" {
		return
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Merchant == nil {
		return
	}
	merchantRuntimeMu.Lock()
	defer merchantRuntimeMu.Unlock()
	if _, ok := s.merchantRuntimeStates[inst.ID]; !ok {
		s.merchantRuntimeStates[inst.ID] = npc.InitRuntimeState(tmpl.Merchant, time.Now())
	}
}

// merchantStateFor returns the MerchantRuntimeState for instID, or nil if absent.
//
// Precondition: none.
// Postcondition: Returns nil when instID is not in merchantRuntimeStates.
func (s *GameServiceServer) merchantStateFor(instID string) *npc.MerchantRuntimeState {
	merchantRuntimeMu.RLock()
	defer merchantRuntimeMu.RUnlock()
	return s.merchantRuntimeStates[instID]
}

// findMerchantInRoom locates a merchant NPC by name in roomID.
//
// Precondition: roomID and npcName are non-empty.
// Postcondition: Returns (inst, "") on success; (nil, errMsg) on failure.
func (s *GameServiceServer) findMerchantInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("You don't see %q here.", npcName)
	}
	if inst.NPCType != "merchant" {
		return nil, fmt.Sprintf("%s is not a merchant.", inst.Name())
	}
	if inst.Cowering {
		return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
	}
	return inst, ""
}

// normalizeMerchantQuery lowercases s and replaces spaces and hyphens with underscores.
//
// Precondition: none.
// Postcondition: Returns a lowercase string with spaces/hyphens replaced by underscores.
func normalizeMerchantQuery(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	s = strings.ReplaceAll(s, "-", "_")
	return s
}

// wantedSurchargeFor returns 1.1 if the player has WantedLevel >= 1 in the room's zone, else 1.0.
//
// Precondition: sess is non-nil.
// Postcondition: Returns 1.1 or 1.0.
func (s *GameServiceServer) wantedSurchargeFor(sess *session.PlayerSession, _ *npc.Instance) float64 {
	if room, ok := s.world.GetRoom(sess.RoomID); ok {
		if wl, exists := sess.WantedLevel[room.ZoneID]; exists && wl >= 1 {
			return 1.1
		}
	}
	return 1.0
}

// buildShopView constructs a ShopView ServerEvent for the named merchant NPC in the player's room.
//
// Precondition: uid identifies an active player session; npcName is non-empty.
// Postcondition: Returns a non-nil ServerEvent on success; (nil, nil) when the merchant is absent or has no inventory.
func (s *GameServiceServer) buildShopView(uid string, npcName string) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return nil, nil
	}
	inst, _ := s.findMerchantInRoom(sess.RoomID, npcName)
	if inst == nil {
		return nil, nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Merchant == nil {
		return nil, nil
	}
	state := s.merchantStateFor(inst.ID)
	if state == nil {
		s.initMerchantRuntimeState(inst)
		state = s.merchantStateFor(inst.ID)
	}
	surcharge := s.wantedSurchargeFor(sess, inst)
	rows := npc.BrowseLines(tmpl.Merchant, state, surcharge, sess.NegotiateModifier)
	items := make([]*gamev1.ShopItem, 0, len(rows))
	for _, row := range rows {
		shopItem := &gamev1.ShopItem{
			ItemId:    row.ItemID,
			BuyPrice:  int32(row.BuyPrice),
			SellPrice: int32(row.SellPrice),
			Stock:     int32(row.Stock),
			Name:      row.ItemID,
		}
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(row.ItemID); ok {
				shopItem.Name = def.Name
				shopItem.Kind = def.Kind
				shopItem.Description = def.Description
				if def.WeaponRef != "" {
					if wpn := s.invRegistry.Weapon(def.WeaponRef); wpn != nil {
						shopItem.WeaponDamage = wpn.DamageDice
						shopItem.WeaponDamageType = wpn.DamageType
						shopItem.WeaponRange = int32(wpn.RangeIncrement)
						shopItem.WeaponTraits = wpn.Traits
					}
				}
				if def.ArmorRef != "" {
					if arm, ok := s.invRegistry.Armor(def.ArmorRef); ok {
						shopItem.ArmorAcBonus = int32(arm.ACBonus)
						shopItem.ArmorSlot = string(arm.Slot)
						shopItem.ArmorCheckPenalty = int32(arm.CheckPenalty)
						shopItem.ArmorSpeedPenalty = int32(arm.SpeedPenalty)
						shopItem.ArmorProfCategory = arm.ProficiencyCategory
					}
				}
				if def.Kind == inventory.KindConsumable {
					shopItem.EffectsSummary = buildConsumableEffectsSummary(def)
				}
			}
		}
		items = append(items, shopItem)
	}
	return &gamev1.ServerEvent{
		Payload: &gamev1.ServerEvent_ShopView{
			ShopView: &gamev1.ShopView{
				NpcName: inst.Name(),
				Items:   items,
			},
		},
	}, nil
}

// handleBrowse lists a merchant's inventory with current prices for the requesting player.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
func (s *GameServiceServer) handleBrowse(uid string, req *gamev1.BrowseRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findMerchantInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Merchant == nil {
		return messageEvent("This merchant has no inventory configured."), nil
	}
	evt, err := s.buildShopView(uid, req.GetNpcName())
	if err != nil || evt == nil {
		return messageEvent("This merchant has no inventory configured."), nil
	}
	return evt, nil
}

// handleBuy executes a player purchase from a merchant.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, sess.Currency is reduced by total cost and state.Stock[itemID] is decremented.
func (s *GameServiceServer) handleBuy(uid string, req *gamev1.BuyRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findMerchantInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Merchant == nil {
		return messageEvent("This merchant has no inventory."), nil
	}
	state := s.merchantStateFor(inst.ID)
	if state == nil {
		s.initMerchantRuntimeState(inst)
		state = s.merchantStateFor(inst.ID)
	}
	query := req.GetItemId()
	qty := int(req.GetQuantity())
	if qty < 1 {
		qty = 1
	}
	// Resolve query to canonical item ID using fuzzy matching:
	// 1. exact, 2. case-insensitive, 3. slug-normalized, 4. display name.
	normQuery := normalizeMerchantQuery(query)
	var itemCfg *npc.MerchantItem
	for i := range tmpl.Merchant.Inventory {
		id := tmpl.Merchant.Inventory[i].ItemID
		if id == query || strings.EqualFold(id, query) || normalizeMerchantQuery(id) == normQuery {
			itemCfg = &tmpl.Merchant.Inventory[i]
			break
		}
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(id); ok {
				if strings.EqualFold(def.Name, query) || normalizeMerchantQuery(def.Name) == normQuery {
					itemCfg = &tmpl.Merchant.Inventory[i]
					break
				}
			}
		}
	}
	// Use the canonical ID for all downstream operations.
	itemID := query
	if itemCfg != nil {
		itemID = itemCfg.ItemID
	}
	if itemCfg == nil {
		// Check merchant material stock
		for _, ms := range tmpl.Merchant.MaterialStock {
			if s.materialReg == nil {
				break
			}
			matDef, ok := s.materialReg.Material(ms.ID)
			if !ok || !strings.EqualFold(matDef.Name, itemID) {
				continue
			}
			if sess.Currency < ms.Price {
				return messageEvent(fmt.Sprintf("You can't afford that. It costs %d credits and you have %d.", ms.Price, sess.Currency)), nil
			}
			sess.Currency -= ms.Price
			if sess.Materials == nil {
				sess.Materials = make(map[string]int)
			}
			sess.Materials[ms.ID]++
			if s.materialRepo != nil {
				_ = s.materialRepo.Add(context.Background(), sess.CharacterID, ms.ID, 1)
			}
			return messageEvent(fmt.Sprintf("You buy 1 %s for %d credits.", matDef.Name, ms.Price)), nil
		}
		return messageEvent(fmt.Sprintf("%s doesn't sell %q.", inst.Name(), itemID)), nil
	}
	// Faction item gating check (REQ-FA-19, REQ-FA-30).
	if s.factionSvc != nil && !s.factionSvc.CanBuyItem(sess, itemID) {
		tierLabel, factionName := s.factionSvc.ExclusiveTierLabel(itemID)
		return messageEvent(fmt.Sprintf("You need to be a %s of %s to buy that.", tierLabel, factionName)), nil
	}
	// Enemy faction NPC check (REQ-FA-28).
	if s.factionSvc != nil && inst.FactionID != "" && s.factionSvc.IsEnemyOf(sess, inst.FactionID) {
		return messageEvent(fmt.Sprintf("%s eyes you coldly. 'We don't serve your kind here.'", inst.Name())), nil
	}
	merchantRuntimeMu.RLock()
	stock := state.Stock[itemID]
	merchantRuntimeMu.RUnlock()
	if stock < qty {
		return messageEvent(fmt.Sprintf("%s is out of stock on %s.", inst.Name(), itemID)), nil
	}
	surcharge := s.wantedSurchargeFor(sess, inst)
	unitPrice := npc.ComputeBuyPrice(itemCfg.BasePrice, tmpl.Merchant.SellMargin, surcharge, sess.NegotiateModifier)
	// Apply faction discount if merchant belongs to player's faction (REQ-FA-32, 33).
	if s.factionSvc != nil && inst.FactionID != "" && inst.FactionID == sess.FactionID {
		rep := sess.FactionRep[sess.FactionID]
		discount := s.factionSvc.DiscountFor(sess.FactionID, rep)
		if discount > 0 {
			unitPrice = int(math.Floor(float64(itemCfg.BasePrice) * float64(tmpl.Merchant.SellMargin) * (1.0 - discount)))
		}
	}
	total := unitPrice * qty
	if sess.Currency < total {
		return messageEvent(fmt.Sprintf("You can't afford that. It costs %d credits and you have %d.", total, sess.Currency)), nil
	}
	merchantRuntimeMu.Lock()
	state.Stock[itemID] -= qty
	merchantRuntimeMu.Unlock()
	sess.Currency -= total

	// Add purchased item to player backpack.
	// Precondition: Backpack is non-nil (guaranteed by session.AddPlayer).
	// Postcondition: On failure the stock and currency changes are rolled back so
	// the player does not lose credits for items they cannot carry.
	if sess.Backpack == nil || s.invRegistry == nil {
		merchantRuntimeMu.Lock()
		state.Stock[itemID] += qty
		merchantRuntimeMu.Unlock()
		sess.Currency += total
		return messageEvent("Purchase failed: inventory unavailable."), nil
	}
	if _, addErr := sess.Backpack.Add(itemID, qty, s.invRegistry); addErr != nil {
		merchantRuntimeMu.Lock()
		state.Stock[itemID] += qty
		merchantRuntimeMu.Unlock()
		sess.Currency += total
		s.logger.Warn("handleBuy: failed to add item to backpack — rolled back",
			zap.String("uid", uid),
			zap.String("itemID", itemID),
			zap.Int("qty", qty),
			zap.Error(addErr),
		)
		return messageEvent(fmt.Sprintf("Purchase failed: %s", addErr.Error())), nil
	}

	// Persist inventory and currency.
	if s.charSaver != nil && sess.CharacterID > 0 {
		ctx := context.Background()
		if err := s.charSaver.SaveInventory(ctx, sess.CharacterID, backpackToInventoryItems(sess.Backpack)); err != nil {
			s.logger.Warn("handleBuy: SaveInventory failed", zap.Error(err))
		}
		if err := s.charSaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency); err != nil {
			s.logger.Warn("handleBuy: SaveCurrency failed", zap.Error(err))
		}
	}

	// Push InventoryView to player stream so the frontend reflects the purchase.
	if invEvt, _ := s.handleInventory(uid); invEvt != nil && sess.Entity != nil {
		if data, marshalErr := proto.Marshal(invEvt); marshalErr == nil {
			_ = sess.Entity.PushBlocking(data, time.Second)
		}
	}

	// Push updated ShopView so client reflects new stock quantities (BUG-102).
	if shopEvt, shopErr := s.buildShopView(uid, req.GetNpcName()); shopErr == nil && shopEvt != nil {
		if sess.Entity != nil {
			if data, marshalErr := proto.Marshal(shopEvt); marshalErr == nil {
				_ = sess.Entity.PushBlocking(data, time.Second)
			}
		}
	}

	// Push CharacterSheetView so Crypto balance updates immediately after purchase (REQ-BUG74).
	s.pushCharacterSheet(sess)

	return messageEvent(fmt.Sprintf("You buy %d× %s for %d credits.", qty, itemID, total)), nil
}

// handleSell executes a player sale to a merchant.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On success, sess.Currency is increased by payout and state.CurrentBudget is reduced.
func (s *GameServiceServer) handleSell(uid string, req *gamev1.SellRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findMerchantInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	tmpl := s.npcMgr.TemplateByID(inst.TemplateID)
	if tmpl == nil || tmpl.Merchant == nil {
		return messageEvent("This merchant doesn't buy anything."), nil
	}
	state := s.merchantStateFor(inst.ID)
	if state == nil {
		s.initMerchantRuntimeState(inst)
		state = s.merchantStateFor(inst.ID)
	}
	itemID := req.GetItemId()
	qty := int(req.GetQuantity())
	if qty < 1 {
		qty = 1
	}
	var itemCfg *npc.MerchantItem
	for i := range tmpl.Merchant.Inventory {
		if tmpl.Merchant.Inventory[i].ItemID == itemID {
			itemCfg = &tmpl.Merchant.Inventory[i]
			break
		}
	}
	if itemCfg == nil {
		return messageEvent(fmt.Sprintf("%s doesn't buy %q.", inst.Name(), itemID)), nil
	}
	// Verify the player actually has the item in their backpack.
	if sess.Backpack == nil {
		return messageEvent("You don't have that item."), nil
	}
	instances := sess.Backpack.FindByItemDefID(itemID)
	if len(instances) == 0 {
		return messageEvent(fmt.Sprintf("You don't have %q to sell.", itemID)), nil
	}
	// Tally total owned quantity.
	owned := 0
	for _, inst2 := range instances {
		owned += inst2.Quantity
	}
	if owned < qty {
		return messageEvent(fmt.Sprintf("You only have %d of %q.", owned, itemID)), nil
	}

	payout := npc.ComputeSellPayout(itemCfg.BasePrice, tmpl.Merchant.BuyMargin, qty, sess.NegotiateModifier)
	merchantRuntimeMu.RLock()
	budget := state.CurrentBudget
	merchantRuntimeMu.RUnlock()
	if budget < payout {
		return messageEvent(fmt.Sprintf("%s can't afford to buy that right now.", inst.Name())), nil
	}
	merchantRuntimeMu.Lock()
	state.CurrentBudget -= payout
	merchantRuntimeMu.Unlock()

	// Remove qty items from the backpack, draining stacks in order.
	remaining := qty
	for _, backpackInst := range instances {
		if remaining <= 0 {
			break
		}
		take := backpackInst.Quantity
		if take > remaining {
			take = remaining
		}
		if removeErr := sess.Backpack.Remove(backpackInst.InstanceID, take); removeErr != nil {
			s.logger.Warn("handleSell: failed to remove item from backpack",
				zap.String("uid", uid),
				zap.String("itemID", itemID),
				zap.Error(removeErr),
			)
		}
		remaining -= take
	}

	sess.Currency += payout

	// Persist inventory and currency.
	if s.charSaver != nil && sess.CharacterID > 0 {
		ctx := context.Background()
		if err := s.charSaver.SaveInventory(ctx, sess.CharacterID, backpackToInventoryItems(sess.Backpack)); err != nil {
			s.logger.Warn("handleSell: SaveInventory failed", zap.Error(err))
		}
		if err := s.charSaver.SaveCurrency(ctx, sess.CharacterID, sess.Currency); err != nil {
			s.logger.Warn("handleSell: SaveCurrency failed", zap.Error(err))
		}
	}

	// Push updated InventoryView so the frontend reflects the sold item.
	if invEvt, _ := s.handleInventory(uid); invEvt != nil && sess.Entity != nil {
		if data, marshalErr := proto.Marshal(invEvt); marshalErr == nil {
			_ = sess.Entity.PushBlocking(data, time.Second)
		}
	}

	// Push updated ShopView so client reflects new budget.
	if shopEvt, shopErr := s.buildShopView(uid, req.GetNpcName()); shopErr == nil && shopEvt != nil {
		if sess.Entity != nil {
			if data, marshalErr := proto.Marshal(shopEvt); marshalErr == nil {
				_ = sess.Entity.PushBlocking(data, time.Second)
			}
		}
	}

	// Push CharacterSheetView so Crypto balance updates immediately after sale (REQ-BUG74).
	s.pushCharacterSheet(sess)

	return messageEvent(fmt.Sprintf("%s buys %d× %s from you for %d credits.", inst.Name(), qty, itemID, payout)), nil
}

// handleNegotiate attempts a skill check for a session-scoped price modifier. REQ-NPC-5.
//
// Precondition: uid identifies an active player session; req is non-nil.
// Postcondition: Returns a non-nil ServerEvent; error is always nil.
// On first attempt, sess.NegotiateModifier and sess.NegotiatedMerchantID are set.
func (s *GameServiceServer) handleNegotiate(uid string, req *gamev1.NegotiateRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findMerchantInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	if sess.NegotiatedMerchantID == inst.ID {
		return messageEvent(fmt.Sprintf("You've already tried negotiating with %s this visit.", inst.Name())), nil
	}
	dc := 10 + inst.Awareness
	roll := rand.Intn(20) + 1
	skillID := req.GetSkill()
	if skillID == "" {
		skillID = "smooth_talk"
	}
	skillMod := merchantSkillModifier(sess.Skills[skillID])
	total := roll + skillMod
	var outcome string
	switch {
	case total >= dc+10:
		outcome = "crit_success"
	case total >= dc:
		outcome = "success"
	case total <= dc-10:
		outcome = "crit_failure"
	default:
		outcome = "failure"
	}
	mod := npc.ApplyNegotiateOutcome(outcome)
	sess.NegotiateModifier = mod
	sess.NegotiatedMerchantID = inst.ID
	var msg string
	switch outcome {
	case "crit_success":
		msg = fmt.Sprintf("You charm %s brilliantly. Prices improved by 20%%!", inst.Name())
	case "success":
		msg = fmt.Sprintf("You negotiate with %s. Prices improved by 10%%.", inst.Name())
	case "crit_failure":
		msg = fmt.Sprintf("%s is insulted by your pitch. Buy prices increased by 10%%.", inst.Name())
	default:
		msg = fmt.Sprintf("%s shrugs off your pitch. No change.", inst.Name())
	}
	return messageEvent(msg), nil
}

// merchantSkillModifier converts a proficiency rank to a flat modifier.
//
// Precondition: rank is one of "trained", "expert", "master", "legendary", or empty/unknown.
// Postcondition: Returns a non-negative integer.
func merchantSkillModifier(rank string) int {
	switch rank {
	case "trained":
		return 2
	case "expert":
		return 4
	case "master":
		return 6
	case "legendary":
		return 8
	default:
		return 0
	}
}

// clearNegotiateState resets session-scoped negotiate fields on room transition. REQ-NPC-5a.
//
// Precondition: sess is non-nil.
// Postcondition: sess.NegotiateModifier == 0.0 and sess.NegotiatedMerchantID == "".
func (s *GameServiceServer) clearNegotiateState(sess *session.PlayerSession) {
	sess.NegotiateModifier = 0.0
	sess.NegotiatedMerchantID = ""
}
