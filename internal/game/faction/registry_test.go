package faction_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/cory-johannsen/mud/internal/game/faction"
	"github.com/stretchr/testify/require"
)

func writeFactionYAML(t *testing.T, dir, filename, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatalf("write yaml: %v", err)
	}
}

const validGunYAML = `
id: gun
name: Team Gun
zone_id: rustbucket
hostile_factions: [machete]
tiers:
  - id: outsider
    label: Outsider
    min_rep: 0
    price_discount: 0.0
  - id: gunhand
    label: Gunhand
    min_rep: 100
    price_discount: 0.05
  - id: sharpshooter
    label: Sharpshooter
    min_rep: 300
    price_discount: 0.10
  - id: warchief
    label: Warchief
    min_rep: 600
    price_discount: 0.15
`

const validMacheteYAML = `
id: machete
name: Team Machete
zone_id: ironyard
hostile_factions: [gun]
tiers:
  - id: outsider
    label: Outsider
    min_rep: 0
    price_discount: 0.0
  - id: blade
    label: Blade
    min_rep: 100
    price_discount: 0.05
  - id: cutter
    label: Cutter
    min_rep: 300
    price_discount: 0.10
  - id: warsmith
    label: Warsmith
    min_rep: 600
    price_discount: 0.15
`

func TestLoadFactions_Happy(t *testing.T) {
	dir := t.TempDir()
	writeFactionYAML(t, dir, "gun.yaml", validGunYAML)
	writeFactionYAML(t, dir, "machete.yaml", validMacheteYAML)

	reg, err := faction.LoadFactions(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reg) != 2 {
		t.Fatalf("expected 2 factions, got %d", len(reg))
	}
	if _, ok := reg["gun"]; !ok {
		t.Error("expected 'gun' faction in registry")
	}
}

func TestLoadFactions_ValidationFailure(t *testing.T) {
	dir := t.TempDir()
	// id: with no value is valid YAML (empty string), but fails FactionDef.Validate() due to missing required fields.
	writeFactionYAML(t, dir, "bad.yaml", `id: `)
	_, err := faction.LoadFactions(dir)
	if err == nil {
		t.Fatal("expected error for faction that fails validation")
	}
}

func TestLoadFactions_MalformedYAML(t *testing.T) {
	dir := t.TempDir()
	writeFactionYAML(t, dir, "bad.yaml", `{bad: [unclosed`)
	_, err := faction.LoadFactions(dir)
	if err == nil {
		t.Fatal("expected error for malformed YAML")
	}
}

func TestFactionRegistry_ValidateHostileFactionsMustExist(t *testing.T) {
	dir := t.TempDir()
	yamlContent := `
id: gun
name: Team Gun
zone_id: rustbucket
hostile_factions: [nonexistent]
tiers:
  - {id: t1, label: L1, min_rep: 0, price_discount: 0.0}
  - {id: t2, label: L2, min_rep: 10, price_discount: 0.05}
  - {id: t3, label: L3, min_rep: 20, price_discount: 0.10}
  - {id: t4, label: L4, min_rep: 30, price_discount: 0.15}
`
	writeFactionYAML(t, dir, "gun.yaml", yamlContent)
	reg, err := faction.LoadFactions(dir)
	if err != nil {
		t.Fatalf("LoadFactions: %v", err)
	}
	zoneIDs := map[string]bool{"rustbucket": true}
	roomIDs := map[string]bool{}
	itemIDs := map[string]bool{}
	roomZoneIDs := map[string]string{}
	zoneOwners := map[string]string{"rustbucket": "gun"}
	if err := reg.Validate(zoneIDs, roomIDs, itemIDs, roomZoneIDs, zoneOwners); err == nil {
		t.Fatal("expected error for unknown hostile faction reference")
	}
}

// minimalFactionYAML returns a minimal valid single-faction YAML with the given id and zone_id.
// It includes exactly four tiers so FactionDef.Validate() passes, plus any extra fields appended.
func minimalFactionYAML(id, zoneID, extra string) string {
	return `
id: ` + id + `
name: ` + id + ` team
zone_id: ` + zoneID + `
` + extra + `
tiers:
  - {id: t1, label: L1, min_rep: 0, price_discount: 0.0}
  - {id: t2, label: L2, min_rep: 10, price_discount: 0.05}
  - {id: t3, label: L3, min_rep: 20, price_discount: 0.10}
  - {id: t4, label: L4, min_rep: 30, price_discount: 0.15}
`
}

