package ruleset_test

import (
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/game/ruleset"
)

func TestRegion_DisplayName_WithArticle(t *testing.T) {
	r := &ruleset.Region{Name: "Northeast", Article: "the"}
	if got := r.DisplayName(); got != "the Northeast" {
		t.Errorf("DisplayName() = %q, want %q", got, "the Northeast")
	}
}

func TestRegion_DisplayName_WithoutArticle(t *testing.T) {
	r := &ruleset.Region{Name: "Gresham Outskirts", Article: ""}
	if got := r.DisplayName(); got != "Gresham Outskirts" {
		t.Errorf("DisplayName() = %q, want %q", got, "Gresham Outskirts")
	}
}

func TestProperty_Region_DisplayName_NonEmpty(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringMatching(`[A-Za-z ]+`).Draw(t, "name")
		article := rapid.SampledFrom([]string{"", "the", "a"}).Draw(t, "article")
		r := &ruleset.Region{Name: name, Article: article}
		got := r.DisplayName()
		if got == "" {
			t.Fatalf("DisplayName() must not be empty for non-empty name %q", name)
		}
	})
}
