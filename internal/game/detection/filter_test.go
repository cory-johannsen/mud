package detection_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/game/detection"
)

func TestDecideForNPC_ObservedNoChange(t *testing.T) {
	m := detection.NewMap()
	d := detection.DecideForNPC("hero", "thug", m, false)
	if d.Drop || d.RedactName != "" || d.HideCell {
		t.Errorf("Observed: %+v want zero", d)
	}
}

func TestDecideForNPC_ConcealedNoChange(t *testing.T) {
	m := detection.NewMap()
	m.Set("hero", "thug", detection.Concealed)
	d := detection.DecideForNPC("hero", "thug", m, false)
	if d.Drop || d.RedactName != "" || d.HideCell {
		t.Errorf("Concealed: %+v want zero", d)
	}
}

func TestDecideForNPC_HiddenSilhouetteCellShown(t *testing.T) {
	m := detection.NewMap()
	m.Set("hero", "thug", detection.Hidden)
	d := detection.DecideForNPC("hero", "thug", m, false)
	if d.Drop || d.RedactName != "<silhouette>" || d.HideCell {
		t.Errorf("Hidden: %+v want silhouette+cell", d)
	}
}

func TestDecideForNPC_UndetectedTripleQuestionCellHidden(t *testing.T) {
	m := detection.NewMap()
	m.Set("hero", "stalker", detection.Undetected)
	d := detection.DecideForNPC("hero", "stalker", m, false)
	if d.Drop || d.RedactName != "???" || !d.HideCell {
		t.Errorf("Undetected: %+v want ??? + hide cell", d)
	}
}

func TestDecideForNPC_UnnoticedDropped(t *testing.T) {
	m := detection.NewMap()
	m.Set("hero", "ghost", detection.Unnoticed)
	d := detection.DecideForNPC("hero", "ghost", m, false)
	if !d.Drop {
		t.Errorf("Unnoticed: %+v want Drop", d)
	}
}

func TestDecideForNPC_InvisibleSoundActsLikeHidden(t *testing.T) {
	m := detection.NewMap()
	m.Set("hero", "ghost", detection.Invisible)
	d := detection.DecideForNPC("hero", "ghost", m, true)
	if d.Drop || d.RedactName != "<silhouette>" || d.HideCell {
		t.Errorf("Invisible+sound: %+v want silhouette+cell", d)
	}
}

func TestDecideForNPC_InvisibleSilentActsLikeUndetected(t *testing.T) {
	m := detection.NewMap()
	m.Set("hero", "ghost", detection.Invisible)
	d := detection.DecideForNPC("hero", "ghost", m, false)
	if d.Drop || d.RedactName != "???" || !d.HideCell {
		t.Errorf("Invisible silent: %+v want ??? + hide cell", d)
	}
}

func TestDecideForNPC_PerRecipientDifferentResults(t *testing.T) {
	m := detection.NewMap()
	m.Set("h1", "thug", detection.Undetected)
	d1 := detection.DecideForNPC("h1", "thug", m, false)
	d2 := detection.DecideForNPC("h2", "thug", m, false)
	if d1.RedactName != "???" {
		t.Errorf("h1 sees %+v, want ???", d1)
	}
	if d2.RedactName != "" || d2.Drop {
		t.Errorf("h2 sees %+v, want unchanged", d2)
	}
}
