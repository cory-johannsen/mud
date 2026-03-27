# Game Client (Ebiten) Phase 4: Rendering

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Full Ebiten game screen with room scene, feed, character panel, input panel, and combat animations.

**Architecture:** Stateless pure-function renderers driven by GameState. Layout computed from window dimensions each frame. AnimationQueue manages per-sprite frame sequencing.

**Tech Stack:** Ebiten v2, Go image/color, text/v2

---

## Requirements Covered

- REQ-GCE-3: Initial 1280×800 window; resizable with proportional layout; minimum 800×600 enforced.
- REQ-GCE-20: Four screen regions — Scene (~70% height, full width), Feed (~25% height, ~60% width), Character (~25% height, ~40% width), Input (~5% height, full width).
- REQ-GCE-21: Zone background tile; player sprite anchored bottom-centre; max 6 NPC sprites evenly spaced; 7th+ shown as count badge on 6th slot.
- REQ-GCE-22: Exit indicators at scene edges (N=top-centre, S=bottom-centre, E=right-centre, W=left-centre) for each exit in `RoomView.exits`.
- REQ-GCE-23: Feed accumulates up to 500 `ServerEvent` messages, auto-scrolls to latest; oldest discarded when limit exceeded; colours from `colours.yaml`.
- REQ-GCE-24: Character panel: name, HP bar (green >50%, yellow >25%, red ≤25%), current/max HP, active conditions, hero points from `CharacterSheetView.hero_points`.
- REQ-GCE-25: `CombatEvent` ATTACK → attacker plays attack frames at tiles.yaml fps; target plays hit-flash frame.
- REQ-GCE-26: `CombatEvent` DEATH → target plays death frames; sprite removed only when subsequent `RoomView` excludes that NPC; concurrent events on same sprite are queued sequentially.

---

## Assumptions

- Phases 1–3 are complete: `cmd/ebitenclient/config`, `cmd/ebitenclient/auth`, and `cmd/ebitenclient/assets` packages exist and are importable.
- The asset registry (Phase 3) exposes: `Registry.Background(zoneName string) *ebiten.Image`, `Registry.NPCImage(category string) *ebiten.Image`, `Registry.PlayerImage() *ebiten.Image`, `Registry.AttackFrames(category string) []image.Rectangle`, `Registry.HitFlashFrame(category string) image.Rectangle`, `Registry.DeathFrames(category string) []image.Rectangle`, `Registry.FPS(category string) int`, and `Registry.FeedColour(eventType string) color.RGBA`.
- The `Screen` interface (Phase 2) is defined in `cmd/ebitenclient/screen.go` as:
  ```go
  type Screen interface {
      Update() error
      Draw(screen *ebiten.Image)
      Layout(outsideWidth, outsideHeight int) (int, int)
  }
  ```
- The `ScreenManager` (Phase 2) exposes `SetScreen(s Screen)` to transition between screens.
- Proto types are in package `gamev1 "github.com/cory-johannsen/mud/api/gen/game/v1"`.
- The gRPC session stream is passed into `GameScreen` as a parameter implementing:
  ```go
  type SessionStream interface {
      Send(*gamev1.ClientMessage) error
      Recv() (*gamev1.ServerEvent, error)
  }
  ```
- `internal/command/parse.go` exposes `Parse(input string) (*gamev1.ClientMessage, error)`.

---

## Files

| File | Action | Description |
|------|--------|-------------|
| `cmd/ebitenclient/game/state.go` | Create | `GameState` struct; feed cap; state-mutation methods |
| `cmd/ebitenclient/game/state_test.go` | Create | TDD: feed cap, state updates |
| `cmd/ebitenclient/game/layout.go` | Create | `Layout` struct; pixel-region computation from window size |
| `cmd/ebitenclient/game/layout_test.go` | Create | TDD: proportions, min-size enforcement |
| `cmd/ebitenclient/game/animation.go` | Create | `Animation`, `AnimationQueue`; frame advancement per Update tick |
| `cmd/ebitenclient/game/animation_test.go` | Create | TDD: queue ordering, frame advancement |
| `cmd/ebitenclient/game/scene.go` | Create | `DrawScene`: background, NPC sprites, player, exit indicators, count badge |
| `cmd/ebitenclient/game/feed.go` | Create | `DrawFeed`: colour-coded messages, scroll to latest |
| `cmd/ebitenclient/game/character_panel.go` | Create | `DrawCharacter`: HP bar colour coding, conditions, hero points |
| `cmd/ebitenclient/game/input_panel.go` | Create | `DrawInput`: text field, Send button |
| `cmd/ebitenclient/game/screen.go` | Create | `GameScreen` implementing `Screen`; integrates all panels; handles gRPC recv loop |
| `cmd/ebitenclient/main.go` | Modify | Add transition from character select into `GameScreen` |

---

## Tasks

### Task 1 — GameState: failing tests first (TDD)

Create `cmd/ebitenclient/game/state_test.go` **before** the implementation.

