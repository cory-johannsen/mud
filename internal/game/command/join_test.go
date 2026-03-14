package command

import (
	"testing"

	"pgregory.net/rapid"
)

func TestHandleJoin_ReturnsNonNilRequest(t *testing.T) {
	req, err := HandleJoin(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if req == nil {
		t.Fatal("expected non-nil JoinRequest")
	}
}

func TestHandleJoin_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		args := rapid.SliceOf(rapid.String()).Draw(rt, "args")
		req, err := HandleJoin(args)
		if err != nil {
			rt.Fatalf("unexpected error: %v", err)
		}
		if req == nil {
			rt.Fatal("expected non-nil JoinRequest")
		}
	})
}
