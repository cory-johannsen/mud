# Login Splash Image

**Date:** 2026-03-30
**Status:** spec

## Overview

Replace the hardcoded ASCII art on the login page with `gunchete.png` as a full-viewport background image. The login/register form is presented as a semi-transparent dark card centered over the image. The image is optimized to WebP before deployment.

## Requirements

### 1. Image Optimization

- REQ-LSI-1: `gunchete.png` (6.2MB) MUST be converted to WebP format using `cwebp` at quality 80, resized to a maximum width of 1920px, and saved as `cmd/webclient/ui/public/gunchete.webp`.
- REQ-LSI-2: The original `gunchete.png` at the repository root MUST NOT be moved or deleted — it is the source asset.
- REQ-LSI-3: `cmd/webclient/ui/public/` MUST be created if it does not exist. Vite serves files in `public/` as static assets at the root URL path with no hashing.

### 2. LoginPage.tsx

- REQ-LSI-4: The three ASCII art constants (`AK47`, `GUNCHETE_TITLE`, `MACHETE`) and their rendered `<pre>` elements MUST be removed from `LoginPage.tsx`.
- REQ-LSI-5: The outermost container MUST be styled as a full-viewport background:
  - `width: 100vw`, `height: 100vh`
  - `backgroundImage: "url('/gunchete.webp')"`, `backgroundSize: "cover"`, `backgroundPosition: "center"`
  - `display: "flex"`, `alignItems: "center"`, `justifyContent: "center"`
  - `position: "relative"`
- REQ-LSI-6: A darkening overlay `<div>` MUST be inserted as a direct child of the outer container:
  - `position: "absolute"`, `inset: 0`
  - `background: "rgba(0,0,0,0.55)"`
  - `zIndex: 0`
- REQ-LSI-7: The login card `<div>` MUST be positioned above the overlay:
  - `position: "relative"`, `zIndex: 1`
  - `background: "rgba(13,13,13,0.85)"`
  - All existing gold border, padding, monospace font, and form content MUST be retained unchanged.

### 3. No Other Changes

- REQ-LSI-8: No other pages, components, or routes MUST be modified.
- REQ-LSI-9: The `public/` directory MUST be added to `.gitignore` exclusions only if `dist/` is already excluded — `public/gunchete.webp` MUST be committed to the repository as a build artifact.

## Out of Scope

- Image optimization tooling added to Makefile (manual one-time conversion)
- Responsive image srcset or multiple resolutions
- Animation or parallax effects