```go
package game_test

import (
	"testing"

	gamev1 "github.com/cory-johannsen/mud/api/gen/game/v1"
	"github.com/cory-johannsen/mud/cmd/ebitenclient/game"
	"pgregory.net/rapid"
)

func TestFeedCap_DiscardOldest(t *testing.T) {
	s := game.NewGameState()
	for i := 0; i < 510; i++ {
		s.AddFeedMessage(game.FeedMessage{Text: "msg", Colour: [4]uint8{255, 255, 255, 255}})
	}
	if got := s.FeedLen(); got != 500 {
		t.Fatalf("want 500 feed messages, got %d", got)
	}
}

func TestFeedCap_Property(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		n := rapid.IntRange(0, 1000).Draw(rt, "n")
		s := game.NewGameState()
		for i := 0; i < n; i++ {
			s.AddFeedMessage(game.FeedMessage{Text: "x", Colour: [4]uint8{}})
		}
		want := n
		if want > 500 {
			want = 500
		}
		if got := s.FeedLen(); got != want {
			rt.Fatalf("want %d, got %d", want, got)
		}
	})
}

func TestApplyRoomView(t *testing.T) {
	s := game.NewGameState()
	rv := &gamev1.RoomView{
		RoomId:      "r1",
		Title:       "The Alley",
		Description: "Dark.",
		ZoneName:    "wooklyn",
	}
	s.ApplyRoomView(rv)
	if s.RoomView == nil || s.RoomView.RoomId != "r1" {
		t.Fatal("RoomView not applied")
	}
}

func TestApplyCharacterInfo(t *testing.T) {
	s := game.NewGameState()
	ci := &gamev1.CharacterInfo{Name: "Zork", CurrentHp: 38, MaxHp: 50, Level: 5}
	s.ApplyCharacterInfo(ci)
	if s.CharacterInfo == nil || s.CharacterInfo.Name != "Zork" {
		t.Fatal("CharacterInfo not applied")
	}
}

func TestApplyHeroPoints(t *testing.T) {
	s := game.NewGameState()
	s.ApplyHeroPoints(3)
	if s.HeroPoints != 3 {
		t.Fatalf("want HeroPoints=3, got %d", s.HeroPoints)
	}
}
```

Now create `cmd/ebitenclient/game/state.go`:

```go
package game

import (
	"sync"

	gamev1 "github.com/cory-johannsen/mud/api/gen/game/v1"
)

const feedCap = 500

// FeedMessage is a single entry in the Feed panel.
type FeedMessage struct {
	Text   string
	Colour [4]uint8 // RGBA
}

// GameState holds all mutable state for the game screen.
// All methods are safe for concurrent use.
type GameState struct {
	mu             sync.Mutex
	RoomView       *gamev1.RoomView
	CharacterInfo  *gamev1.CharacterInfo
	HeroPoints     int32
	feed           []FeedMessage
	CombatEvents   []*gamev1.CombatEvent
}

func NewGameState() *GameState { return &GameState{} }

func (s *GameState) AddFeedMessage(m FeedMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.feed = append(s.feed, m)
	if len(s.feed) > feedCap {
		s.feed = s.feed[len(s.feed)-feedCap:]
	}
}

// FeedSnapshot returns a copy of the current feed slice.
func (s *GameState) FeedSnapshot() []FeedMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]FeedMessage, len(s.feed))
	copy(out, s.feed)
	return out
}

func (s *GameState) FeedLen() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.feed)
}

func (s *GameState) ApplyRoomView(rv *gamev1.RoomView) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RoomView = rv
}

func (s *GameState) ApplyCharacterInfo(ci *gamev1.CharacterInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CharacterInfo = ci
}

func (s *GameState) ApplyHeroPoints(hp int32) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.HeroPoints = hp
}

func (s *GameState) AppendCombatEvent(ev *gamev1.CombatEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.CombatEvents = append(s.CombatEvents, ev)
}

// Snapshot returns a consistent read of all fields (safe for the render goroutine).
type StateSnapshot struct {
	RoomView      *gamev1.RoomView
	CharacterInfo *gamev1.CharacterInfo
	HeroPoints    int32
	Feed          []FeedMessage
	CombatEvents  []*gamev1.CombatEvent
}

func (s *GameState) Snapshot() StateSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	feed := make([]FeedMessage, len(s.feed))
	copy(feed, s.feed)
	evts := make([]*gamev1.CombatEvent, len(s.CombatEvents))
	copy(evts, s.CombatEvents)
	return StateSnapshot{
		RoomView:      s.RoomView,
		CharacterInfo: s.CharacterInfo,
		HeroPoints:    s.HeroPoints,
		Feed:          feed,
		CombatEvents:  evts,
	}
}
```

**Acceptance:** `mise exec -- go test ./cmd/ebitenclient/game/... -run TestFeed -v` exits 0.

---

### Task 2 — Layout: failing tests first (TDD)

Create `cmd/ebitenclient/game/layout_test.go` **before** the implementation.

