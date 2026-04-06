package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestToSnakeCase_TableDriven(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Corrosive Projectile", "corrosive_projectile"},
		{"Cranial Shock", "cranial_shock"},
		{"Chrome Reflex", "chrome_reflex"},
		{"K'galaserke's Axes", "kgalaserkes_axes"},
		{"100 Volt Shock", "100_volt_shock"},
		{"  Trim   Me  ", "trim_me"},
		{"Acid Storm", "acid_storm"},
		{"Single", "single"},
		{"Already_snake", "already_snake"},
		{"Hyphens-Are-Removed", "hyphens_are_removed"},
		{"Dots.Are.Removed", "dots_are_removed"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, ToSnakeCase(tc.input))
		})
	}
}

func TestToSnakeCase_Property_OutputOnlySnakeChars(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.String().Draw(rt, "name")
		result := ToSnakeCase(input)
		for _, r := range result {
			if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
				rt.Fatalf("ToSnakeCase(%q) = %q contains invalid char %q", input, result, r)
			}
		}
	})
}

func TestToSnakeCase_Property_NoLeadingOrTrailingUnderscore(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.String().Draw(rt, "name")
		result := ToSnakeCase(input)
		if len(result) > 0 {
			if result[0] == '_' {
				rt.Fatalf("ToSnakeCase(%q) = %q starts with underscore", input, result)
			}
			if result[len(result)-1] == '_' {
				rt.Fatalf("ToSnakeCase(%q) = %q ends with underscore", input, result)
			}
		}
	})
}

func TestToSnakeCase_Property_NoConsecutiveUnderscores(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		input := rapid.String().Draw(rt, "name")
		result := ToSnakeCase(input)
		for i := 0; i < len(result)-1; i++ {
			if result[i] == '_' && result[i+1] == '_' {
				rt.Fatalf("ToSnakeCase(%q) = %q has consecutive underscores", input, result)
			}
		}
	})
}

func TestStripTraditionSuffix(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"acid_arrow_technical", "acid_arrow"},
		{"daze_neural", "daze"},
		{"sleep_bio_synthetic", "sleep"},
		{"bless_fanatic_doctrine", "bless"},
		{"chrome_reflex", "chrome_reflex"}, // no suffix
		{"neural_static", "neural_static"}, // no suffix
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, stripTraditionSuffix(tc.input))
		})
	}
}

func TestIsPF2EFlagged(t *testing.T) {
	cases := []struct {
		name   string
		oldID  string
		wantFl bool
		desc   string
	}{
		// REQ-TIR-PF2: name never localized — derived matches stripped old_id
		{"Acid Arrow", "acid_arrow_technical", true, "PF2E name unchanged"},
		{"Daze", "daze_neural", true, "PF2E name unchanged single word"},
		// REQ-TIR-PF3: keyword deny-list
		{"Antimagic Field", "antimagic_field_neural", true, "keyword: antimagic"},
		{"Scrying Lens", "scrying_lens_neural", true, "keyword: scrying"},
		// Already Gunchete — no flag
		{"Corrosive Projectile", "acid_arrow_technical", false, "localized name"},
		{"Cranial Shock", "daze_neural", false, "localized name"},
		{"Chrome Reflex", "chrome_reflex", false, "innate already correct"},
		{"Neural Static", "neural_static", false, "innate already correct"},
	}
	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			assert.Equal(t, tc.wantFl, IsPF2EFlagged(tc.name, tc.oldID))
		})
	}
}
