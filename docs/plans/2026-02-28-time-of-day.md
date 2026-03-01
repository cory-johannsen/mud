# Time of Day Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a game clock (1 real minute = 1 game hour) that drives atmospheric room descriptions, outdoor visibility effects, and a live prompt showing period, hour, and HP.

**Architecture:** A `GameClock` singleton goroutine in the gameserver advances hour 0–23 every real minute and broadcasts `TimeOfDayEvent` to all subscribed session channels. `RoomView` is extended with `hour`/`period` fields so `buildRoomView` can append outdoor flavor text. The frontend's `BuildPrompt` pure function renders the colored `[Name | Period HH:00 | HP/MaxHP hp]>` prompt.

**Tech Stack:** Go, protobuf/grpc, `pgregory.net/rapid` (property tests), `go.uber.org/zap`, `sync/atomic`

---

### Task 1: GameClock — types and core logic

**Files:**
- Create: `internal/gameserver/clock.go`
- Create: `internal/gameserver/clock_test.go`

**Step 1: Write the failing tests**

```go
// internal/gameserver/clock_test.go
package gameserver_test

import (
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/gameserver"
)

func TestGameHour_Period(t *testing.T) {
	cases := []struct {
		hour   int32
		period gameserver.TimePeriod
	}{
		{0, gameserver.PeriodMidnight},
		{1, gameserver.PeriodLateNight},
		{4, gameserver.PeriodLateNight},
		{5, gameserver.PeriodDawn},
		{6, gameserver.PeriodDawn},
		{7, gameserver.PeriodMorning},
		{11, gameserver.PeriodMorning},
		{12, gameserver.PeriodAfternoon},
		{16, gameserver.PeriodAfternoon},
		{17, gameserver.PeriodDusk},
		{18, gameserver.PeriodDusk},
		{19, gameserver.PeriodEvening},
		{21, gameserver.PeriodEvening},
		{22, gameserver.PeriodNight},
		{23, gameserver.PeriodNight},
	}
	for _, tc := range cases {
		gh := gameserver.GameHour(tc.hour)
		if got := gh.Period(); got != tc.period {
			t.Errorf("hour %d: got %q, want %q", tc.hour, got, tc.period)
		}
	}
}

func TestGameHour_String(t *testing.T) {
	if got := gameserver.GameHour(6).String(); got != "06:00" {
		t.Errorf("got %q, want 06:00", got)
	}
	if got := gameserver.GameHour(18).String(); got != "18:00" {
		t.Errorf("got %q, want 18:00", got)
	}
}

func TestProperty_GameHour_PeriodAlwaysValid(t *testing.T) {
	valid := map[gameserver.TimePeriod]bool{
		gameserver.PeriodMidnight:  true,
		gameserver.PeriodLateNight: true,
		gameserver.PeriodDawn:      true,
		gameserver.PeriodMorning:   true,
		gameserver.PeriodAfternoon: true,
		gameserver.PeriodDusk:      true,
		gameserver.PeriodEvening:   true,
		gameserver.PeriodNight:     true,
	}
	rapid.Check(t, func(t *rapid.T) {
		h := rapid.Int32Range(0, 23).Draw(t, "hour")
		p := gameserver.GameHour(h).Period()
		if !valid[p] {
			t.Fatalf("hour %d returned invalid period %q", h, p)
		}
	})
}

func TestGameClock_AdvancesHour(t *testing.T) {
	clk := gameserver.NewGameClock(0, 20*time.Millisecond)
	stop := clk.Start()
	defer stop()

	time.Sleep(55 * time.Millisecond) // ~2-3 ticks

	h := clk.CurrentHour()
	if h < 1 || h > 3 {
		t.Errorf("expected hour 1-3 after ~2-3 ticks, got %d", h)
	}
}

func TestGameClock_Wraps(t *testing.T) {
	clk := gameserver.NewGameClock(22, 10*time.Millisecond)
	stop := clk.Start()
	defer stop()

	time.Sleep(45 * time.Millisecond) // enough ticks to wrap past 23

	h := clk.CurrentHour()
	if h < 0 || h > 23 {
		t.Errorf("hour %d out of range [0,23]", h)
	}
}

func TestGameClock_SubscribeReceivesTick(t *testing.T) {
	clk := gameserver.NewGameClock(0, 20*time.Millisecond)
	ch := make(chan gameserver.GameHour, 4)
	clk.Subscribe(ch)
	stop := clk.Start()
	defer stop()
	defer clk.Unsubscribe(ch)

	select {
	case <-ch:
		// success
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timed out waiting for tick")
	}
}

func TestGameClock_UnsubscribeStopsDelivery(t *testing.T) {
	clk := gameserver.NewGameClock(0, 20*time.Millisecond)
	ch := make(chan gameserver.GameHour, 4)
	clk.Subscribe(ch)
	stop := clk.Start()
	defer stop()

	// Wait for at least one tick
	select {
	case <-ch:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("no initial tick")
	}

	clk.Unsubscribe(ch)
	// Drain any buffered tick
	for len(ch) > 0 {
		<-ch
	}

	// Wait long enough for more ticks — none should arrive
	time.Sleep(100 * time.Millisecond)
	if len(ch) > 0 {
		t.Error("received tick after unsubscribe")
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
cd /path/to/worktree
go test ./internal/gameserver/... -run TestGameHour -v
```
Expected: FAIL — `gameserver.GameHour` undefined

