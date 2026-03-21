// Package trap implements trap templates, runtime state, danger scaling,
// payload resolution, and procedural placement for the MUD game.
package trap

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// TrapTrigger identifies when a trap fires.
type TrapTrigger string

const (
	TriggerEntry         TrapTrigger = "entry"
	TriggerInteraction   TrapTrigger = "interaction"
	TriggerPressurePlate TrapTrigger = "pressure_plate"
	TriggerRegion        TrapTrigger = "region"
)

// ResetMode controls lifecycle after a trap fires.
type ResetMode string

const (
	ResetOneShot ResetMode = "one_shot"
	ResetAuto    ResetMode = "auto"
	ResetManual  ResetMode = "manual"
)

// TrapPayload describes the effect delivered when a trap fires.
// Exactly one of (Type fields) applies per payload.
type TrapPayload struct {
	// Type is one of: mine | pit | bear_trap | trip_wire | honkeypot
	Type            string `yaml:"type"`
	Damage          string `yaml:"damage,omitempty"`
	Condition       string `yaml:"condition,omitempty"`
	SaveType        string `yaml:"save_type,omitempty"`
	SaveDC          int    `yaml:"save_dc,omitempty"`
	TechnologyEffect string `yaml:"technology_effect,omitempty"`
}

// DangerScalingEntry holds per-danger-level bonus overrides for one template.
type DangerScalingEntry struct {
	DamageBonus    string `yaml:"damage_bonus,omitempty"`
	SaveDCBonus    int    `yaml:"save_dc_bonus,omitempty"`
	StealthDCBonus int    `yaml:"stealth_dc_bonus,omitempty"`
	DisableDCBonus int    `yaml:"disable_dc_bonus,omitempty"`
}

// DangerScalingTier holds per-template overrides for each danger level.
type DangerScalingTier struct {
	Sketchy    *DangerScalingEntry `yaml:"sketchy,omitempty"`
	Dangerous  *DangerScalingEntry `yaml:"dangerous,omitempty"`
	AllOutWar  *DangerScalingEntry `yaml:"all_out_war,omitempty"`
}

// TrapTemplate is the static definition loaded from content/traps/<id>.yaml.
//
// Precondition: ID must match the filename stem.
// Postcondition: LoadTrapTemplate validates REQ-TR-11 (Pressure Plate must not reference another Pressure Plate).
type TrapTemplate struct {
	ID              string            `yaml:"id"`
	Name            string            `yaml:"name"`
	Description     string            `yaml:"description"`
	Trigger         TrapTrigger       `yaml:"trigger"`
	TargetRegions   []string          `yaml:"target_regions,omitempty"`
	TriggerAction   string            `yaml:"trigger_action,omitempty"`
	Payload         *TrapPayload      `yaml:"payload,omitempty"`
	PayloadTemplate string            `yaml:"payload_template,omitempty"`
	StealthDC       int               `yaml:"stealth_dc"`
	DisableDC       int               `yaml:"disable_dc"`
	ResetMode       ResetMode         `yaml:"reset_mode"`
	ResetTimer      string            `yaml:"reset_timer,omitempty"`
	DangerScaling   *DangerScalingTier `yaml:"danger_scaling,omitempty"`
}

// LoadTrapTemplate loads and validates a single trap template from path.
//
// Precondition: path must be readable and contain valid YAML.
// Postcondition: Returns error if a pressure_plate trigger has an empty payload_template field.
// Full REQ-TR-11 cross-template validation (no pressure_plate may chain to another pressure_plate)
// is performed only by LoadTrapTemplates after all templates are loaded.
func LoadTrapTemplate(path string) (*TrapTemplate, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading trap template %q: %w", path, err)
	}
	var tmpl TrapTemplate
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parsing trap template %q: %w", path, err)
	}
	// REQ-TR-11: Pressure Plate payload_template must not be a Pressure Plate itself.
	// At single-file load time we can only check if the template itself is pressure_plate
	// and its payload_template value matches its own trigger; cross-template validation
	// is performed in LoadTrapTemplates after all files are loaded.
	if tmpl.Trigger == TriggerPressurePlate && tmpl.PayloadTemplate == "" {
		return nil, fmt.Errorf("trap template %q: pressure_plate trigger requires payload_template", tmpl.ID)
	}
	return &tmpl, nil
}

// LoadTrapTemplates loads all *.yaml files in dir as trap templates, keyed by ID.
//
// Precondition: dir must be a readable directory containing valid trap YAML files.
// Postcondition: Returns error if any file fails to load or if REQ-TR-11 is violated across the loaded set.
func LoadTrapTemplates(dir string) (map[string]*TrapTemplate, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading trap templates dir %q: %w", dir, err)
	}
	templates := make(map[string]*TrapTemplate)
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		// Skip defaults.yaml — it is not a template.
		if e.Name() == "defaults.yaml" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		tmpl, err := LoadTrapTemplate(path)
		if err != nil {
			return nil, err
		}
		templates[tmpl.ID] = tmpl
	}
	// REQ-TR-11: Validate that no Pressure Plate references another Pressure Plate.
	for id, tmpl := range templates {
		if tmpl.Trigger == TriggerPressurePlate {
			linked, ok := templates[tmpl.PayloadTemplate]
			if !ok {
				return nil, fmt.Errorf("trap template %q: payload_template %q not found in loaded templates", id, tmpl.PayloadTemplate)
			}
			if linked.Trigger == TriggerPressurePlate {
				return nil, fmt.Errorf("trap template %q (pressure_plate): payload_template %q is also a pressure_plate — REQ-TR-11 violation", id, tmpl.PayloadTemplate)
			}
		}
	}
	return templates, nil
}
