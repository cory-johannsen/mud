package command

import (
	"testing"

	"pgregory.net/rapid"
)

func TestHandleAction_NoArgs(t *testing.T) {
	req, err := HandleAction([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Name != "" {
		t.Errorf("Name: got %q, want empty", req.Name)
	}
	if req.Target != "" {
		t.Errorf("Target: got %q, want empty", req.Target)
	}
}

func TestHandleAction_NameOnly(t *testing.T) {
	req, err := HandleAction([]string{"surge"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Name != "surge" {
		t.Errorf("Name: got %q, want %q", req.Name, "surge")
	}
	if req.Target != "" {
		t.Errorf("Target: got %q, want empty", req.Target)
	}
}

func TestHandleAction_NameAndTarget(t *testing.T) {
	req, err := HandleAction([]string{"slam", "Guard"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Name != "slam" {
		t.Errorf("Name: got %q, want %q", req.Name, "slam")
	}
	if req.Target != "Guard" {
		t.Errorf("Target: got %q, want %q", req.Target, "Guard")
	}
}

func TestHandleAction_ExtraArgsIgnored(t *testing.T) {
	req, err := HandleAction([]string{"slam", "Guard", "extra"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req.Name != "slam" {
		t.Errorf("Name: got %q, want %q", req.Name, "slam")
	}
	if req.Target != "Guard" {
		t.Errorf("Target: got %q, want %q", req.Target, "Guard")
	}
}

func TestProperty_HandleAction_NeverPanics(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		n := rapid.IntRange(0, 5).Draw(t, "n")
		args := make([]string, n)
		for i := range args {
			args[i] = rapid.StringMatching(`[a-zA-Z0-9_]+`).Draw(t, "arg")
		}
		req, err := HandleAction(args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			t.Fatal("nil ActionRequest returned")
		}
	})
}
