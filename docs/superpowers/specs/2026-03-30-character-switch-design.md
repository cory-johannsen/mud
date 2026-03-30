# Character Switch

**Date:** 2026-03-30
**Status:** spec

## Overview

Players can switch characters without fully logging out. The Logout button in the `GamePage` toolbar becomes a dropdown with two options: **Switch Character** and **Logout**. The `CharactersPage` Logout button remains a plain logout button.

## Architecture

```
LogoutDropdown (GamePage toolbar)
    │
    ├── "Switch Character" → navigate('/characters')
    │       GamePage unmounts → gRPC stream teardown (existing useEffect cleanup)
    │       Account JWT remains valid
    │       CharactersPage loads → user picks new character → play(id) → /game
    │
    └── "Logout" → auth.logout()
            Clears localStorage token
            Clears auth state
            Redirects to /login
```

**Key constraints:**
- No server-side disconnect call required — gRPC stream closes on `GamePage` unmount
- Account-level JWT is preserved across character switches; only character-scoped session token changes when `play(id)` is called
- `CharactersPage` Logout button is unmodified

## Requirements

### 1. LogoutDropdown Component

- REQ-CS-1: A new `LogoutDropdown` React component MUST be created at `src/components/LogoutDropdown.tsx`.
- REQ-CS-2: `LogoutDropdown` MUST render a single trigger button labeled `Logout ▾`.
- REQ-CS-3: Clicking the trigger button MUST toggle a dropdown menu open or closed.
- REQ-CS-4: The dropdown menu MUST contain exactly two items: **Switch Character** and **Logout**, in that order.
- REQ-CS-5: Clicking **Switch Character** MUST call `navigate('/characters')` and close the dropdown.
- REQ-CS-6: Clicking **Logout** MUST call `logout()` from `useAuth()` and close the dropdown.
- REQ-CS-7: Clicking anywhere outside the `LogoutDropdown` component MUST close the dropdown.
- REQ-CS-8: `LogoutDropdown` MUST use a `useEffect` with a `document` click listener (cleaned up on unmount) to implement outside-click dismissal.

### 2. GamePage Integration

- REQ-CS-9: `GamePage.tsx` MUST replace the existing plain `Logout` button with `<LogoutDropdown />`.
- REQ-CS-10: `GamePage.tsx` MUST NOT pass any props to `LogoutDropdown`; all dependencies (`useAuth`, `useNavigate`) MUST be sourced inside the component.

### 3. Styling

- REQ-CS-11: The `LogoutDropdown` trigger button MUST share the existing `.toolbar-btn` and `.toolbar-btn-logout` CSS classes so it is visually consistent with other toolbar buttons.
- REQ-CS-12: The dropdown panel MUST be absolutely positioned below the trigger button with `z-index` sufficient to appear above all other game UI elements.
- REQ-CS-13: The dropdown panel MUST use the toolbar's dark color scheme: background `#0d0d0d`, border `1px solid #333`, and a visible hover highlight on each item.
- REQ-CS-14: Each dropdown item MUST be a full-width clickable element with `cursor: pointer` and a hover background of `#1a1a1a`.

### 4. CharactersPage

- REQ-CS-15: The `CharactersPage` Logout button MUST remain unchanged — a plain button that calls `logout()` directly.

### 5. Testing

- REQ-CS-16: `LogoutDropdown` MUST have unit tests covering:
  - Dropdown is closed by default.
  - Clicking the trigger opens the dropdown.
  - Clicking **Switch Character** navigates to `/characters`.
  - Clicking **Logout** calls `logout()`.
  - Clicking outside the component closes the dropdown.

## Out of Scope

- Server-side explicit disconnect on character switch
- Keyboard navigation of the dropdown
- Animation on dropdown open/close
- Any changes to `CharactersPage`
