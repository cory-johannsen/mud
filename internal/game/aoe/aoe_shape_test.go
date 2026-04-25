package aoe

import (
	"strings"
	"testing"
)

func TestValidateAoeFields_Legacy(t *testing.T) {
	// No shape, no radius → no AoE, no error.
	if err := ValidateAoeFields(AoeShapeNone, 0, 0, 0); err != nil {
		t.Fatalf("expected nil for empty shape with no radius; got %v", err)
	}
	// Legacy burst back-compat: no shape, positive radius.
	if err := ValidateAoeFields(AoeShapeNone, 10, 0, 0); err != nil {
		t.Fatalf("expected nil for legacy aoe_radius back-compat; got %v", err)
	}
	// Legacy form must reject aoe_length / aoe_width.
	if err := ValidateAoeFields(AoeShapeNone, 0, 10, 0); err == nil {
		t.Fatal("expected error when aoe_length set without aoe_shape")
	}
	if err := ValidateAoeFields(AoeShapeNone, 0, 0, 5); err == nil {
		t.Fatal("expected error when aoe_width set without aoe_shape")
	}
}

func TestValidateAoeFields_Burst(t *testing.T) {
	if err := ValidateAoeFields(AoeShapeBurst, 10, 0, 0); err != nil {
		t.Fatalf("expected nil for burst with radius; got %v", err)
	}
	if err := ValidateAoeFields(AoeShapeBurst, 0, 0, 0); err == nil {
		t.Fatal("expected error: burst requires aoe_radius > 0")
	}
	err := ValidateAoeFields(AoeShapeBurst, 10, 30, 0)
	if err == nil || !strings.Contains(err.Error(), "aoe_length") {
		t.Fatalf("expected aoe_length error for burst+length; got %v", err)
	}
}

func TestValidateAoeFields_Cone(t *testing.T) {
	if err := ValidateAoeFields(AoeShapeCone, 0, 30, 0); err != nil {
		t.Fatalf("expected nil for cone with length; got %v", err)
	}
	err := ValidateAoeFields(AoeShapeCone, 0, 0, 0)
	if err == nil || !strings.Contains(err.Error(), "aoe_length") {
		t.Fatalf("expected aoe_length error for cone with no length; got %v", err)
	}
	if err := ValidateAoeFields(AoeShapeCone, 10, 30, 0); err == nil {
		t.Fatal("expected error: cone does not accept aoe_radius")
	}
}

func TestValidateAoeFields_Line(t *testing.T) {
	// Line with length, default width.
	if err := ValidateAoeFields(AoeShapeLine, 0, 30, 0); err != nil {
		t.Fatalf("expected nil for line with length and default width; got %v", err)
	}
	// Line with explicit width.
	if err := ValidateAoeFields(AoeShapeLine, 0, 30, 10); err != nil {
		t.Fatalf("expected nil for line with explicit width; got %v", err)
	}
	err := ValidateAoeFields(AoeShapeLine, 0, 0, 5)
	if err == nil || !strings.Contains(err.Error(), "aoe_length") {
		t.Fatalf("expected aoe_length error for line with no length; got %v", err)
	}
	if err := ValidateAoeFields(AoeShapeLine, 10, 30, 0); err == nil {
		t.Fatal("expected error: line does not accept aoe_radius")
	}
}

func TestValidateAoeFields_NegativeFields(t *testing.T) {
	if err := ValidateAoeFields(AoeShapeNone, -1, 0, 0); err == nil {
		t.Fatal("expected error for negative radius")
	}
	if err := ValidateAoeFields(AoeShapeCone, 0, -1, 0); err == nil {
		t.Fatal("expected error for negative length")
	}
	if err := ValidateAoeFields(AoeShapeLine, 0, 30, -1); err == nil {
		t.Fatal("expected error for negative width")
	}
}

func TestValidateAoeFields_UnknownShape(t *testing.T) {
	if err := ValidateAoeFields(AoeShape("orb"), 0, 0, 0); err == nil {
		t.Fatal("expected error for unknown shape")
	}
}

func TestResolveLineWidth(t *testing.T) {
	if got := ResolveLineWidth(0); got != DefaultLineWidthFt {
		t.Fatalf("ResolveLineWidth(0) = %d; want %d", got, DefaultLineWidthFt)
	}
	if got := ResolveLineWidth(15); got != 15 {
		t.Fatalf("ResolveLineWidth(15) = %d; want 15", got)
	}
}
