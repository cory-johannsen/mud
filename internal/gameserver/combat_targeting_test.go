package gameserver

import "testing"

// TestTargetingRegistry_LazyCreate verifies first-access creates state and
// subsequent accesses return the same pointer.
func TestTargetingRegistry_LazyCreate(t *testing.T) {
	r := newTargetingRegistry()
	a := r.For("p1")
	if a == nil {
		t.Fatalf("For returned nil")
	}
	b := r.For("p1")
	if a != b {
		t.Errorf("For returned different instances for same owner")
	}
	c := r.For("p2")
	if c == a {
		t.Errorf("different owners share state")
	}
}

// TestTargetingRegistry_Drop verifies Drop removes state and a fresh
// instance is created on next access.
func TestTargetingRegistry_Drop(t *testing.T) {
	r := newTargetingRegistry()
	a := r.For("p1")
	a.Set("n1")
	r.Drop("p1")
	b := r.For("p1")
	if b == a {
		t.Errorf("Drop did not remove state")
	}
	if b.TargetID() != "" {
		t.Errorf("post-Drop fresh state has TargetID = %q", b.TargetID())
	}
}

// TestTargetingRegistry_NilSafe verifies nil receiver and empty owner safety.
func TestTargetingRegistry_NilSafe(t *testing.T) {
	var r *targetingRegistry
	if got := r.For("p1"); got != nil {
		t.Errorf("nil registry For returned non-nil")
	}
	r.Drop("p1") // must not panic

	r2 := newTargetingRegistry()
	if got := r2.For(""); got != nil {
		t.Errorf("empty-owner For returned non-nil")
	}
}
