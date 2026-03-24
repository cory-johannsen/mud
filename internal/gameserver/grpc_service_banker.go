package gameserver

import (
	"fmt"
	"sync"

	"github.com/cory-johannsen/mud/internal/game/npc"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

var bankerRuntimeMu sync.RWMutex

// findBankerInRoom returns the first banker NPC matching npcName in roomID,
// or nil and an error message if the NPC is absent, not a banker, or cowering.
func (s *GameServiceServer) findBankerInRoom(roomID, npcName string) (*npc.Instance, string) {
	inst := s.npcMgr.FindInRoom(roomID, npcName)
	if inst == nil {
		return nil, fmt.Sprintf("You don't see %q here.", npcName)
	}
	if inst.NPCType != "banker" {
		return nil, fmt.Sprintf("%s is not a banker.", inst.Name())
	}
	if inst.Cowering {
		return nil, fmt.Sprintf("%s is cowering in fear and won't respond right now.", inst.Name())
	}
	return inst, ""
}

// bankerRateFor returns the current exchange rate for a banker instance (default 1.0).
func (s *GameServiceServer) bankerRateFor(instID string) float64 {
	bankerRuntimeMu.RLock()
	defer bankerRuntimeMu.RUnlock()
	if st, ok := s.bankerRuntimeStates[instID]; ok {
		return st.CurrentRate
	}
	return 1.0
}

// handleStashDeposit deducts amount from carried credits and adds floor(amount*rate) to stash. REQ-NPC-14.
func (s *GameServiceServer) handleStashDeposit(uid string, req *gamev1.StashDepositRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findBankerInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	// Enemy faction non-combat NPC check (REQ-FA-28).
	if s.factionSvc != nil && s.factionSvc.IsEnemyOf(sess, inst.FactionID) {
		return messageEvent(fmt.Sprintf("%s eyes you coldly. 'We don't serve your kind here.'", inst.Name())), nil
	}
	amount := int(req.GetAmount())
	if amount <= 0 {
		return messageEvent("You must deposit a positive amount."), nil
	}
	if sess.Currency < amount {
		return messageEvent(fmt.Sprintf("You don't have %d credits to deposit.", amount)), nil
	}
	rate := s.bankerRateFor(inst.ID)
	stashAdded := npc.ComputeDeposit(amount, rate)
	sess.Currency -= amount
	sess.StashBalance += stashAdded
	return messageEvent(fmt.Sprintf(
		"You deposited %d credits. %d added to your stash (rate: %.2f). Stash balance: %d.",
		amount, stashAdded, rate, sess.StashBalance,
	)), nil
}

// handleStashWithdraw deducts amount from stash and adds floor(amount/rate) to carried credits. REQ-NPC-14.
func (s *GameServiceServer) handleStashWithdraw(uid string, req *gamev1.StashWithdrawRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findBankerInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	// Enemy faction non-combat NPC check (REQ-FA-28).
	if s.factionSvc != nil && s.factionSvc.IsEnemyOf(sess, inst.FactionID) {
		return messageEvent(fmt.Sprintf("%s eyes you coldly. 'We don't serve your kind here.'", inst.Name())), nil
	}
	amount := int(req.GetAmount())
	if amount <= 0 {
		return messageEvent("You must withdraw a positive amount."), nil
	}
	if sess.StashBalance < amount {
		return messageEvent(fmt.Sprintf("You don't have enough in your stash. Balance: %d.", sess.StashBalance)), nil
	}
	rate := s.bankerRateFor(inst.ID)
	creditsReceived := npc.ComputeWithdrawal(amount, rate)
	sess.StashBalance -= amount
	sess.Currency += creditsReceived
	return messageEvent(fmt.Sprintf(
		"You withdrew %d from your stash and received %d credits (rate: %.2f). Stash balance: %d.",
		amount, creditsReceived, rate, sess.StashBalance,
	)), nil
}

// handleStashBalance displays stash balance and the banker's current exchange rate.
func (s *GameServiceServer) handleStashBalance(uid string, req *gamev1.StashBalanceRequest) (*gamev1.ServerEvent, error) {
	sess, ok := s.sessions.GetPlayer(uid)
	if !ok {
		return messageEvent("player not found"), nil
	}
	inst, errMsg := s.findBankerInRoom(sess.RoomID, req.GetNpcName())
	if inst == nil {
		return messageEvent(errMsg), nil
	}
	// Enemy faction non-combat NPC check (REQ-FA-28).
	if s.factionSvc != nil && s.factionSvc.IsEnemyOf(sess, inst.FactionID) {
		return messageEvent(fmt.Sprintf("%s eyes you coldly. 'We don't serve your kind here.'", inst.Name())), nil
	}
	rate := s.bankerRateFor(inst.ID)
	return messageEvent(fmt.Sprintf(
		"Stash balance: %d credits.\n%s's current rate: %.2f.",
		sess.StashBalance, inst.Name(), rate,
	)), nil
}