func TestFactionRegistry_Validate_ExclusiveItemConflict(t *testing.T) {
	dir := t.TempDir()
	// Both factions claim the same item exclusively.
	gunExtra := `exclusive_items:
  - tier_id: t1
    item_ids: [shared_item]`
	macheteExtra := `exclusive_items:
  - tier_id: t1
    item_ids: [shared_item]`
	writeFactionYAML(t, dir, "gun.yaml", minimalFactionYAML("gun", "rustbucket", gunExtra))
	writeFactionYAML(t, dir, "machete.yaml", minimalFactionYAML("machete", "ironyard", macheteExtra))

	reg, err := faction.LoadFactions(dir)
	if err != nil {
		t.Fatalf("LoadFactions: %v", err)
	}
	zoneIDs := map[string]bool{"rustbucket": true, "ironyard": true}
	roomIDs := map[string]bool{}
	itemIDs := map[string]bool{"shared_item": true}
	roomZoneIDs := map[string]string{}
	zoneOwners := map[string]string{"rustbucket": "gun", "ironyard": "machete"}
	if err := reg.Validate(zoneIDs, roomIDs, itemIDs, roomZoneIDs, zoneOwners); err == nil {
		t.Fatal("expected error for item claimed exclusively by two factions")
	}
}

func TestFactionRegistry_Validate_GatedRoomNoZoneMapping(t *testing.T) {
	dir := t.TempDir()
	extra := `gated_rooms:
  - room_id: room1
    min_tier_id: t1`
	writeFactionYAML(t, dir, "gun.yaml", minimalFactionYAML("gun", "rustbucket", extra))

	reg, err := faction.LoadFactions(dir)
	if err != nil {
		t.Fatalf("LoadFactions: %v", err)
	}
	zoneIDs := map[string]bool{"rustbucket": true}
	roomIDs := map[string]bool{"room1": true}
	itemIDs := map[string]bool{}
	roomZoneIDs := map[string]string{} // room1 has no zone mapping
	zoneOwners := map[string]string{"rustbucket": "gun"}
	if err := reg.Validate(zoneIDs, roomIDs, itemIDs, roomZoneIDs, zoneOwners); err == nil {
		t.Fatal("expected error for gated room with no zone mapping")
	}
}

func TestFactionRegistry_Validate_GatedRoomUnownedZone(t *testing.T) {
	dir := t.TempDir()
	extra := `gated_rooms:
  - room_id: room1
    min_tier_id: t1`
	writeFactionYAML(t, dir, "gun.yaml", minimalFactionYAML("gun", "rustbucket", extra))

	reg, err := faction.LoadFactions(dir)
	if err != nil {
		t.Fatalf("LoadFactions: %v", err)
	}
	zoneIDs := map[string]bool{"rustbucket": true}
	roomIDs := map[string]bool{"room1": true}
	itemIDs := map[string]bool{}
	roomZoneIDs := map[string]string{"room1": "rustbucket"}
	zoneOwners := map[string]string{} // rustbucket has no owner
	if err := reg.Validate(zoneIDs, roomIDs, itemIDs, roomZoneIDs, zoneOwners); err == nil {
		t.Fatal("expected error for gated room in zone with no faction owner")
	}
}

func TestDSFAndJOFFactionsLoad(t *testing.T) {
	root := func() string {
		_, thisFile, _, _ := runtime.Caller(0)
		dir := filepath.Dir(thisFile)
		for {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				return dir
			}
			dir = filepath.Dir(dir)
		}
	}()
	reg, err := faction.LoadFactions(filepath.Join(root, "content", "factions"))
	require.NoError(t, err)
	dsf, ok := reg["dick_sucking_factory"]
	require.True(t, ok, "dick_sucking_factory faction must be registered")
	jof, ok := reg["jerk_off_factory"]
	require.True(t, ok, "jerk_off_factory faction must be registered")
	require.Contains(t, dsf.HostileFactions, "jerk_off_factory")
	require.Contains(t, jof.HostileFactions, "dick_sucking_factory")
}
