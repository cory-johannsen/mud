package gamev1

// LoadoutWeaponPreset describes one weapon preset (main-hand + off-hand).
// This is a hand-written proto mirror for the web client; it uses encoding/json tags
// because protojson cannot marshal types without a registered protoreflect.Message.
type LoadoutWeaponPreset struct {
	MainHand       string `json:"mainHand,omitempty"`
	OffHand        string `json:"offHand,omitempty"`
	MainHandDamage string `json:"mainHandDamage,omitempty"`
	OffHandDamage  string `json:"offHandDamage,omitempty"`
}

// LoadoutView delivers the player's full weapon loadout state to the web client.
// This is a hand-written proto mirror; see LoadoutWeaponPreset.
type LoadoutView struct {
	Presets     []*LoadoutWeaponPreset `json:"presets,omitempty"`
	ActiveIndex int32                  `json:"activeIndex,omitempty"`
}

// ServerEvent_LoadoutView is the oneof wrapper for LoadoutView in ServerEvent.Payload.
// It is defined here (not in the generated game.pb.go) because LoadoutView is a
// hand-written type and is not a registered protoreflect.Message.
type ServerEvent_LoadoutView struct {
	LoadoutView *LoadoutView `protobuf:"bytes,33,opt,name=loadout_view,json=loadoutView,proto3,oneof"`
}

func (*ServerEvent_LoadoutView) isServerEvent_Payload() {}
