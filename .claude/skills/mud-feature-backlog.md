---
name: mud-feature-backlog
description: Use when reading, updating, or managing the product feature backlog — adding features, changing priorities, updating status or effort, querying the backlog by category or status.
---

# Feature Backlog Management

## Canonical files

- **Index:** `docs/features/index.yaml` — priority-ordered manifest, one entry per slug
- **Feature files:** `docs/features/<slug>.md` — requirement checklists, one per feature

`FEATURES.md` is a deprecated redirect stub. Never edit it.

## Index schema

All fields required unless noted:

```yaml
- slug: kebab-case-id          # unique; matches filename for own-file features
  name: Human Readable Name
  status: done                 # backlog | spec | planned | in_progress | done | blocked
  priority: 10                 # unique positive integer; lower = higher priority
  category: combat             # combat | character | technology | world | ui | meta
  file: docs/features/foo.md  # authoritative path (may differ from slug for consolidated files)
  effort: "not defined"        # "-" | XS | S | M | L | XL | "not defined"
  dependencies:                # optional; list of slugs that must be done first
    - other-slug
```

## Status rules

Statuses follow a linear lifecycle. `blocked` is a special state any status can transition into.

```
backlog → spec → planned → in_progress → done
       ↕       ↕           ↕
            blocked (any status can become blocked if a dependency is not yet done)
```

| Status | Meaning |
|---|---|
| `backlog` | Feature identified but not yet designed or specced |
| `spec` | Feature spec written; implementation plan not yet written |
| `planned` | Implementation plan written; work not yet started |
| `in_progress` | Implementation underway (at least one checklist item `[x]`) |
| `done` | All checklist items `[x]`; set `effort: "-"` |
| `blocked` | Cannot proceed due to an unfinished dependency; restore prior status when unblocked |

**When marking a feature `done`:** verify the feature file's checklist items are all `[x]` before setting status. If the file has unchecked items, either update the file or use `in_progress`.

## Priority rules

- The YAML list order MUST match ascending `priority` values (lowest first)
- `priority` values MUST be unique integers
- Use multiples of 10 (10, 20, 30…) to leave gaps for insertion
- Sibling slugs sharing one file use consecutive integers (e.g. 90, 91)
- To reprioritize: update the `priority` value AND move the entry to the correct position in the list

## Common operations

### View backlog
```bash
# All features sorted by priority
grep -E "slug:|priority:|status:|effort:" docs/features/index.yaml

# Features by status
grep -B1 "status: backlog" docs/features/index.yaml | grep slug
grep -B1 "status: spec" docs/features/index.yaml | grep slug
grep -B1 "status: planned" docs/features/index.yaml | grep slug

# Unplanned features (backlog + spec) sorted by priority
grep -E "slug:|priority:|status:" docs/features/index.yaml | \
  paste - - - | grep -E "status: backlog|status: spec" | sort -t: -k4 -n
grep -B1 "status: in_progress" docs/features/index.yaml | grep slug
grep -B1 "status: blocked" docs/features/index.yaml | grep slug

# Features by category
grep -B3 "category: combat" docs/features/index.yaml | grep slug
```

### Update status
1. Read the feature file checklist to confirm actual state
2. Update `status:` in index.yaml to match
3. If marking `done`: set `effort: "-"`

### Reprioritize
1. Change the `priority:` value
2. Move the entry to the correct position in the list (list order = priority order)
3. Renumber surrounding entries if needed to maintain unique values

### Update effort
Valid values: `"-"` (done), `XS`, `S`, `M`, `L`, `XL`, `"not defined"`

### Add a new feature
1. Create `docs/features/<slug>.md` (heading + description + `## Requirements` + checklists)
2. Add entry to `docs/features/index.yaml` at desired priority position
3. Use multiples of 10 for priority; shift adjacent entries if needed

### Verify index integrity
```bash
# Slug count should match expected total
grep "^  - slug:" docs/features/index.yaml | wc -l

# No duplicate slugs
grep "^  - slug:" docs/features/index.yaml | sort | uniq -d

# No duplicate priorities
grep "  priority:" docs/features/index.yaml | awk '{print $2}' | sort -n | uniq -d

# All referenced files exist
grep "file: docs/features/" docs/features/index.yaml | \
  awk '{print $2}' | sort -u | while read f; do
    [ -f "$f" ] && echo "OK: $f" || echo "MISSING: $f"
  done
```

## Commit convention
```bash
git add docs/features/index.yaml docs/features/<slug>.md
git commit -m "docs(backlog): <description of change>"
```