```go
package game_test

import (
	"testing"

	"github.com/cory-johannsen/mud/cmd/ebitenclient/game"
	"pgregory.net/rapid"
)

const tolerancePct = 0.05 // ±5%

func withinTolerance(got, want, total int) bool {
	diff := float64(got-want)
	if diff < 0 {
		diff = -diff
	}
	return diff/float64(total) <= tolerancePct
}

func TestLayout_NominalProportions(t *testing.T) {
	l := game.ComputeLayout(1280, 800)

	// Scene: full width, ~70% height
	if l.Scene.Dx() != 1280 {
		t.Errorf("scene width: want 1280 got %d", l.Scene.Dx())
	}
	if !withinTolerance(l.Scene.Dy(), 560, 800) {
		t.Errorf("scene height: want ~560 got %d", l.Scene.Dy())
	}

	// Feed: ~60% width
	if !withinTolerance(l.Feed.Dx(), 768, 1280) {
		t.Errorf("feed width: want ~768 got %d", l.Feed.Dx())
	}

	// Character: ~40% width, same row as Feed
	if !withinTolerance(l.Character.Dx(), 512, 1280) {
		t.Errorf("character width: want ~512 got %d", l.Character.Dx())
	}

	// Input: full width, ~5% height
	if l.Input.Dx() != 1280 {
		t.Errorf("input width: want 1280 got %d", l.Input.Dx())
	}
	if !withinTolerance(l.Input.Dy(), 40, 800) {
		t.Errorf("input height: want ~40 got %d", l.Input.Dy())
	}
}

func TestLayout_MinWindowEnforcement(t *testing.T) {
	// Windows smaller than 800×600 must be clamped to 800×600.
	l := game.ComputeLayout(400, 300)
	if l.WindowW < 800 || l.WindowH < 600 {
		t.Errorf("min size not enforced: got %dx%d", l.WindowW, l.WindowH)
	}
}

func TestLayout_TotalHeightCoverage(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		w := rapid.IntRange(800, 3840).Draw(rt, "w")
		h := rapid.IntRange(600, 2160).Draw(rt, "h")
		l := game.ComputeLayout(w, h)
		total := l.Scene.Dy() + l.Feed.Dy() + l.Input.Dy()
		if total != l.WindowH {
			rt.Fatalf("height regions do not sum to window height: %d+%d+%d=%d != %d",
				l.Scene.Dy(), l.Feed.Dy(), l.Input.Dy(), total, l.WindowH)
		}
	})
}

func TestLayout_FeedAndCharacterAdjacent(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		w := rapid.IntRange(800, 3840).Draw(rt, "w")
		h := rapid.IntRange(600, 2160).Draw(rt, "h")
		l := game.ComputeLayout(w, h)
		// Feed right edge must equal Character left edge
		if l.Feed.Max.X != l.Character.Min.X {
			rt.Fatalf("feed/char gap: feed.Max.X=%d char.Min.X=%d", l.Feed.Max.X, l.Character.Min.X)
		}
		// Total width of feed+character must equal window width
		if l.Feed.Dx()+l.Character.Dx() != l.WindowW {
			rt.Fatalf("feed+char width %d != window %d", l.Feed.Dx()+l.Character.Dx(), l.WindowW)
		}
	})
}
```

Now create `cmd/ebitenclient/game/layout.go`:

```go
package game

import "image"

// Layout holds pixel-coordinate rectangles for all four screen regions.
type Layout struct {
	WindowW, WindowH int
	Scene            image.Rectangle
	Feed             image.Rectangle
	Character        image.Rectangle
	Input            image.Rectangle
}

const (
	minWindowW = 800
	minWindowH = 600

	sceneHeightPct     = 0.70
	feedWidthPct       = 0.60
	inputHeightPct     = 0.05
	// middlePanelHeight = 1.0 - sceneHeightPct - inputHeightPct = 0.25
)

// ComputeLayout computes pixel regions for the given window dimensions.
// Window sizes below 800×600 are clamped to the minimum.
func ComputeLayout(w, h int) Layout {
	if w < minWindowW {
		w = minWindowW
	}
	if h < minWindowH {
		h = minWindowH
	}

	inputH := int(float64(h) * inputHeightPct)
	sceneH := int(float64(h) * sceneHeightPct)
	middleH := h - sceneH - inputH // absorbs rounding

	feedW := int(float64(w) * feedWidthPct)
	charW := w - feedW

	scene := image.Rect(0, 0, w, sceneH)
	feed := image.Rect(0, sceneH, feedW, sceneH+middleH)
	char := image.Rect(feedW, sceneH, w, sceneH+middleH)
	input := image.Rect(0, sceneH+middleH, w, h)

	return Layout{
		WindowW:   w,
		WindowH:   h,
		Scene:     scene,
		Feed:      feed,
		Character: char,
		Input:     input,
	}
}
```

**Acceptance:** `mise exec -- go test ./cmd/ebitenclient/game/... -run TestLayout -v` exits 0.

---

### Task 3 — AnimationQueue: failing tests first (TDD)

Create `cmd/ebitenclient/game/animation_test.go` **before** the implementation.

```go
package game_test

import (
	"image"
	"testing"

	"github.com/cory-johannsen/mud/cmd/ebitenclient/game"
	"pgregory.net/rapid"
)

func makeFrames(n int) []image.Rectangle {
	frames := make([]image.Rectangle, n)
	for i := range frames {
		frames[i] = image.Rect(i*16, 0, (i+1)*16, 16)
	}
	return frames
}

func TestAnimationQueue_SequentialPlay(t *testing.T) {
	q := game.NewAnimationQueue()
	q.Enqueue("npc1", game.Animation{SpriteID: "npc1", Frames: makeFrames(3), FPS: 12})
	q.Enqueue("npc1", game.Animation{SpriteID: "npc1", Frames: makeFrames(2), FPS: 12})

	// First animation must play before second
	first := q.Current("npc1")
	if first == nil {
		t.Fatal("expected animation, got nil")
	}
	if len(first.Frames) != 3 {
		t.Fatalf("want first anim 3 frames, got %d", len(first.Frames))
	}
}

func TestAnimationQueue_FrameAdvance(t *testing.T) {
	q := game.NewAnimationQueue()
	q.Enqueue("npc1", game.Animation{SpriteID: "npc1", Frames: makeFrames(4), FPS: 60})

	// At 60fps, one tick advances one frame.
	// Tick enough times to cycle all 4 frames.
	for i := 0; i < 4; i++ {
		q.Tick(1) // 1 tick at 60fps = 1 frame
	}
	if cur := q.Current("npc1"); cur != nil {
		// After all frames, first animation should be done and dequeued.
		t.Fatalf("expected animation done and dequeued, still have %v", cur)
	}
}

func TestAnimationQueue_DoneTransitionsToNext(t *testing.T) {
	q := game.NewAnimationQueue()
	q.Enqueue("npc1", game.Animation{SpriteID: "npc1", Frames: makeFrames(2), FPS: 60})
	q.Enqueue("npc1", game.Animation{SpriteID: "npc1", Frames: makeFrames(3), FPS: 60})

	// Advance through first animation (2 frames at 60fps = 2 ticks)
	q.Tick(2)

	second := q.Current("npc1")
	if second == nil {
		t.Fatal("expected second animation after first completes")
	}
	if len(second.Frames) != 3 {
		t.Fatalf("want second anim 3 frames, got %d", len(second.Frames))
	}
}

func TestAnimationQueue_CurrentFrame(t *testing.T) {
	frames := makeFrames(4)
	q := game.NewAnimationQueue()
	q.Enqueue("s", game.Animation{SpriteID: "s", Frames: frames, FPS: 60})

	q.Tick(2)
	fr := q.CurrentFrame("s")
	if fr != frames[2] {
		t.Fatalf("want frame[2]=%v got %v", frames[2], fr)
	}
}

func TestAnimationQueue_Property_NoFrameOutOfBounds(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		nFrames := rapid.IntRange(1, 20).Draw(rt, "nFrames")
		nTicks := rapid.IntRange(0, 100).Draw(rt, "nTicks")
		q := game.NewAnimationQueue()
		q.Enqueue("s", game.Animation{SpriteID: "s", Frames: makeFrames(nFrames), FPS: 60})
		q.Tick(nTicks)
		// Should never panic — accessing CurrentFrame is safe whether or not the animation is done.
		_ = q.CurrentFrame("s")
	})
}
```

