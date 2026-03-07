# Modifier-Only Ability Score Display Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Remove raw ability scores from all display surfaces; show only the PF2E modifier (e.g. `+2` instead of `+2 (14)`).

**Architecture:** Single function change to `abilityBonus` in `internal/frontend/handlers/text_renderer.go`. Update its one test assertion and update FEATURES.md. No DB, proto, or command changes.

**Tech Stack:** Go, existing telnet text renderer.

---

### Task 1: Drop raw score from `abilityBonus` and update tests

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go:348-356`
- Modify: `internal/frontend/handlers/text_renderer_test.go` (any assertions checking `(14)`-style output)

**Step 1: Write the failing test**

In `internal/frontend/handlers/text_renderer_test.go`, add (or update) a test:

```go
func TestAbilityBonus_ModifierOnly(t *testing.T) {
    // Score 14 → modifier +2, no raw score in parens
    result := RenderCharacterSheet(&gamev1.CharacterSheetView{
        Name:      "Hero",
        Level:     1,
        Brutality: 14,
    }, 80)
    assert.Contains(t, result, "BRT: +2")
    assert.NotContains(t, result, "(14)")
}
```

Run: `cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -run TestAbilityBonus_ModifierOnly -v`
Expected: FAIL (currently renders `+2 (14)`)

**Step 2: Change `abilityBonus`**

Replace lines 348–356 of `internal/frontend/handlers/text_renderer.go`:

```go
// abilityBonus formats an ability score as its PF2E modifier only.
// e.g. score 14 → "+2", score 10 → "+0", score 8 → "-1"
func abilityBonus(score int32) string {
    mod := (score - 10) / 2
    if mod >= 0 {
        return fmt.Sprintf("+%d", mod)
    }
    return fmt.Sprintf("%d", mod)
}
```

**Step 3: Run tests to verify they pass**

Run: `cd /home/cjohannsen/src/mud && go test ./internal/frontend/handlers/... -timeout 60s`
Expected: ok

Run: `go test ./... -timeout 180s 2>&1 | grep -E "^(ok|FAIL)"`
Expected: all ok (except pre-existing postgres timeout)

**Step 4: Update FEATURES.md**

Mark `[ ] Only show modifier bonuses` as `[x]`.

**Step 5: Commit and deploy**

```bash
git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go docs/requirements/FEATURES.md
git commit -m "feat: display modifier-only ability scores (drop raw score)"
make k8s-redeploy
```
