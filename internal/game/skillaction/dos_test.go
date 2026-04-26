package skillaction_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/game/skillaction"
)

func TestDoS_PlusTenIsCritSuccess(t *testing.T) {
	require.Equal(t, skillaction.CritSuccess, skillaction.DoS(15, 5, 10)) // 20 vs 10 → +10 → crit success
}

func TestDoS_HitDcIsSuccess(t *testing.T) {
	require.Equal(t, skillaction.Success, skillaction.DoS(10, 0, 10))
}

func TestDoS_OneShortIsFailure(t *testing.T) {
	require.Equal(t, skillaction.Failure, skillaction.DoS(9, 0, 10))
}

func TestDoS_MinusTenIsCritFailure(t *testing.T) {
	require.Equal(t, skillaction.CritFailure, skillaction.DoS(2, 0, 15)) // 2 vs 15 → -13 → crit failure
}

func TestDoS_Nat20BumpsFailureToSuccess(t *testing.T) {
	// roll 20 + bonus 0 vs DC 30 → raw failure (20 vs 30 in -10..0 band).
	// Nat 20 bumps one step → success.
	require.Equal(t, skillaction.Success, skillaction.DoS(20, 0, 30))
}

func TestDoS_Nat20OnSuccessBumpsToCritSuccess(t *testing.T) {
	// roll 20 + bonus 0 vs DC 15 → raw success; nat 20 → crit success.
	require.Equal(t, skillaction.CritSuccess, skillaction.DoS(20, 0, 15))
}

func TestDoS_Nat20OnCritStaysCrit(t *testing.T) {
	require.Equal(t, skillaction.CritSuccess, skillaction.DoS(20, 20, 15))
}

func TestDoS_Nat1OnCritFailureStaysCritFailure(t *testing.T) {
	require.Equal(t, skillaction.CritFailure, skillaction.DoS(1, 0, 15))
}

func TestDoS_Nat1OnCritSuccessBumpsDownToSuccess(t *testing.T) {
	// roll 1 + bonus 30 vs DC 10 → raw crit success (31 vs 10); nat 1 → success.
	require.Equal(t, skillaction.Success, skillaction.DoS(1, 30, 10))
}

func TestDoS_Nat1OnSuccessBumpsDownToFailure(t *testing.T) {
	// roll 1 + bonus 12 vs DC 10 → raw success (13 vs 10); nat 1 → failure.
	require.Equal(t, skillaction.Failure, skillaction.DoS(1, 12, 10))
}