Now create `cmd/ebitenclient/game/animation.go`:

```go
package game

import "image"

// Animation describes a single animation clip for one sprite.
type Animation struct {
	SpriteID string
	Frames   []image.Rectangle // sub-rects within the sprite sheet
	FPS      int               // frames per second; default 12 if zero
}

type animEntry struct {
	anim       Animation
	frameIndex int
	tickAccum  int
}

func (e *animEntry) fps() int {
	if e.anim.FPS <= 0 {
		return 12
	}
	return e.anim.FPS
}

// ticksPerFrame returns how many Ebiten ticks (at 60 TPS) equal one animation frame.
func (e *animEntry) ticksPerFrame() int {
	tpf := 60 / e.fps()
	if tpf < 1 {
		tpf = 1
	}
	return tpf
}

// advance advances by delta ticks; returns true when the animation has completed.
func (e *animEntry) advance(delta int) bool {
	e.tickAccum += delta
	tpf := e.ticksPerFrame()
	for e.tickAccum >= tpf {
		e.tickAccum -= tpf
		e.frameIndex++
		if e.frameIndex >= len(e.anim.Frames) {
			return true
		}
	}
	return false
}

// AnimationQueue manages per-sprite queued animations.
// Concurrent events on the same sprite play sequentially.
// Safe for single-goroutine use (Ebiten Update runs on one goroutine).
type AnimationQueue struct {
	queues map[string][]*animEntry
}

func NewAnimationQueue() *AnimationQueue {
	return &AnimationQueue{queues: make(map[string][]*animEntry)}
}

// Enqueue adds an animation to the tail of the queue for spriteID.
func (aq *AnimationQueue) Enqueue(spriteID string, anim Animation) {
	aq.queues[spriteID] = append(aq.queues[spriteID], &animEntry{anim: anim})
}

// Tick advances all active animations by delta Ebiten ticks.
// Completed animations are dequeued; the next queued animation becomes active.
func (aq *AnimationQueue) Tick(delta int) {
	for id, queue := range aq.queues {
		if len(queue) == 0 {
			continue
		}
		if done := queue[0].advance(delta); done {
			aq.queues[id] = queue[1:]
		}
	}
}

// Current returns the active Animation for spriteID, or nil if the queue is empty.
func (aq *AnimationQueue) Current(spriteID string) *Animation {
	queue := aq.queues[spriteID]
	if len(queue) == 0 {
		return nil
	}
	return &queue[0].anim
}

// CurrentFrame returns the current image.Rectangle for spriteID.
// Returns image.ZR if no animation is active.
func (aq *AnimationQueue) CurrentFrame(spriteID string) image.Rectangle {
	queue := aq.queues[spriteID]
	if len(queue) == 0 {
		return image.ZR
	}
	e := queue[0]
	idx := e.frameIndex
	if idx >= len(e.anim.Frames) {
		idx = len(e.anim.Frames) - 1
	}
	return e.anim.Frames[idx]
}

// Remove deletes all queued animations for spriteID (used when NPC leaves the room).
func (aq *AnimationQueue) Remove(spriteID string) {
	delete(aq.queues, spriteID)
}
```

**Acceptance:** `mise exec -- go test ./cmd/ebitenclient/game/... -run TestAnimation -v` exits 0.

---

### Task 4 — Scene panel draw

Create `cmd/ebitenclient/game/scene.go`. This is a draw function; verification is manual (run the binary, observe the scene).

