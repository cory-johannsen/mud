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
