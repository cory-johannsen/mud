package detection_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/detection"
)

// fixedSrc returns a fixed Intn value (the d20 face minus 1, since the gate
// adds +1 to the result of Intn(20)).
type fixedSrc struct{ face int }

// Intn for fixedSrc ignores n and returns face-1 so callers see exactly face
// when the gate evaluates `Intn(20) + 1`.
func (f fixedSrc) Intn(int) int { return f.face - 1 }

func TestGate_Observed_Proceeds(t *testing.T) {
	res := detection.GateAttack(detection.Observed, fixedSrc{20})
	if res.Outcome != detection.GateProceed || res.OffGuard {
		t.Errorf("Observed: %+v want Proceed/!OffGuard", res)
	}
}

func TestGate_Concealed_FlatCheckFail_AutoMiss(t *testing.T) {
	res := detection.GateAttack(detection.Concealed, fixedSrc{4}) // < 5
	if res.Outcome != detection.GateAutoMiss {
		t.Errorf("Concealed flat-fail: %+v want AutoMiss", res)
	}
}

func TestGate_Concealed_FlatCheckPass_Proceeds(t *testing.T) {
	res := detection.GateAttack(detection.Concealed, fixedSrc{5})
	if res.Outcome != detection.GateProceed || res.OffGuard {
		t.Errorf("Concealed flat-pass: %+v want Proceed/!OffGuard", res)
	}
}

func TestGate_Hidden_FlatCheckFail_AutoMiss(t *testing.T) {
	res := detection.GateAttack(detection.Hidden, fixedSrc{10})
	if res.Outcome != detection.GateAutoMiss {
		t.Errorf("Hidden flat-fail: %+v want AutoMiss", res)
	}
}

func TestGate_Hidden_PassAppliesOffGuard(t *testing.T) {
	res := detection.GateAttack(detection.Hidden, fixedSrc{15})
	if res.Outcome != detection.GateProceed || !res.OffGuard {
		t.Errorf("Hidden flat-pass: %+v want Proceed/OffGuard", res)
	}
}

func TestGate_Undetected_NoGuess_AutoMiss(t *testing.T) {
	res := detection.GateAttack(detection.Undetected, fixedSrc{20})
	if res.Outcome != detection.GateAutoMiss || !res.OffGuard {
		t.Errorf("Undetected no-guess: %+v want AutoMiss/OffGuard", res)
	}
}

func TestGate_Undetected_WrongSquare_AutoMiss(t *testing.T) {
	res := detection.GateAttack(detection.Undetected, fixedSrc{20},
		detection.WithSquareGuess(detection.Cell{X: 1, Y: 1}),
		detection.WithTargetCell(detection.Cell{X: 9, Y: 9}))
	if res.Outcome != detection.GateAutoMiss || !res.OffGuard {
		t.Errorf("Undetected wrong-square: %+v want AutoMiss/OffGuard", res)
	}
}

func TestGate_Undetected_RightSquareThenFlatCheckFail(t *testing.T) {
	c := detection.Cell{X: 5, Y: 5}
	res := detection.GateAttack(detection.Undetected, fixedSrc{5},
		detection.WithSquareGuess(c), detection.WithTargetCell(c))
	if res.Outcome != detection.GateAutoMiss || !res.OffGuard {
		t.Errorf("Undetected right-square flat-fail: %+v want AutoMiss/OffGuard", res)
	}
}

func TestGate_Undetected_RightSquareFlatCheckPass(t *testing.T) {
	c := detection.Cell{X: 5, Y: 5}
	res := detection.GateAttack(detection.Undetected, fixedSrc{15},
		detection.WithSquareGuess(c), detection.WithTargetCell(c))
	if res.Outcome != detection.GateProceed || !res.OffGuard {
		t.Errorf("Undetected right-square flat-pass: %+v want Proceed/OffGuard", res)
	}
}

func TestGate_Invisible_WithSoundCue_BehavesLikeHidden(t *testing.T) {
	res := detection.GateAttack(detection.Invisible, fixedSrc{15}, detection.WithSoundCue(true))
	if res.Outcome != detection.GateProceed || !res.OffGuard {
		t.Errorf("Invisible+sound flat-pass: %+v want Proceed/OffGuard", res)
	}
}

func TestGate_Invisible_WithoutSoundCue_BehavesLikeUndetected(t *testing.T) {
	res := detection.GateAttack(detection.Invisible, fixedSrc{20}, detection.WithSoundCue(false))
	if res.Outcome != detection.GateAutoMiss || !res.OffGuard {
		t.Errorf("Invisible no-sound: %+v want AutoMiss/OffGuard", res)
	}
}
