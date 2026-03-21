package npc_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/npc"
)

// TestAwarenessFieldExists verifies that Template and Instance expose Awareness, not Perception.
//
// Precondition: npc package is compiled.
// Postcondition: Both Template.Awareness and Instance.Awareness are accessible int fields.
func TestAwarenessFieldExists(t *testing.T) {
	tmpl := &npc.Template{Awareness: 10}
	if tmpl.Awareness != 10 {
		t.Errorf("Template.Awareness = %d; want 10", tmpl.Awareness)
	}
	inst := &npc.Instance{Awareness: 5}
	if inst.Awareness != 5 {
		t.Errorf("Instance.Awareness = %d; want 5", inst.Awareness)
	}
}