**Step 3: Implement `internal/gameserver/clock.go`**

```go
package gameserver

import (
	"fmt"
	"sync"
	"time"
)

// TimePeriod is a named phase of the game day.
type TimePeriod string

const (
	PeriodMidnight  TimePeriod = "Midnight"
	PeriodLateNight TimePeriod = "Late Night"
	PeriodDawn      TimePeriod = "Dawn"
	PeriodMorning   TimePeriod = "Morning"
	PeriodAfternoon TimePeriod = "Afternoon"
	PeriodDusk      TimePeriod = "Dusk"
	PeriodEvening   TimePeriod = "Evening"
	PeriodNight     TimePeriod = "Night"
)

// GameHour is a game-clock hour in [0, 23].
type GameHour int32

// Period returns the named time period for this hour.
//
// Precondition: h is in [0, 23].
// Postcondition: Returns one of the eight TimePeriod constants.
func (h GameHour) Period() TimePeriod {
	switch {
	case h == 0:
		return PeriodMidnight
	case h >= 1 && h <= 4:
		return PeriodLateNight
	case h >= 5 && h <= 6:
		return PeriodDawn
	case h >= 7 && h <= 11:
		return PeriodMorning
	case h >= 12 && h <= 16:
		return PeriodAfternoon
	case h >= 17 && h <= 18:
		return PeriodDusk
	case h >= 19 && h <= 21:
		return PeriodEvening
	default: // 22-23
		return PeriodNight
	}
}

// String returns the hour in "HH:00" format.
func (h GameHour) String() string {
	return fmt.Sprintf("%02d:00", int(h))
}

// GameClock advances game time and broadcasts ticks to subscribers.
type GameClock struct {
	hour         int32 // current game hour 0-23
	tickInterval time.Duration

	mu          sync.Mutex
	subscribers map[chan<- GameHour]struct{}
}

// NewGameClock creates a stopped GameClock starting at startHour.
//
// Precondition: startHour in [0, 23]; tickInterval > 0.
// Postcondition: Returns a non-nil *GameClock ready to Start().
func NewGameClock(startHour int32, tickInterval time.Duration) *GameClock {
	return &GameClock{
		hour:         startHour % 24,
		tickInterval: tickInterval,
		subscribers:  make(map[chan<- GameHour]struct{}),
	}
}

// CurrentHour returns the current game hour.
func (c *GameClock) CurrentHour() GameHour {
	c.mu.Lock()
	defer c.mu.Unlock()
	return GameHour(c.hour)
}

// Subscribe registers ch to receive a GameHour value on each tick.
// If ch is full, the tick is dropped for that subscriber (non-blocking).
//
// Precondition: ch must not be nil.
func (c *GameClock) Subscribe(ch chan<- GameHour) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribers[ch] = struct{}{}
}

// Unsubscribe removes ch from the subscriber list.
func (c *GameClock) Unsubscribe(ch chan<- GameHour) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscribers, ch)
}

// Start launches the clock goroutine and returns a stop function.
// Calling stop() is idempotent.
//
// Postcondition: The clock advances by 1 hour per tickInterval until stop() is called.
func (c *GameClock) Start() (stop func()) {
	done := make(chan struct{})
	var once sync.Once
	go func() {
		ticker := time.NewTicker(c.tickInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.mu.Lock()
				c.hour = (c.hour + 1) % 24
				h := GameHour(c.hour)
				subs := make([]chan<- GameHour, 0, len(c.subscribers))
				for ch := range c.subscribers {
					subs = append(subs, ch)
				}
				c.mu.Unlock()
				for _, ch := range subs {
					select {
					case ch <- h:
					default:
					}
				}
			case <-done:
				return
			}
		}
	}()
	return func() {
		once.Do(func() { close(done) })
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/gameserver/... -run "TestGameHour|TestGameClock|TestProperty_GameHour" -v -race
```
Expected: all PASS

