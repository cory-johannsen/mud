package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// writeTechYAML writes a minimal tech YAML file to dir/subdir/filename.
func writeTechYAML(t *testing.T, dir, subdir, filename, id, name string) {
	t.Helper()
	sub := filepath.Join(dir, subdir)
	require.NoError(t, os.MkdirAll(sub, 0755))
	content := fmt.Sprintf("id: %s\nname: %s\ntradition: technical\nlevel: 1\nusage_type: prepared\n", id, name)
	require.NoError(t, os.WriteFile(filepath.Join(sub, filename), []byte(content), 0644))
}

func TestBuildRenameMap_Basic(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, "technical", "acid_arrow_technical.yaml", "acid_arrow_technical", "Corrosive Projectile")
	writeTechYAML(t, dir, "neural", "daze_neural.yaml", "daze_neural", "Cranial Shock")
	writeTechYAML(t, dir, "innate", "chrome_reflex.yaml", "chrome_reflex", "Chrome Reflex")

	rm, err := BuildRenameMap(dir)
	require.NoError(t, err)
	require.Len(t, rm.Renames, 3)

	byOld := make(map[string]RenameEntry)
	for _, e := range rm.Renames {
		byOld[e.OldID] = e
	}

	e := byOld["acid_arrow_technical"]
	assert.Equal(t, "corrosive_projectile", e.NewID)
	assert.False(t, e.Skip)
	assert.False(t, e.PF2EFlag)
	assert.False(t, e.Collision)

	e = byOld["daze_neural"]
	assert.Equal(t, "cranial_shock", e.NewID)
	assert.False(t, e.Skip)

	e = byOld["chrome_reflex"]
	assert.Equal(t, "chrome_reflex", e.NewID)
	assert.True(t, e.Skip, "already-correct IDs must be marked skip")
}

func TestBuildRenameMap_CollisionDetected(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, "technical", "acid_arrow_technical.yaml", "acid_arrow_technical", "Shock Wave")
	writeTechYAML(t, dir, "neural", "daze_neural.yaml", "daze_neural", "Shock Wave")

	rm, err := BuildRenameMap(dir)
	require.NoError(t, err)

	for _, e := range rm.Renames {
		assert.True(t, e.Collision, "both entries deriving same new_id must be flagged collision: old=%s", e.OldID)
	}
}

func TestBuildRenameMap_PF2EFlagSet(t *testing.T) {
	dir := t.TempDir()
	// Name never localized — derives same ID as old (minus suffix)
	writeTechYAML(t, dir, "technical", "acid_arrow_technical.yaml", "acid_arrow_technical", "Acid Arrow")

	rm, err := BuildRenameMap(dir)
	require.NoError(t, err)
	require.Len(t, rm.Renames, 1)
	assert.True(t, rm.Renames[0].PF2EFlag)
}

func TestBuildRenameMap_SortedByOldID(t *testing.T) {
	dir := t.TempDir()
	writeTechYAML(t, dir, "technical", "z_tech.yaml", "z_tech", "Zap")
	writeTechYAML(t, dir, "technical", "a_tech.yaml", "a_tech", "Alpha Strike")

	rm, err := BuildRenameMap(dir)
	require.NoError(t, err)
	require.Len(t, rm.Renames, 2)
	assert.True(t, rm.Renames[0].OldID < rm.Renames[1].OldID, "entries must be sorted by old_id")
}
