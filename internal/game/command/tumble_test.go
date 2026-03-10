package command

import (
	"testing"

	"pgregory.net/rapid"
)

func TestHandleTumble_WithTarget(t *testing.T) {
	req, err := HandleTumble([]string{"bandit"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil TumbleRequest")
	}
	if req.Target != "bandit" {
		t.Errorf("expected Target=%q, got %q", "bandit", req.Target)
	}
}

func TestHandleTumble_NoArgs(t *testing.T) {
	req, err := HandleTumble(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil TumbleRequest")
	}
	if req.Target != "" {
		t.Errorf("expected empty Target, got %q", req.Target)
	}
}

func TestHandleTumble_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := HandleTumble(args)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			rt.Fatal("expected non-nil TumbleRequest")
		}
		// Invariant: Target is the first arg when args are present, empty otherwise.
		if len(args) >= 1 {
			if req.Target != args[0] {
				rt.Fatalf("expected Target=%q, got %q", args[0], req.Target)
			}
		} else {
			if req.Target != "" {
				rt.Fatalf("expected empty Target when no args, got %q", req.Target)
			}
		}
	})
}