```go
package game

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	gamev1 "github.com/cory-johannsen/mud/api/gen/game/v1"
	"github.com/cory-johannsen/mud/cmd/ebitenclient/assets"
)

const maxNPCSprites = 6

// DrawScene renders the scene panel onto dst within the bounds of region.
func DrawScene(
	dst *ebiten.Image,
	region image.Rectangle,
	rv *gamev1.RoomView,
	reg *assets.Registry,
	anims *AnimationQueue,
) {
	sub := dst.SubImage(region).(*ebiten.Image)

	// Background tile
	if rv != nil {
		bg := reg.Background(rv.ZoneName)
		if bg != nil {
			op := &ebiten.DrawImageOptions{}
			scaleX := float64(region.Dx()) / float64(bg.Bounds().Dx())
			scaleY := float64(region.Dy()) / float64(bg.Bounds().Dy())
			op.GeoM.Scale(scaleX, scaleY)
			op.GeoM.Translate(float64(region.Min.X), float64(region.Min.Y))
			dst.DrawImage(bg, op)
		}
	}

	// NPC sprites
	if rv != nil {
		npcs := rv.Npcs
		shown := npcs
		extra := 0
		if len(npcs) > maxNPCSprites {
			shown = npcs[:maxNPCSprites]
			extra = len(npcs) - maxNPCSprites
		}

		slotW := region.Dx() / maxNPCSprites
		spriteH := region.Dy() / 3 // sprites occupy upper third of scene

		for i, npc := range shown {
			x := region.Min.X + i*slotW + slotW/4
			y := region.Min.Y + spriteH/4

			isLast := i == maxNPCSprites-1 && extra > 0

			img := reg.NPCImage(npc.Name)
			if img != nil && !isLast {
				frame := anims.CurrentFrame(npc.Name)
				var src *ebiten.Image
				if frame != image.ZR {
					src = img.SubImage(frame).(*ebiten.Image)
				} else {
					src = img
				}
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(float64(x), float64(y))
				dst.DrawImage(src, op)
			} else if isLast {
				// Draw count badge "+N more"
				label := fmt.Sprintf("+%d more", extra+1)
				ebitenutil.DebugPrintAt(sub, label, x-region.Min.X, y-region.Min.Y)
			}
		}
	}

	// Player sprite — bottom-centre of scene
	player := reg.PlayerImage()
	if player != nil {
		pw := player.Bounds().Dx()
		ph := player.Bounds().Dy()
		px := region.Min.X + region.Dx()/2 - pw/2
		py := region.Max.Y - ph
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(px), float64(py))
		dst.DrawImage(player, op)
	}

	// Exit indicators
	if rv != nil {
		drawExitIndicators(dst, region, rv.Exits)
	}
}

// exitIndicator is a labelled rectangle drawn at a scene edge.
type exitIndicator struct {
	label string
	rect  image.Rectangle
}

func drawExitIndicators(dst *ebiten.Image, region image.Rectangle, exits []*gamev1.ExitInfo) {
	const indW, indH = 40, 20
	for _, ex := range exits {
		var r image.Rectangle
		cx := region.Min.X + region.Dx()/2
		cy := region.Min.Y + region.Dy()/2
		switch ex.Direction {
		case "north", "n":
			r = image.Rect(cx-indW/2, region.Min.Y, cx+indW/2, region.Min.Y+indH)
		case "south", "s":
			r = image.Rect(cx-indW/2, region.Max.Y-indH, cx+indW/2, region.Max.Y)
		case "east", "e":
			r = image.Rect(region.Max.X-indW, cy-indH/2, region.Max.X, cy+indH/2)
		case "west", "w":
			r = image.Rect(region.Min.X, cy-indH/2, region.Min.X+indW, cy+indH/2)
		default:
			continue
		}
		drawFilledRect(dst, r, color.RGBA{R: 80, G: 80, B: 80, A: 200})
		ebitenutil.DebugPrintAt(dst, ex.Direction, r.Min.X+4, r.Min.Y+4)
	}
}

func drawFilledRect(dst *ebiten.Image, r image.Rectangle, c color.Color) {
	img := ebiten.NewImage(r.Dx(), r.Dy())
	img.Fill(c)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(r.Min.X), float64(r.Min.Y))
	dst.DrawImage(img, op)
}
```

**Manual verification:** Run binary, join world, confirm background scales to fill scene, up to 6 NPC boxes appear evenly spaced, "+N more" appears when more than 6 NPCs are present, exit indicators appear at correct edges.

---

### Task 5 — Feed panel draw

Create `cmd/ebitenclient/game/feed.go`:

```go
package game

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const feedLineHeight = 14

// DrawFeed renders the feed panel onto dst within region.
// messages MUST be ordered oldest-first; the function auto-scrolls to the latest.
func DrawFeed(dst *ebiten.Image, region image.Rectangle, messages []FeedMessage) {
	// Background
	drawFilledRect(dst, region, color.RGBA{R: 10, G: 10, B: 10, A: 230})

	maxLines := region.Dy() / feedLineHeight
	start := 0
	if len(messages) > maxLines {
		start = len(messages) - maxLines
	}

	for i, msg := range messages[start:] {
		y := region.Min.Y + i*feedLineHeight
		c := color.RGBA{R: msg.Colour[0], G: msg.Colour[1], B: msg.Colour[2], A: msg.Colour[3]}
		_ = c // ebitenutil.DebugPrintAt does not support per-call colour in the basic API;
		      // use the colour for future text/v2 upgrade
		ebitenutil.DebugPrintAt(dst, msg.Text, region.Min.X+4, y)
	}
}
```

**Note:** `ebitenutil.DebugPrintAt` does not support per-call colour; colour-coded rendering MUST be upgraded to `text/v2` in a follow-up task once the asset pack's font is integrated. The `Colour` field is stored and passed through now so the upgrade requires only the render call, not the data model.

**Manual verification:** Join world, generate chat events, confirm messages appear in feed panel, latest message visible at bottom.

---

### Task 6 — Character panel draw

Create `cmd/ebitenclient/game/character_panel.go`:

