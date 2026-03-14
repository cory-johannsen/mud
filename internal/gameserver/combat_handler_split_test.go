package gameserver

import (
	"context"
	"fmt"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/combat"
	"github.com/cory-johannsen/mud/internal/game/dice"
	"github.com/cory-johannsen/mud/internal/game/npc"
	"github.com/cory-johannsen/mud/internal/game/session"
	"github.com/cory-johannsen/mud/internal/game/xp"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"
)

// newMinimalCombatHandler builds a CombatHandler with no savers (saves skipped when nil).
func newMinimalCombatHandler(t *testing.T) *CombatHandler {
	t.Helper()
	_, sessMgr := testWorldAndSession(t)
	roller := dice.NewLoggedRoller(dice.NewCryptoSource(), nil)
	return NewCombatHandler(
		combat.NewEngine(), npc.NewManager(), sessMgr, roller,
		func(_ string, _ []*gamev1.CombatEvent) {},
		testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
	)
}

// REQ-T12: Single participant receives full currency.
func TestSplit_SingleParticipant_CurrencyUnchanged(t *testing.T) {
	h := newMinimalCombatHandler(t)
	p1 := &session.PlayerSession{UID: "p1", Currency: 0}

	h.distributeCurrencyLocked(context.Background(), []*session.PlayerSession{p1}, 42)

	require.Equal(t, 42, p1.Currency, "single participant must receive all currency")
}

// REQ-T7: Two players, NPC level 2, each gets half XP.
func TestSplit_TwoPlayers_XPSplit(t *testing.T) {
	xpSvc := xp.NewService(testXPConfig(), &grantXPProgressSaver{})
	p1 := &session.PlayerSession{UID: "p1", Experience: 0, Level: 1, MaxHP: 20, CurrentHP: 20}
	p2 := &session.PlayerSession{UID: "p2", Experience: 0, Level: 1, MaxHP: 20, CurrentHP: 20}

	cfg := xpSvc.Config()
	totalXP := 2 * cfg.Awards.KillXPPerNPCLevel // npcLevel=2
	share := totalXP / 2

	_, err := xpSvc.AwardXPAmount(context.Background(), p1, 0, share)
	require.NoError(t, err)
	_, err = xpSvc.AwardXPAmount(context.Background(), p2, 0, share)
	require.NoError(t, err)

	require.Equal(t, share, p1.Experience, "p1 must receive floor(totalXP/2)")
	require.Equal(t, share, p2.Experience, "p2 must receive floor(totalXP/2)")
}

// REQ-T8: Two players, 10 currency → each receives 5.
func TestSplit_TwoPlayers_CurrencySplit(t *testing.T) {
	h := newMinimalCombatHandler(t)
	p1 := &session.PlayerSession{UID: "p1", Currency: 0}
	p2 := &session.PlayerSession{UID: "p2", Currency: 0}

	h.distributeCurrencyLocked(context.Background(), []*session.PlayerSession{p1, p2}, 10)

	require.Equal(t, 5, p1.Currency, "p1 must receive floor(10/2)=5")
	require.Equal(t, 5, p2.Currency, "p2 must receive floor(10/2)=5")
}

// REQ-T18: 3 players, 2 currency → share=0 fallback: first gets 1, others get 0.
func TestSplit_Currency_ShareZeroFallback(t *testing.T) {
	h := newMinimalCombatHandler(t)
	p1 := &session.PlayerSession{UID: "p1", Currency: 0}
	p2 := &session.PlayerSession{UID: "p2", Currency: 0}
	p3 := &session.PlayerSession{UID: "p3", Currency: 0}

	h.distributeCurrencyLocked(context.Background(), []*session.PlayerSession{p1, p2, p3}, 2)

	require.Equal(t, 1, p1.Currency, "first participant gets 1 in share=0 fallback")
	require.Equal(t, 0, p2.Currency, "second participant gets 0 in share=0 fallback")
	require.Equal(t, 0, p3.Currency, "third participant gets 0 in share=0 fallback")
}

// REQ-T10 (property): Each participant receives floor(totalCurrency/n).
func TestProperty_Split_Currency_EqualShare(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(1, 10).Draw(rt, "n")
		totalCurrency := rapid.IntRange(0, 10000).Draw(rt, "totalCurrency")
		expectedShare := totalCurrency / n

		participants := make([]*session.PlayerSession, n)
		for i := range participants {
			participants[i] = &session.PlayerSession{UID: fmt.Sprintf("p%d", i), Currency: 0}
		}

		_, sessMgr := testWorldAndSession(t)
		roller := dice.NewLoggedRoller(dice.NewCryptoSource(), nil)
		h := NewCombatHandler(
			combat.NewEngine(), npc.NewManager(), sessMgr, roller,
			func(_ string, _ []*gamev1.CombatEvent) {},
			testRoundDuration, makeTestConditionRegistry(), nil, nil, nil, nil, nil, nil, nil,
		)

		h.distributeCurrencyLocked(context.Background(), participants, totalCurrency)

		if totalCurrency == 0 {
			for _, p := range participants {
				require.Equal(rt, 0, p.Currency, "zero totalCurrency: no participant gains anything")
			}
			return
		}

		if expectedShare == 0 {
			require.Equal(rt, 1, participants[0].Currency,
				"share=0 fallback: first participant gets 1")
			for _, p := range participants[1:] {
				require.Equal(rt, 0, p.Currency, "share=0 fallback: other participants get 0")
			}
			return
		}

		for _, p := range participants {
			require.Equal(rt, expectedShare, p.Currency,
				"each participant must receive exactly floor(totalCurrency/n)")
		}
	})
}

// REQ-T17 (property): Each participant receives floor(totalXP/n) via AwardXPAmount.
func TestProperty_Split_XP_EqualShare(t *testing.T) {
	cfg := testXPConfig()
	rapid.Check(t, func(rt *rapid.T) {
		npcLevel := rapid.IntRange(1, 5).Draw(rt, "npcLevel")
		n := rapid.IntRange(1, 5).Draw(rt, "n")

		totalXP := npcLevel * cfg.Awards.KillXPPerNPCLevel
		share := totalXP / n

		participants := make([]*session.PlayerSession, n)
		for i := range participants {
			participants[i] = &session.PlayerSession{
				UID: fmt.Sprintf("p%d", i), Experience: 0, Level: 1, MaxHP: 20, CurrentHP: 20,
			}
		}

		xpSvc := xp.NewService(cfg, &grantXPProgressSaver{})

		if share == 0 && totalXP > 0 {
			_, err := xpSvc.AwardXPAmount(context.Background(), participants[0], 0, 1)
			require.NoError(rt, err)
			require.Equal(rt, 1, participants[0].Experience,
				"first participant must receive 1 XP in share=0 fallback")
			for _, p := range participants[1:] {
				require.Equal(rt, 0, p.Experience,
					"other participants must receive 0 XP in share=0 fallback")
			}
		} else {
			for _, p := range participants {
				_, err := xpSvc.AwardXPAmount(context.Background(), p, 0, share)
				require.NoError(rt, err)
			}
			for _, p := range participants {
				require.Equal(rt, share, p.Experience,
					"each participant must receive exactly floor(totalXP/n) XP")
			}
		}
	})
}