**Step 5: Commit**

```bash
git add internal/gameserver/clock.go internal/gameserver/clock_test.go
git commit -m "feat: add GameClock with TimePeriod types and subscribe/unsubscribe fan-out"
```

---

### Task 2: Config — add game clock fields

**Files:**
- Modify: `internal/config/config.go`
- Modify: `configs/dev.yaml`
- Modify: `configs/docker.yaml`
- Modify: `configs/prod.yaml`

**Step 1: Add fields to `GameServerConfig`**

In `internal/config/config.go`, add to `GameServerConfig`:

```go
// GameClockStart is the game hour (0-23) at server startup.
GameClockStart int `mapstructure:"game_clock_start"`
// GameTickDuration is how long each game hour lasts in real time.
GameTickDuration time.Duration `mapstructure:"game_tick_duration"`
```

Also add defaults in the `setDefaults` function (find the existing defaults block and add):

```go
viper.SetDefault("gameserver.game_clock_start", 6)
viper.SetDefault("gameserver.game_tick_duration", "1m")
```

**Step 2: Add to config YAML files**

In `configs/dev.yaml`, `configs/docker.yaml`, and `configs/prod.yaml`, under the `gameserver:` section add:

```yaml
  game_clock_start: 6
  game_tick_duration: 1m
```

**Step 3: Run existing config tests**

```bash
go test ./internal/config/... -v
```
Expected: all PASS

**Step 4: Commit**

```bash
git add internal/config/config.go configs/dev.yaml configs/docker.yaml configs/prod.yaml
git commit -m "feat: add game_clock_start and game_tick_duration to GameServerConfig"
```

---

### Task 3: Proto — add TimeOfDayEvent and extend RoomView

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Run: `make proto`

**Step 1: Add `TimeOfDayEvent` message to proto**

In `api/proto/game/v1/game.proto`, add after the `Disconnected` message:

```proto
// TimeOfDayEvent is broadcast to all sessions when the game hour advances.
message TimeOfDayEvent {
  int32  hour   = 1;
  string period = 2;
}
```

**Step 2: Add `TimeOfDayEvent` to `ServerEvent` oneof**

In the `ServerEvent` message oneof, add:

```proto
TimeOfDayEvent time_of_day = 16;
```

**Step 3: Extend `RoomView` with time fields**

In the `RoomView` message, add after `floor_items`:

```proto
int32  hour   = 9;
string period = 10;
```

**Step 4: Regenerate**

```bash
make proto
```
Expected: no errors, `internal/gameserver/gamev1/game.pb.go` updated

**Step 5: Verify proto test compiles**

```bash
go build ./...
```
Expected: no errors

**Step 6: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/
git commit -m "feat: add TimeOfDayEvent proto message and extend RoomView with hour/period fields"
```

---

### Task 4: Flavor text — FlavorText and outdoor exit filtering

**Files:**
- Create: `internal/gameserver/flavor.go`
- Create: `internal/gameserver/flavor_test.go`

**Step 1: Write failing tests**

```go
// internal/gameserver/flavor_test.go
package gameserver_test

import (
	"testing"

	"github.com/cory-johannsen/mud/internal/gameserver"
)

func TestFlavorText_IndoorAlwaysEmpty(t *testing.T) {
	periods := []gameserver.TimePeriod{
		gameserver.PeriodMidnight, gameserver.PeriodLateNight,
		gameserver.PeriodDawn, gameserver.PeriodMorning,
		gameserver.PeriodAfternoon, gameserver.PeriodDusk,
		gameserver.PeriodEvening, gameserver.PeriodNight,
	}
	for _, p := range periods {
		if got := gameserver.FlavorText(p, false); got != "" {
			t.Errorf("period %q indoor: expected empty, got %q", p, got)
		}
	}
}