```go
package game

import (
	"fmt"
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	gamev1 "github.com/cory-johannsen/mud/api/gen/game/v1"
)

var (
	hpGreen  = color.RGBA{R: 0x22, G: 0xc5, B: 0x5e, A: 0xff} // >50% HP  (#22c55e)
	hpYellow = color.RGBA{R: 0xea, G: 0xb3, B: 0x08, A: 0xff} // >25% HP  (#eab308)
	hpRed    = color.RGBA{R: 0xef, G: 0x44, B: 0x44, A: 0xff} // ≤25% HP  (#ef4444)
)

// hpColour returns the HP bar fill colour for the given current/max HP values.
func hpColour(current, max int32) color.RGBA {
	if max <= 0 {
		return hpGreen
	}
	pct := float64(current) / float64(max)
	switch {
	case pct > 0.50:
		return hpGreen
	case pct > 0.25:
		return hpYellow
	default:
		return hpRed
	}
}

// DrawCharacter renders the character panel onto dst within region.
func DrawCharacter(
	dst *ebiten.Image,
	region image.Rectangle,
	ci *gamev1.CharacterInfo,
	rv *gamev1.RoomView,
	heroPoints int32,
) {
	drawFilledRect(dst, region, color.RGBA{R: 15, G: 15, B: 20, A: 230})

	if ci == nil {
		ebitenutil.DebugPrintAt(dst, "No character data", region.Min.X+4, region.Min.Y+4)
		return
	}

	lineY := region.Min.Y + 4

	// Name, level, class
	header := fmt.Sprintf("%s  Lv%d  %s", ci.Name, ci.Level, ci.Class)
	ebitenutil.DebugPrintAt(dst, header, region.Min.X+4, lineY)
	lineY += feedLineHeight + 2

	// HP bar
	barMaxW := region.Dx() - 8
	barH := 10
	barFillW := 0
	if ci.MaxHp > 0 {
		barFillW = int(float64(barMaxW) * float64(ci.CurrentHp) / float64(ci.MaxHp))
		if barFillW > barMaxW {
			barFillW = barMaxW
		}
	}
	barBg := image.Rect(region.Min.X+4, lineY, region.Min.X+4+barMaxW, lineY+barH)
	barFill := image.Rect(region.Min.X+4, lineY, region.Min.X+4+barFillW, lineY+barH)
	drawFilledRect(dst, barBg, color.RGBA{R: 40, G: 40, B: 40, A: 255})
	drawFilledRect(dst, barFill, hpColour(ci.CurrentHp, ci.MaxHp))
	lineY += barH + 2

	// Current/max HP text
	hpText := fmt.Sprintf("%d / %d HP", ci.CurrentHp, ci.MaxHp)
	ebitenutil.DebugPrintAt(dst, hpText, region.Min.X+4, lineY)
	lineY += feedLineHeight + 2

	// Active conditions
	if rv != nil {
		for _, cond := range rv.ActiveConditions {
			ebitenutil.DebugPrintAt(dst, "• "+cond.Name, region.Min.X+4, lineY)
			lineY += feedLineHeight
		}
	}

	// Hero points
	heroText := fmt.Sprintf("Hero: %d", heroPoints)
	ebitenutil.DebugPrintAt(dst, heroText, region.Min.X+4, lineY)
}
```

**Manual verification:** Join world with a character, verify name/level/class header, HP bar fills proportionally with correct colour, conditions list, hero points display.

---

### Task 7 — Input panel draw

Create `cmd/ebitenclient/game/input_panel.go`:

```go
package game

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

const sendButtonW = 60

// InputPanelState holds the mutable state of the input panel.
type InputPanelState struct {
	Text   string
	Cursor int // byte offset of cursor within Text
}

// SendButtonRect returns the pixel rectangle for the Send button within region.
func SendButtonRect(region image.Rectangle) image.Rectangle {
	return image.Rect(
		region.Max.X-sendButtonW-4,
		region.Min.Y+2,
		region.Max.X-4,
		region.Max.Y-2,
	)
}

// DrawInput renders the input panel onto dst within region.
func DrawInput(dst *ebiten.Image, region image.Rectangle, state InputPanelState) {
	// Background
	drawFilledRect(dst, region, color.RGBA{R: 5, G: 5, B: 5, A: 240})

	// Prompt character
	ebitenutil.DebugPrintAt(dst, ">", region.Min.X+4, region.Min.Y+4)

	// Text field area
	fieldRect := image.Rect(region.Min.X+16, region.Min.Y+2, region.Max.X-sendButtonW-8, region.Max.Y-2)
	drawFilledRect(dst, fieldRect, color.RGBA{R: 20, G: 20, B: 20, A: 255})

	displayText := state.Text
	if displayText == "" {
		displayText = "Type a command..."
	}
	ebitenutil.DebugPrintAt(dst, displayText, fieldRect.Min.X+4, fieldRect.Min.Y+4)

	// Send button
	sendRect := SendButtonRect(region)
	drawFilledRect(dst, sendRect, color.RGBA{R: 50, G: 100, B: 180, A: 255})
	ebitenutil.DebugPrintAt(dst, "Send", sendRect.Min.X+8, sendRect.Min.Y+4)
}
```

**Manual verification:** Input field visible with placeholder text; typing appends characters; Send button visible at right edge.

---

### Task 8 — GameScreen: integrate all panels, handle gRPC recv loop

Create `cmd/ebitenclient/game/screen.go`:

