package command

import (
	"testing"

	"pgregory.net/rapid"
)

func TestHandleDecline_ReturnsNonNilRequest(t *testing.T) {
	req, err := HandleDecline(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil DeclineRequest")
	}
}

func TestHandleDecline_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := HandleDecline(args)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			rt.Fatal("expected non-nil DeclineRequest")
		}
	})
}
