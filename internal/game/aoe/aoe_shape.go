// Package aoe defines the shared AoE (area-of-effect) shape vocabulary used by
// content types that may produce a templated area effect — feats, class
// features, technologies, and explosives.
//
// The package contains only value-typed declarations and pure validation; it
// imports nothing from the rest of the game so it can be safely depended on by
// every content package without risking an import cycle.
package aoe

import "fmt"

// AoeShape names the geometric template a content definition produces.
//
// AoeShapeNone (the zero value, "") indicates no AoE — single-target.
// AoeShapeBurst is a Chebyshev-radius burst centered on a target cell.
// AoeShapeCone is a PF2E cone anchored at an apex with a facing.
// AoeShapeLine is a PF2E line with length and width centered on a facing.
type AoeShape string

const (
	// AoeShapeNone is the zero value indicating no area effect.
	AoeShapeNone AoeShape = ""
	// AoeShapeBurst is a square (Chebyshev) burst centered on a cell.
	AoeShapeBurst AoeShape = "burst"
	// AoeShapeCone is a PF2E cone anchored at an apex.
	AoeShapeCone AoeShape = "cone"
	// AoeShapeLine is a PF2E line oriented along a facing.
	AoeShapeLine AoeShape = "line"
)

// DefaultLineWidthFt is the width applied when aoe_shape is "line" but
// aoe_width is unset or zero. Per AOE-3 a default line is 5 ft wide
// (one cell across).
const DefaultLineWidthFt = 5

// IsValidShape reports whether s is a recognised AoeShape value (including
// the empty AoeShapeNone).
func IsValidShape(s AoeShape) bool {
	switch s {
	case AoeShapeNone, AoeShapeBurst, AoeShapeCone, AoeShapeLine:
		return true
	}
	return false
}

// ValidateAoeFields enforces the AoE-shape field rules across all content
// types that carry an AoE template.
//
// Rules (AOE-3 / AOE-4 / AOE-5 / AOE-6):
//
//   - aoe_shape == "" and aoe_radius == 0 → no AoE, no error.
//   - aoe_shape == "" and aoe_radius > 0  → legacy burst form (back-compat).
//     aoe_length and aoe_width must be 0.
//   - aoe_shape == "burst" → aoe_radius must be > 0; aoe_length must be 0.
//     aoe_width is ignored.
//   - aoe_shape == "cone"  → aoe_length must be > 0; aoe_radius and
//     aoe_width must be 0.
//   - aoe_shape == "line"  → aoe_length must be > 0; aoe_radius must be 0.
//     aoe_width may be 0 (caller defaults to DefaultLineWidthFt) or > 0.
//
// All numeric fields must be non-negative.
//
// Precondition: none.
// Postcondition: returns nil iff the combination is legal under the rules
// above; otherwise returns a non-nil error whose message names the offending
// field(s).
func ValidateAoeFields(shape AoeShape, radiusFt, lengthFt, widthFt int) error {
	if !IsValidShape(shape) {
		return fmt.Errorf("aoe_shape %q is not a recognised shape", shape)
	}
	if radiusFt < 0 {
		return fmt.Errorf("aoe_radius must be >= 0, got %d", radiusFt)
	}
	if lengthFt < 0 {
		return fmt.Errorf("aoe_length must be >= 0, got %d", lengthFt)
	}
	if widthFt < 0 {
		return fmt.Errorf("aoe_width must be >= 0, got %d", widthFt)
	}
	switch shape {
	case AoeShapeNone:
		// No explicit shape: legacy form. Only aoe_radius may be set.
		if lengthFt > 0 {
			return fmt.Errorf("aoe_length is only valid with aoe_shape: cone or line")
		}
		if widthFt > 0 {
			return fmt.Errorf("aoe_width is only valid with aoe_shape: line")
		}
	case AoeShapeBurst:
		if radiusFt <= 0 {
			return fmt.Errorf("aoe_shape: burst requires aoe_radius > 0")
		}
		if lengthFt > 0 {
			return fmt.Errorf("aoe_shape: burst does not accept aoe_length")
		}
		if widthFt > 0 {
			return fmt.Errorf("aoe_shape: burst does not accept aoe_width")
		}
	case AoeShapeCone:
		if lengthFt <= 0 {
			return fmt.Errorf("aoe_shape: cone requires aoe_length > 0")
		}
		if radiusFt > 0 {
			return fmt.Errorf("aoe_shape: cone does not accept aoe_radius")
		}
		if widthFt > 0 {
			return fmt.Errorf("aoe_shape: cone does not accept aoe_width")
		}
	case AoeShapeLine:
		if lengthFt <= 0 {
			return fmt.Errorf("aoe_shape: line requires aoe_length > 0")
		}
		if radiusFt > 0 {
			return fmt.Errorf("aoe_shape: line does not accept aoe_radius")
		}
		// widthFt may be 0 (caller defaults) or any positive value.
	}
	return nil
}

// ResolveLineWidth returns the effective line width in feet given the raw
// widthFt field. Callers that have already passed ValidateAoeFields with
// AoeShapeLine should use this helper to substitute the default when the
// raw field is zero.
//
// Precondition: shape == AoeShapeLine; widthFt >= 0.
// Postcondition: returns DefaultLineWidthFt when widthFt == 0, else widthFt.
func ResolveLineWidth(widthFt int) int {
	if widthFt == 0 {
		return DefaultLineWidthFt
	}
	return widthFt
}