```go
package game

import (
	"context"
	"fmt"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	gamev1 "github.com/cory-johannsen/mud/api/gen/game/v1"
	"github.com/cory-johannsen/mud/cmd/ebitenclient/assets"
	"github.com/cory-johannsen/mud/internal/command"
	"go.uber.org/zap"
)

// SessionStream is the bidirectional gRPC stream for an active game session.
type SessionStream interface {
	Send(*gamev1.ClientMessage) error
	Recv() (*gamev1.ServerEvent, error)
}

// ScreenManager allows the GameScreen to trigger screen transitions.
type ScreenManager interface {
	SetScreen(Screen)
}

// Screen is the interface all screens must implement (defined in Phase 2).
type Screen interface {
	Update() error
	Draw(screen *ebiten.Image)
	Layout(outsideWidth, outsideHeight int) (int, int)
}

// GameScreen is the main game screen, implementing Screen.
type GameScreen struct {
	state    *GameState
	anims    *AnimationQueue
	registry *assets.Registry
	stream   SessionStream
	manager  ScreenManager
	logger   *zap.Logger

	input     InputPanelState
	layout    Layout
	cancelRecv context.CancelFunc
}

// NewGameScreen constructs and starts the recv goroutine.
func NewGameScreen(
	stream SessionStream,
	registry *assets.Registry,
	manager ScreenManager,
	charName string,
	logger *zap.Logger,
) *GameScreen {
	gs := &GameScreen{
		state:    NewGameState(),
		anims:    NewAnimationQueue(),
		registry: registry,
		stream:   stream,
		manager:  manager,
		logger:   logger,
	}
	gs.layout = ComputeLayout(1280, 800)
	go gs.recvLoop()
	return gs
}

// recvLoop runs in a background goroutine, reading ServerEvents from the stream
// and applying them to GameState.
func (gs *GameScreen) recvLoop() {
	for {
		ev, err := gs.stream.Recv()
		if err != nil {
			gs.logger.Error("stream recv error", zap.Error(err))
			return
		}
		gs.handleServerEvent(ev)
	}
}

func (gs *GameScreen) handleServerEvent(ev *gamev1.ServerEvent) {
	switch v := ev.Event.(type) {
	case *gamev1.ServerEvent_RoomView:
		gs.state.ApplyRoomView(v.RoomView)
		// Remove sprites for NPCs no longer in the room.
		if v.RoomView != nil {
			present := make(map[string]bool)
			for _, npc := range v.RoomView.Npcs {
				present[npc.Name] = true
			}
			// AnimationQueue Remove is called for departed NPCs.
			// We track prior NPC list via a snapshot.
			snap := gs.state.Snapshot()
			if snap.RoomView != nil {
				for _, npc := range snap.RoomView.Npcs {
					if !present[npc.Name] {
						gs.anims.Remove(npc.Name)
					}
				}
			}
		}
	case *gamev1.ServerEvent_CharacterInfo:
		gs.state.ApplyCharacterInfo(v.CharacterInfo)
	case *gamev1.ServerEvent_CharacterSheet:
		if v.CharacterSheet != nil {
			gs.state.ApplyHeroPoints(v.CharacterSheet.HeroPoints)
		}
	case *gamev1.ServerEvent_CombatEvent:
		ce := v.CombatEvent
		gs.state.AppendCombatEvent(ce)
		gs.enqueueCombatAnimation(ce)
	default:
		// All other event types are appended to the feed.
		gs.state.AddFeedMessage(FeedMessage{
			Text:   ev.String(),
			Colour: gs.registry.FeedColour(fmt.Sprintf("%T", v)),
		})
	}
}

func (gs *GameScreen) enqueueCombatAnimation(ce *gamev1.CombatEvent) {
	switch ce.Type {
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_ATTACK:
		attackFrames := gs.registry.AttackFrames(ce.Attacker)
		if len(attackFrames) > 0 {
			gs.anims.Enqueue(ce.Attacker, Animation{
				SpriteID: ce.Attacker,
				Frames:   attackFrames,
				FPS:      gs.registry.FPS(ce.Attacker),
			})
		}
		hitFrame := gs.registry.HitFlashFrame(ce.Target)
		if hitFrame.Dx() > 0 {
			gs.anims.Enqueue(ce.Target, Animation{
				SpriteID: ce.Target,
				Frames:   []image.Rectangle{hitFrame},
				FPS:      12,
			})
		}
	case gamev1.CombatEventType_COMBAT_EVENT_TYPE_DEATH:
		deathFrames := gs.registry.DeathFrames(ce.Target)
		if len(deathFrames) > 0 {
			gs.anims.Enqueue(ce.Target, Animation{
				SpriteID: ce.Target,
				Frames:   deathFrames,
				FPS:      gs.registry.FPS(ce.Target),
			})
		}
	}
}

// Update is called every Ebiten tick (60 TPS).
func (gs *GameScreen) Update() error {
	gs.anims.Tick(1)
	gs.handleKeyboardInput()
	gs.handleMouseInput()
	return nil
}

func (gs *GameScreen) handleKeyboardInput() {
	// Printable keys → append to input field.
	for _, r := range ebiten.AppendInputChars(nil) {
		gs.input.Text += string(r)
	}
	// Enter → submit
	if isKeyJustPressed(ebiten.KeyEnter) || isKeyJustPressed(ebiten.KeyNumpadEnter) {
		gs.submitInput()
	}
	// Escape → clear
	if isKeyJustPressed(ebiten.KeyEscape) {
		gs.input.Text = ""
	}
	// Backspace → delete last char
	if isKeyJustPressed(ebiten.KeyBackspace) && len(gs.input.Text) > 0 {
		gs.input.Text = gs.input.Text[:len(gs.input.Text)-1]
	}
}

func isKeyJustPressed(k ebiten.Key) bool {
	return ebiten.IsKeyPressed(k)
}

func (gs *GameScreen) handleMouseInput() {
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		return
	}
	mx, my := ebiten.CursorPosition()
	clickPt := image.Point{X: mx, Y: my}

	// Send button click
	if clickPt.In(SendButtonRect(gs.layout.Input)) {
		gs.submitInput()
		return
	}

	// Exit indicator clicks
	snap := gs.state.Snapshot()
	if snap.RoomView != nil {
		for _, ex := range snap.RoomView.Exits {
			// Reconstruct the exit indicator rect to hit-test
			indR := exitIndicatorRect(gs.layout.Scene, ex.Direction)
			if clickPt.In(indR) {
				gs.input.Text = "move " + ex.Direction
				gs.submitInput()
				return
			}
		}
	}
}

func (gs *GameScreen) submitInput() {
	text := gs.input.Text
	gs.input.Text = ""
	if text == "" {
		return
	}
	msg, err := command.Parse(text)
	if err != nil {
		gs.state.AddFeedMessage(FeedMessage{
			Text:   "Unknown command: " + text,
			Colour: [4]uint8{255, 80, 80, 255},
		})
		return
	}
	if err := gs.stream.Send(msg); err != nil {
		gs.logger.Error("stream send error", zap.Error(err))
	}
}

// Draw renders all panels.
func (gs *GameScreen) Draw(screen *ebiten.Image) {
	screen.Fill(color.Black)
	snap := gs.state.Snapshot()

	DrawScene(screen, gs.layout.Scene, snap.RoomView, gs.registry, gs.anims)
	DrawFeed(screen, gs.layout.Feed, snap.Feed)
	DrawCharacter(screen, gs.layout.Character, snap.CharacterInfo, snap.RoomView, snap.HeroPoints)
	DrawInput(screen, gs.layout.Input, gs.input)
}

// Layout implements Screen; recomputes Layout on resize.
func (gs *GameScreen) Layout(outsideWidth, outsideHeight int) (int, int) {
	gs.layout = ComputeLayout(outsideWidth, outsideHeight)
	return gs.layout.WindowW, gs.layout.WindowH
}
```

