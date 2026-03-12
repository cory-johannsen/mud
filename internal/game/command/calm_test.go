package command

import (
	"testing"

	"pgregory.net/rapid"
)

func TestHandleCalm_NoArgs(t *testing.T) {
	req, err := HandleCalm(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil CalmRequest")
	}
}

func TestHandleCalm_ArgsIgnored(t *testing.T) {
	req, err := HandleCalm([]string{"anything"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil CalmRequest")
	}
}

func TestHandleCalm_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := HandleCalm(args)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			rt.Fatal("expected non-nil CalmRequest")
		}
	})
}
