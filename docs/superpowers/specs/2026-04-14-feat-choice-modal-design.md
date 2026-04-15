# Feat Choice Modal Design

## Overview

When a player has unresolved feat choices (from a level-up pool), the web client must show a
notification on the character sheet and open a structured selection modal when clicked. The current
bug (issue #91) renders the raw string `"Choose 1: rage, Reactive Block, ..."` in the Job Drawer
instead of a proper UI.

---

## Requirements

- REQ-FCM-1: Unresolved feat choices MUST be represented as structured data in `JobGrantsResponse`, not as raw display strings in `JobFeatGrant.feat_name`.
- REQ-FCM-2: `JobGrantsResponse` MUST include a `repeated PendingFeatChoice pending_feat_choices` field carrying the full option pool (feat ID, name, description, category) and required selection count per grant level.
- REQ-FCM-3: The `CharacterPanel` MUST display a clickable notification badge when `jobGrants.pending_feat_choices` is non-empty.
- REQ-FCM-4: Clicking the notification badge MUST open the `FeatChoiceModal`.
- REQ-FCM-5: `FeatChoiceModal` MUST display each feat option with its name, category badge, and description.
- REQ-FCM-6: `FeatChoiceModal` MUST enforce the required selection count before enabling the Confirm button.
- REQ-FCM-7: On confirm, the client MUST send one `ChooseFeatRequest` per selected feat to the server.
- REQ-FCM-8: The server MUST validate the request (pending grant exists, feat in pool, not already owned) and deny invalid requests.
- REQ-FCM-9: On success, the server MUST persist the feat, mark the grant level as fulfilled in `character_feat_level_grants`, and push an updated `CharacterSheetView` and `JobGrantsResponse` to the player's stream.
- REQ-FCM-10: `JobDrawer` MUST NOT display raw "Choose N: ..." strings; unresolved grant levels MUST show a "Pending choice" badge instead.
- REQ-FCM-11: The notification badge and modal MUST follow the existing dark-theme monospace styling used by `AbilityBoostModal` and `SkillIncreaseModal`.

---

## Section 1: Protocol

### 1.1 New Proto Messages

```protobuf
// FeatOption describes a single selectable feat in a pending choice pool.
//
// Precondition: feat_id is a valid feat ID in the server's feat registry.
message FeatOption {
  string feat_id     = 1;
  string name        = 2;
  string description = 3;
  string category    = 4;
}

// PendingFeatChoice describes one unresolved feat choice pool at a specific grant level.
//
// Precondition: count >= 1; options is non-empty; grant_level >= 1.
message PendingFeatChoice {
  int32             grant_level = 1;
  int32             count       = 2;
  repeated FeatOption options   = 3;
}
```

### 1.2 Modified `JobGrantsResponse`

Add field 3:

```protobuf
message JobGrantsResponse {
  repeated JobFeatGrant      feat_grants          = 1;
  repeated JobTechGrant      tech_grants          = 2;
  repeated PendingFeatChoice pending_feat_choices = 3;  // new
}
```

### 1.3 New `ChooseFeatRequest` ClientMessage

Add as a new field in the `ClientMessage` oneof:

```protobuf
message ChooseFeatRequest {
  int32  grant_level = 1;
  string feat_id     = 2;
}
```

---

## Section 2: Server-side

### 2.1 `handleJobGrants()` modification (`internal/gameserver/grpc_service.go`)

For unresolved choice pool entries (where the player has not yet selected a feat for a given level):

- Set `JobFeatGrant.feat_id = ""` and `JobFeatGrant.feat_name = ""` (leave blank — the client will render a "Pending choice" badge for empty rows)
- Build a `PendingFeatChoice` for each unresolved level: look up each pool feat ID in the feat registry to get name, description, and category; set `count` from the pool's required count
- Append each `PendingFeatChoice` to `JobGrantsResponse.pending_feat_choices`

### 2.2 `handleChooseFeat()` (new file: `internal/gameserver/grpc_service_feat_choice.go`)

```
Precondition: uid non-empty; grant_level >= 1; feat_id non-empty.
Postcondition: On success — feat stored in character_feats, grant level marked in
               character_feat_level_grants, updated CharacterSheetView and
               JobGrantsResponse pushed to player stream.

1. Load player session by uid → error if not found
2. Call handleJobGrants() to retrieve current pending choices
3. Find PendingFeatChoice where grant_level matches request → denial if not found
4. Verify feat_id is in PendingFeatChoice.options → denial if not
5. Verify player does not already own feat_id → denial if already owned
6. Store feat via featsRepo.Add(characterID, feat_id, grant_level)
7. Mark level as granted in character_feat_level_grants
8. Push updated CharacterSheetView to player stream
9. Push updated JobGrantsResponse (re-call handleJobGrants) to player stream
10. Return success event
```

---

## Section 3: Client-side

### 3.1 `CharacterPanel.tsx` — notification badge

When `state.jobGrants?.pendingFeatChoices` is non-empty, render a badge below the XP bar
following the exact same pattern as the existing ability boost / skill increase notification
buttons:

```tsx
{(state.jobGrants?.pendingFeatChoices?.length ?? 0) > 0 && (
  <button onClick={() => setShowFeatChoiceModal(true)} style={pendingBadgeStyle}>
    {state.jobGrants!.pendingFeatChoices!.length} feat choice(s) available
  </button>
)}
{showFeatChoiceModal && (
  <FeatChoiceModal
    choices={state.jobGrants!.pendingFeatChoices!}
    onClose={() => setShowFeatChoiceModal(false)}
  />
)}
```

Local state: `const [showFeatChoiceModal, setShowFeatChoiceModal] = useState(false)`.

### 3.2 `FeatChoiceModal.tsx` (new component)

**Props:** `{ choices: PendingFeatChoice[]; onClose: () => void }`

**Behavior:**
- Shows one `PendingFeatChoice` at a time (index tracked with local state); if multiple pending levels exist, a "Next" button advances to the next after the current one is confirmed
- Renders a grid of feat cards: name (bold), category badge, description
- Click a card to toggle selection (green highlight); clicking a selected card deselects it
- Confirm button enabled only when selected count === `choice.count`
- On confirm: for each selected feat ID, calls `sendMessage('ChooseFeatRequest', { grantLevel: choice.grant_level, featId })` then advances to next choice or calls `onClose()`
- No click-outside dismiss (must confirm or exhaust choices)

**Styling:** Dark overlay (`#111` background, 80% opacity backdrop), monospace font, green
(`#8d4`) for selected cards, yellow (`#e0c060`) for header, matches `AbilityBoostModal`.

### 3.3 `JobDrawer.tsx` — suppress raw text

For `JobFeatGrant` rows where `feat_id === ""` and `feat_name === ""`:

```tsx
// Before (buggy):
<td>{grant.featName}</td>

// After:
<td>
  {grant.featId
    ? grant.featName
    : <span style={{ color: '#e0c060' }}>Pending choice</span>}
</td>
```

### 3.4 `GameContext.tsx` — no new dispatch needed

`pending_feat_choices` is a field on `JobGrantsResponse`, which already maps to
`state.jobGrants` via the existing `SET_JOB_GRANTS` action. TypeScript types are regenerated
from proto; no new reducer case required.

---

## Section 4: Testing

- REQ-FCM-TEST-1: Unit test — `handleJobGrants()` with an unresolved choice pool MUST produce a non-empty `pending_feat_choices` and empty `feat_id`/`feat_name` for that grant level.
- REQ-FCM-TEST-2: Unit test — `handleChooseFeat()` with a valid feat_id MUST persist the feat and return a success event.
- REQ-FCM-TEST-3: Unit test — `handleChooseFeat()` with a feat_id not in the pool MUST return a denial event and not modify state.
- REQ-FCM-TEST-4: Unit test — `handleChooseFeat()` for an already-owned feat MUST return a denial event and not modify state.
- REQ-FCM-TEST-5: React component test — `FeatChoiceModal` with count=1 MUST keep Confirm disabled until exactly one feat is selected.
- REQ-FCM-TEST-6: React component test — `FeatChoiceModal` on confirm MUST call `sendMessage('ChooseFeatRequest', ...)` for each selected feat.
