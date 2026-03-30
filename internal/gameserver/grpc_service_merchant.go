package gameserver

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"sync"
	"time"

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
	state := s.merchantStateFor(inst.ID)
	if state == nil {
		s.initMerchantRuntimeState(inst)
		state = s.merchantStateFor(inst.ID)
	}
	surcharge := s.wantedSurchargeFor(sess, inst)
	rows := npc.BrowseLines(tmpl.Merchant, state, surcharge, sess.NegotiateModifier)
	items := make([]*gamev1.ShopItem, 0, len(rows))
	for _, row := range rows {
		displayName := row.ItemID
		if s.invRegistry != nil {
			if def, ok := s.invRegistry.Item(row.ItemID); ok {
				displayName = def.Name
			}
		}
		items = append(items, &gamev1.ShopItem{
			Name:      displayName,
			ItemId:    row.ItemID,
			BuyPrice:  int32(row.BuyPrice),
			SellPrice: int32(row.SellPrice),
			Stock:     int32(row.Stock),
		})
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
	sess.Currency += payout
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
