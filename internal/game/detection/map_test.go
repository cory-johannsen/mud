package detection_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/detection"
)

func TestMap_AbsentPairDefaultsObserved(t *testing.T) {
	m := detection.NewMap()
	if got := m.Get("a", "b"); got != detection.Observed {
		t.Errorf("absent pair = %v, want Observed", got)
	}
}

func TestMap_NilReceiverReturnsObserved(t *testing.T) {
	var m *detection.Map
	if got := m.Get("a", "b"); got != detection.Observed {
		t.Errorf("nil-receiver Get = %v, want Observed", got)
	}
	// Set on a nil receiver must be a no-op (no panic).
	m.Set("a", "b", detection.Hidden)
}

func TestMap_AsymmetricGetSet(t *testing.T) {
	m := detection.NewMap()
	m.Set("a", "b", detection.Hidden)
	if got := m.Get("a", "b"); got != detection.Hidden {
		t.Errorf("Get(a,b) = %v, want Hidden", got)
	}
	if got := m.Get("b", "a"); got != detection.Observed {
		t.Errorf("Get(b,a) = %v, want Observed (asymmetric)", got)
	}
}

func TestMap_ClearRevertsToObserved(t *testing.T) {
	m := detection.NewMap()
	m.Set("a", "b", detection.Hidden)
	m.Clear("a", "b")
	if got := m.Get("a", "b"); got != detection.Observed {
		t.Errorf("after Clear: Get = %v, want Observed", got)
	}
}

func TestMap_SetObservedRemovesEntry(t *testing.T) {
	m := detection.NewMap()
	m.Set("a", "b", detection.Hidden)
	m.Set("a", "b", detection.Observed)
	if got := m.Get("a", "b"); got != detection.Observed {
		t.Errorf("Set(Observed) should clear: Get = %v, want Observed", got)
	}
}

func TestMap_SetSelfPairRejected(t *testing.T) {
	m := detection.NewMap()
	m.Set("a", "a", detection.Hidden) // observer == target — no-op
	if got := m.Get("a", "a"); got != detection.Observed {
		t.Errorf("self-pair must remain Observed")
	}
}

func TestMap_ForObserverIteratesAllTargets(t *testing.T) {
	m := detection.NewMap()
	m.Set("a", "b", detection.Hidden)
	m.Set("a", "c", detection.Concealed)
	m.Set("b", "c", detection.Undetected) // unrelated row
	got := m.ForObserver("a")
	if len(got) != 2 || got["b"] != detection.Hidden || got["c"] != detection.Concealed {
		t.Errorf("ForObserver(a) = %v, want {b:Hidden, c:Concealed}", got)
	}
}

func TestAdvanceTowardObserved_Ladder(t *testing.T) {
	cases := []struct {
		from, to detection.State
	}{
		{detection.Concealed, detection.Observed},
		{detection.Hidden, detection.Concealed},
		{detection.Undetected, detection.Hidden},
		{detection.Unnoticed, detection.Undetected},
	}
	for _, c := range cases {
		m := detection.NewMap()
		m.Set("o", "t", c.from)
		detection.AdvanceTowardObserved(m, "o", "t")
		if got := m.Get("o", "t"); got != c.to {
			t.Errorf("Advance(%v) = %v, want %v", c.from, got, c.to)
		}
	}
}

func TestAdvanceTowardObserved_NoOpAtTopAndInvisible(t *testing.T) {
	m := detection.NewMap()
	// Observed (absent) should remain Observed.
	detection.AdvanceTowardObserved(m, "o", "t")
	if got := m.Get("o", "t"); got != detection.Observed {
		t.Errorf("Advance from Observed = %v, want Observed", got)
	}
	m.Set("o", "t", detection.Invisible)
	detection.AdvanceTowardObserved(m, "o", "t")
	if got := m.Get("o", "t"); got != detection.Invisible {
		t.Errorf("Advance from Invisible = %v, want Invisible (no-op in v1)", got)
	}
}