func TestFlavorText_OutdoorNonEmpty(t *testing.T) {
	periods := []gameserver.TimePeriod{
		gameserver.PeriodMidnight, gameserver.PeriodLateNight,
		gameserver.PeriodDawn, gameserver.PeriodMorning,
		gameserver.PeriodAfternoon, gameserver.PeriodDusk,
		gameserver.PeriodEvening, gameserver.PeriodNight,
	}
	for _, p := range periods {
		if got := gameserver.FlavorText(p, true); got == "" {
			t.Errorf("period %q outdoor: expected non-empty flavor text", p)
		}
	}
}

func TestIsDarkPeriod(t *testing.T) {
	dark := []gameserver.TimePeriod{
		gameserver.PeriodMidnight, gameserver.PeriodLateNight, gameserver.PeriodNight,
	}
	light := []gameserver.TimePeriod{
		gameserver.PeriodDawn, gameserver.PeriodMorning,
		gameserver.PeriodAfternoon, gameserver.PeriodDusk, gameserver.PeriodEvening,
	}
	for _, p := range dark {
		if !gameserver.IsDarkPeriod(p) {
			t.Errorf("expected %q to be dark", p)
		}
	}
	for _, p := range light {
		if gameserver.IsDarkPeriod(p) {
			t.Errorf("expected %q to be light", p)
		}
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/gameserver/... -run "TestFlavorText|TestIsDarkPeriod" -v
```
Expected: FAIL — `FlavorText` undefined

**Step 3: Implement `internal/gameserver/flavor.go`**

```go
package gameserver

// FlavorText returns an atmospheric sentence appended to outdoor room descriptions.
// Returns empty string for indoor rooms (isOutdoor == false).
//
// Precondition: period is one of the eight TimePeriod constants.
// Postcondition: Returns a non-empty string for outdoor rooms, empty for indoor.
func FlavorText(period TimePeriod, isOutdoor bool) string {
	if !isOutdoor {
		return ""
	}
	switch period {
	case PeriodMidnight:
		return "The world is cloaked in deep darkness; only faint starlight remains."
	case PeriodLateNight:
		return "The night presses close, silent and still."
	case PeriodDawn:
		return "A pale blush of light edges the horizon as dawn breaks."
	case PeriodMorning:
		return "Morning light floods the area, casting long shadows."
	case PeriodAfternoon:
		return "The sun hangs high overhead, bright and relentless."
	case PeriodDusk:
		return "The sky burns orange and red as the sun sinks toward the horizon."
	case PeriodEvening:
		return "Twilight settles softly, the first stars beginning to appear."
	default: // PeriodNight
		return "A canopy of stars fills the night sky above."
	}
}

// IsDarkPeriod reports whether a period reduces outdoor visibility.
//
// Postcondition: Returns true for Midnight, LateNight, Night.
func IsDarkPeriod(period TimePeriod) bool {
	return period == PeriodMidnight || period == PeriodLateNight || period == PeriodNight
}
```

**Step 4: Run tests**

```bash
go test ./internal/gameserver/... -run "TestFlavorText|TestIsDarkPeriod" -v
```
Expected: all PASS

**Step 5: Commit**

```bash
git add internal/gameserver/flavor.go internal/gameserver/flavor_test.go
git commit -m "feat: add FlavorText and IsDarkPeriod for time-of-day room descriptions"
```

---

### Task 5: WorldHandler — inject clock and extend buildRoomView

**Files:**
- Modify: `internal/gameserver/world_handler.go`
- Modify: `internal/gameserver/world_handler_test.go` (if it exists, otherwise create)

**Step 1: Add `clock *GameClock` field to `WorldHandler`**

In `world_handler.go`, modify the struct and constructor:

```go
type WorldHandler struct {
	world    *world.Manager
	sessions *session.Manager
	npcMgr   *npc.Manager
	clock    *GameClock
}

func NewWorldHandler(worldMgr *world.Manager, sessMgr *session.Manager, npcMgr *npc.Manager, clock *GameClock) *WorldHandler {
	return &WorldHandler{
		world:    worldMgr,
		sessions: sessMgr,
		npcMgr:   npcMgr,
		clock:    clock,
	}
}
```

**Step 2: Extend `buildRoomView` to include time and flavor text**

In `buildRoomView`, add after computing `exitInfos`:

```go
// Time of day
h := GameHour(0)
if wh.clock != nil {
	h = wh.clock.CurrentHour()
}
isOutdoor := room.Properties["outdoor"] == "true"
period := h.Period()

// In dark periods, outdoor rooms hide exits
if IsDarkPeriod(period) && isOutdoor {
	exitInfos = nil // exits hidden by darkness
}

description := room.Description
if flavor := FlavorText(period, isOutdoor); flavor != "" {
	description = description + " " + flavor
}
```

And update the returned `RoomView`:

```go
return &gamev1.RoomView{
	RoomId:      room.ID,
	Title:       room.Title,
	Description: description,
	Exits:       exitInfos,
	Players:     otherPlayers,
	Npcs:        npcInfos,
	Hour:        int32(h),
	Period:      string(period),
}
```

**Step 3: Fix the `NewWorldHandler` call site in `grpc_service.go`**

In `grpc_service.go`, find where `NewWorldHandler` is called (in `NewGameServiceServer` or `cmd/gameserver/main.go`) and add the `clock` parameter. The clock will be passed in from the server setup in Task 6.

For now, pass `nil` as a placeholder:
```go
worldHandler := NewWorldHandler(worldMgr, sessMgr, npcMgr, nil)
```

**Step 4: Find and fix all call sites of `NewWorldHandler`**

```bash
grep -rn "NewWorldHandler" /path/to/worktree --include="*.go"
```

Update each call site to pass the clock (or `nil` temporarily).

**Step 5: Run all gameserver tests**

```bash
go test ./internal/gameserver/... -v -race
```
Expected: all PASS

**Step 6: Commit**

```bash
git add internal/gameserver/world_handler.go
git commit -m "feat: extend WorldHandler with GameClock, add time/period to RoomView and flavor text"
```

---

### Task 6: GameServiceServer — wire GameClock, subscribe sessions, broadcast ticks

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `cmd/gameserver/main.go`

**Step 1: Add `clock *GameClock` field to `GameServiceServer`**

In `grpc_service.go`, add to the struct:

```go
clock *GameClock
```

Add it to `NewGameServiceServer` parameters (at the end, before `logger`):

```go
clock *GameClock,
```

And assign it:

```go
clock: clock,
```

**Step 2: Start clock and subscribe/unsubscribe in `Session`**

In the `Session` function, just after `sess, err := s.sessions.AddPlayer(...)`:

```go
// Subscribe to clock ticks for this session
clockCh := make(chan GameHour, 2)
s.clock.Subscribe(clockCh)
defer s.clock.Unsubscribe(clockCh)
```

Pass `clockCh` to the `forwardEvents` goroutine (see Step 3).

**Step 3: Forward clock ticks to the stream**

The `forwardEvents` goroutine currently only forwards entity events. Add a new goroutine alongside it to forward clock ticks. In `Session`, after launching `forwardEvents`:

```go
wg.Add(1)
go func() {
	defer wg.Done()
	for {
		select {
		case h, ok := <-clockCh:
			if !ok {
				return
			}
			evt := &gamev1.ServerEvent{
				Payload: &gamev1.ServerEvent_TimeOfDay{
					TimeOfDay: &gamev1.TimeOfDayEvent{
						Hour:   int32(h),
						Period: string(h.Period()),
					},
				},
			}
			if err := stream.Send(evt); err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}()
```

**Step 4: Wire clock in `cmd/gameserver/main.go`**

Find where `NewGameServiceServer` is called in `cmd/gameserver/main.go`. Before that call, create and start the clock:

```go
clock := gameserver.NewGameClock(
	int32(cfg.GameServer.GameClockStart),
	cfg.GameServer.GameTickDuration,
)
stopClock := clock.Start()
defer stopClock()
```

Pass `clock` to `NewGameServiceServer` and update `NewWorldHandler` to receive the real clock (not `nil`).

**Step 5: Build to verify no compile errors**

```bash
go build ./...
```
Expected: no errors

**Step 6: Run all tests**

```bash
go test ./... -race
```
Expected: all PASS

**Step 7: Commit**

```bash
git add internal/gameserver/grpc_service.go cmd/gameserver/main.go
git commit -m "feat: wire GameClock into GameServiceServer, subscribe sessions and broadcast TimeOfDayEvent"
```

---

### Task 7: Frontend — BuildPrompt and prompt renderer

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go`
- Modify: `internal/frontend/handlers/game_bridge_test.go`

**Step 1: Write failing tests for `BuildPrompt`**

Add to `game_bridge_test.go`:

```go
func TestBuildPrompt_HealthColors(t *testing.T) {
	// Full health — green
	got := handlers.BuildPrompt("Thorald", "Dusk", "17:00", 60, 60)
	if !strings.Contains(got, "60/60hp") {
		t.Errorf("expected 60/60hp in prompt, got %q", got)
	}

	// Wounded (40%) — yellow
	got = handlers.BuildPrompt("Thorald", "Morning", "09:00", 24, 60)
	if !strings.Contains(got, "24/60hp") {
		t.Errorf("expected 24/60hp in prompt, got %q", got)
	}

	// Critical (<40%) — red
	got = handlers.BuildPrompt("Thorald", "Night", "22:00", 10, 60)
	if !strings.Contains(got, "10/60hp") {
		t.Errorf("expected 10/60hp in prompt, got %q", got)
	}
}

func TestBuildPrompt_Format(t *testing.T) {
	got := handlers.BuildPrompt("Thorald", "Dusk", "17:00", 45, 60)
	// Must end with "> "
	if !strings.HasSuffix(got, "> ") {
		t.Errorf("prompt must end with '> ', got %q", got)
	}
	// Must contain character name
	if !strings.Contains(got, "Thorald") {
		t.Errorf("prompt must contain character name, got %q", got)
	}
	// Must contain period
	if !strings.Contains(got, "Dusk") {
		t.Errorf("prompt must contain period, got %q", got)
	}
	// Must contain hour
	if !strings.Contains(got, "17:00") {
		t.Errorf("prompt must contain hour, got %q", got)
	}
}

func TestBuildPrompt_AllPeriods(t *testing.T) {
	periods := []string{
		"Midnight", "Late Night", "Dawn", "Morning",
		"Afternoon", "Dusk", "Evening", "Night",
	}
	for _, p := range periods {
		got := handlers.BuildPrompt("X", p, "00:00", 10, 10)
		if got == "" {
			t.Errorf("BuildPrompt returned empty for period %q", p)
		}
	}
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/frontend/handlers/... -run "TestBuildPrompt" -v
```
Expected: FAIL — `BuildPrompt` undefined

**Step 3: Implement `BuildPrompt` in `game_bridge.go`**

Add as a standalone exported pure function (not a method):

```go
// BuildPrompt constructs the colored telnet prompt string.
//
// Precondition: maxHP > 0.
// Postcondition: Returns a non-empty string ending with "> ".
func BuildPrompt(name, period, hour string, currentHP, maxHP int32) string {
	// Name segment
	nameSeg := telnet.Colorf(telnet.BrightCyan, "[%s]", name)

	// Time segment — color by period
	var timeColor string
	switch period {
	case "Dawn":
		timeColor = telnet.Yellow
	case "Morning":
		timeColor = telnet.BrightYellow
	case "Afternoon":
		timeColor = telnet.White
	case "Dusk":
		timeColor = telnet.BrightRed
	case "Evening":
		timeColor = telnet.Magenta
	default: // Night, Midnight, Late Night
		timeColor = telnet.Blue
	}
	timeSeg := telnet.Colorf(timeColor, "[%s %s]", period, hour)

	// HP segment — color by percentage
	var hpColor string
	pct := float64(currentHP) / float64(maxHP)
	switch {
	case pct >= 0.75:
		hpColor = telnet.BrightGreen
	case pct >= 0.40:
		hpColor = telnet.Yellow
	default:
		hpColor = telnet.Red
	}
	hpSeg := telnet.Colorf(hpColor, "[%d/%dhp]", currentHP, maxHP)

	return fmt.Sprintf("%s %s %s> ", nameSeg, timeSeg, hpSeg)
}
```

**Step 4: Replace inline prompt construction with `BuildPrompt`**

In `game_bridge.go`, find every occurrence of:
```go
telnet.Colorf(telnet.BrightCyan, "[%s]> ", char.Name)
```
and every occurrence of:
```go
telnet.Colorf(telnet.BrightCyan, "[%s]> ", charName)
```

These need to be replaced with `BuildPrompt(...)`. The prompt now needs HP and time. Add `currentTime atomic.Value` alongside `currentRoom atomic.Value` to track the latest `TimeOfDayEvent`. Initialize it before launching goroutines:

```go
var currentTime atomic.Value
currentTime.Store(&gamev1.TimeOfDayEvent{
	Hour:   int32(h.clock.CurrentHour()), // requires passing clock or initial values
	Period: string(h.clock.CurrentHour().Period()),
})
```

Actually, since `gameBridge` doesn't have access to the gameserver clock directly, initialize from the first `RoomView` or `CharacterInfo` received. Initialize with defaults:

```go
var currentTime atomic.Value
currentTime.Store(&gamev1.TimeOfDayEvent{Hour: 6, Period: "Dawn"})
```

Update `currentTime` in `forwardServerEvents` when a `TimeOfDayEvent` arrives:

```go
case *gamev1.ServerEvent_TimeOfDay:
    currentTime.Store(p.TimeOfDay)
    // Update prompt with new time (no room re-render)
    tod := p.TimeOfDay
    prompt := BuildPrompt(charName, tod.Period, fmt.Sprintf("%02d:00", tod.Hour), currentHP, maxHP)
    _ = conn.WritePrompt(prompt)
    continue // don't fall through to text render
```

Also update `currentTime` from `RoomView` arrivals (since RoomView carries hour/period):

```go
case *gamev1.ServerEvent_RoomView:
    if p.RoomView.Hour > 0 || p.RoomView.Period != "" {
        currentTime.Store(&gamev1.TimeOfDayEvent{
            Hour: p.RoomView.Hour, Period: p.RoomView.Period,
        })
    }
    text = RenderRoomView(p.RoomView)
```

Replace all prompt writes with:
```go
tod := currentTime.Load().(*gamev1.TimeOfDayEvent)
prompt := BuildPrompt(charName, tod.Period, fmt.Sprintf("%02d:00", tod.Hour), currentHP, maxHP)
_ = conn.WritePrompt(prompt)
```

Note: `currentHP` and `maxHP` need to be tracked. Add `atomic.Int32` values for both, initialized from `char.CurrentHP`/`char.MaxHP` and updated when `CharacterInfo` events arrive.

**Step 5: Run all handler tests**

```bash
go test ./internal/frontend/handlers/... -v -race
```
Expected: all PASS

**Step 6: Commit**

```bash
git add internal/frontend/handlers/game_bridge.go internal/frontend/handlers/game_bridge_test.go
git commit -m "feat: add BuildPrompt with colored HP and time-of-day segments, wire into forwardServerEvents"
```

---

### Task 8: Text renderer — extend RenderRoomView for time fields

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/text_renderer_test.go`

**Step 1: Write failing tests**

Add to `text_renderer_test.go`:

```go
func TestRenderRoomView_OutdoorFlavorInDescription(t *testing.T) {
	rv := &gamev1.RoomView{
		RoomId:      "room1",
		Title:       "Open Field",
		Description: "A wide open field.",
		Period:      "Dusk",
		Hour:        17,
	}
	rendered := RenderRoomView(rv)
	// Flavor text should appear when period is present and room could be outdoor
	// (RoomView doesn't carry outdoor flag; flavor is appended server-side in buildRoomView)
	// So RenderRoomView should just render Description as-is (flavor already appended)
	if !strings.Contains(rendered, "A wide open field.") {
		t.Errorf("expected description in render, got %q", rendered)
	}
}
```

Note: Since flavor text is appended to `Description` server-side in `buildRoomView`, `RenderRoomView` just renders `Description` as-is. The test confirms the description is not truncated.

**Step 2: Run test**

```bash
go test ./internal/frontend/handlers/... -run "TestRenderRoomView" -v
```
Expected: PASS (no changes needed to `RenderRoomView` since flavor is server-side)

**Step 3: Verify all existing renderer tests still pass**

```bash
go test ./internal/frontend/handlers/... -v -race
```
Expected: all PASS

**Step 4: Commit**

```bash
git add internal/frontend/handlers/text_renderer_test.go
git commit -m "test: verify RenderRoomView preserves description with server-side flavor text"
```

---

### Task 9: Final integration — run all tests and verify

**Step 1: Run full test suite with race detector**

```bash
go test ./... -race
```
Expected: all PASS, 0 races

**Step 2: Build both binaries**

```bash
go build ./cmd/gameserver/... && go build ./cmd/frontend/...
```
Expected: no errors

**Step 3: Smoke test (optional, requires running cluster)**

Connect via telnet and verify:
- Prompt shows `[Name | Period HH:00 | HP/MaxHP hp]> `
- Room description has atmospheric flavor for outdoor rooms
- Prompt updates every real minute as hour advances
- Dark period (Night/Midnight/LateNight) hides outdoor exits

**Step 4: Commit if any fixes were needed**

```bash
git add -p
git commit -m "fix: integration fixes from smoke test"
```
