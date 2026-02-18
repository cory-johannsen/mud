package importer_test

import (
	"testing"
	"unicode"

	"github.com/cory-johannsen/mud/internal/importer"
	"github.com/stretchr/testify/assert"
	"pgregory.net/rapid"
)

func TestNameToID_Lowercase(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringOf(rapid.RuneFrom(nil, unicode.Letter, unicode.Digit)).Draw(t, "name")
		id := importer.NameToID(name)
		for _, r := range id {
			assert.True(t, r == '_' || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'),
				"unexpected char %q in id %q", r, id)
		}
	})
}

func TestNameToID_Idempotent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringOf(rapid.RuneFrom(nil, unicode.Letter, unicode.Digit)).Draw(t, "name")
		id := importer.NameToID(name)
		assert.Equal(t, id, importer.NameToID(id))
	})
}

func TestNameToID_NoSpacesOrApostrophes(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		name := rapid.StringOf(rapid.RuneFrom(nil, unicode.Letter, unicode.Space)).Draw(t, "name")
		id := importer.NameToID(name)
		assert.NotContains(t, id, " ")
		assert.NotContains(t, id, "'")
	})
}

func TestNameToID_KnownValues(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"Grinder's Row", "grinders_row"},
		{"The Rusty Oasis", "the_rusty_oasis"},
		{"Scrapshack 23", "scrapshack_23"},
		{"Rustbucket Ridge", "rustbucket_ridge"},
		{"Filth Court", "filth_court"},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			assert.Equal(t, tc.want, importer.NameToID(tc.input))
		})
	}
}
