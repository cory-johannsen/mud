package combat

import "testing"

// TestActionType_Reload_CostOne verifies ActionReload costs 1 AP.
func TestActionType_Reload_CostOne(t *testing.T) {
	if got := ActionReload.Cost(); got != 1 {
		t.Errorf("ActionReload.Cost() = %d, want 1", got)
	}
}

// TestActionType_FireBurst_CostTwo verifies ActionFireBurst costs 2 AP.
func TestActionType_FireBurst_CostTwo(t *testing.T) {
	if got := ActionFireBurst.Cost(); got != 2 {
		t.Errorf("ActionFireBurst.Cost() = %d, want 2", got)
	}
}

// TestActionType_FireAutomatic_CostThree verifies ActionFireAutomatic costs 3 AP.
func TestActionType_FireAutomatic_CostThree(t *testing.T) {
	if got := ActionFireAutomatic.Cost(); got != 3 {
		t.Errorf("ActionFireAutomatic.Cost() = %d, want 3", got)
	}
}

// TestActionType_Throw_CostOne verifies ActionThrow costs 1 AP.
func TestActionType_Throw_CostOne(t *testing.T) {
	if got := ActionThrow.Cost(); got != 1 {
		t.Errorf("ActionThrow.Cost() = %d, want 1", got)
	}
}

// TestActionType_String_NewTypes verifies String() for all 4 new ActionTypes.
func TestActionType_String_NewTypes(t *testing.T) {
	cases := []struct {
		action ActionType
		want   string
	}{
		{ActionReload, "reload"},
		{ActionFireBurst, "burst"},
		{ActionFireAutomatic, "automatic"},
		{ActionThrow, "throw"},
	}
	for _, tc := range cases {
		if got := tc.action.String(); got != tc.want {
			t.Errorf("%v.String() = %q, want %q", tc.action, got, tc.want)
		}
	}
}

// TestQueuedAction_WeaponID_Present verifies WeaponID and ExplosiveID fields are accessible.
func TestQueuedAction_WeaponID_Present(t *testing.T) {
	qa := QueuedAction{
		Type:        ActionReload,
		Target:      "enemy",
		WeaponID:    "pistol-01",
		ExplosiveID: "grenade-02",
	}
	if qa.WeaponID != "pistol-01" {
		t.Errorf("WeaponID = %q, want %q", qa.WeaponID, "pistol-01")
	}
	if qa.ExplosiveID != "grenade-02" {
		t.Errorf("ExplosiveID = %q, want %q", qa.ExplosiveID, "grenade-02")
	}
}

// TestProperty_NewActions_CostPositive is a property-based test asserting
// that all 4 new ActionTypes have Cost() > 0.
func TestProperty_NewActions_CostPositive(t *testing.T) {
	newTypes := []ActionType{
		ActionReload,
		ActionFireBurst,
		ActionFireAutomatic,
		ActionThrow,
	}
	for _, at := range newTypes {
		if cost := at.Cost(); cost <= 0 {
			t.Errorf("ActionType %v has non-positive Cost() = %d; all new actions must cost AP > 0", at, cost)
		}
	}
}