**Note on `exitIndicatorRect`:** Add a package-level helper in `scene.go` (alongside `drawExitIndicators`) that returns the `image.Rectangle` for a given direction without drawing, so `screen.go` can hit-test without duplicating coordinate logic:

```go
// exitIndicatorRect returns the bounding rectangle for an exit indicator in region.
func exitIndicatorRect(region image.Rectangle, direction string) image.Rectangle {
	const indW, indH = 40, 20
	cx := region.Min.X + region.Dx()/2
	cy := region.Min.Y + region.Dy()/2
	switch direction {
	case "north", "n":
		return image.Rect(cx-indW/2, region.Min.Y, cx+indW/2, region.Min.Y+indH)
	case "south", "s":
		return image.Rect(cx-indW/2, region.Max.Y-indH, cx+indW/2, region.Max.Y)
	case "east", "e":
		return image.Rect(region.Max.X-indW, cy-indH/2, region.Max.X, cy+indH/2)
	case "west", "w":
		return image.Rect(region.Min.X, cy-indH/2, region.Min.X+indW, cy+indH/2)
	default:
		return image.ZR
	}
}
```

**Manual verification:** Full game screen renders; all four panels visible; input text echoes; Enter submits and clears field; exit indicators clickable.

---

### Task 9 — Wire GameScreen into main.go

Modify `cmd/ebitenclient/main.go` to add the `CharacterSelected` callback that creates and activates a `GameScreen`:

- The character select screen (Phase 3) calls a provided `OnSelect func(charName string, stream SessionStream)` callback when the user picks a character.
- In `main.go`, construct this callback to:
  1. Call `ebiten.SetWindowTitle(fmt.Sprintf("Mud — %s", charName))`.
  2. Construct `NewGameScreen(stream, registry, manager, charName, logger)`.
  3. Call `manager.SetScreen(gameScreen)`.

Exact changes:

```go
// In main.go, after constructing charSelectScreen:
charSelectScreen.OnSelect = func(charName string, stream game.SessionStream) {
    ebiten.SetWindowTitle(fmt.Sprintf("Mud — %s", charName))
    gs := game.NewGameScreen(stream, registry, manager, charName, logger)
    manager.SetScreen(gs)
}
```

**Acceptance:** `mise exec -- go build ./cmd/ebitenclient/...` exits 0. Binary launches, logs in, selects character, and window title changes to `"Mud — {name}"`.

---

## Run all tests

```
mise exec -- go test ./cmd/ebitenclient/game/... -v
```

All tests MUST pass with 0 failures before this phase is considered complete.

---

## Acceptance Summary

| Requirement | Acceptance Criterion |
|-------------|----------------------|
| REQ-GCE-3 | `TestLayout_MinWindowEnforcement` passes; binary enforces 800×600 minimum |
| REQ-GCE-20 | `TestLayout_NominalProportions` and `TestLayout_TotalHeightCoverage` pass |
| REQ-GCE-21 | Manual: scene renders background, up to 6 NPCs, player, count badge for extras |
| REQ-GCE-22 | Manual: exit indicators appear at correct scene edges per `RoomView.exits` |
| REQ-GCE-23 | `TestFeedCap_DiscardOldest` and `TestFeedCap_Property` pass; manual: feed scrolls to latest |
| REQ-GCE-24 | Manual: HP bar colour-coded; conditions and hero points visible |
| REQ-GCE-25 | `TestAnimationQueue_*` pass; manual: attack animation plays on attacker |
| REQ-GCE-26 | Manual: death animation plays; sprite persists until next `RoomView` removes it |
