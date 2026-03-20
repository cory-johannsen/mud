# Persistent Calendar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `GameCalendar` that tracks in-game day/month, persists them to the DB, and displays them in the room header instead of the prompt.

**Architecture:** `GameCalendar` (server-side) subscribes to `GameClock`, advances day/month on midnight ticks, persists on rollover, and broadcasts `GameDateTime`. The server's gRPC session handler subscribes to `GameCalendar` instead of `GameClock` and sends an extended `TimeOfDayEvent` (with new `day`/`month` proto fields) to connected clients. The frontend `game_bridge.go` receives `TimeOfDayEvent` with day/month, reconstructs a `GameDateTime`, and passes it to `RenderRoomView` which renders date+time in the room name line. `BuildPrompt` loses its time segment.

**Tech Stack:** Go, PostgreSQL (pgx), protobuf/protoc, `sync.Mutex`, `time.Date` for rollover, existing `GameClock` pub/sub pattern.

---

## Task 1: `GameDateTime`, `CalendarRepo`, `GameCalendar` core + unit tests

**Spec refs:** REQ-CAL1 through REQ-CAL8, REQ-CAL18

**Files:**
- Create: `internal/gameserver/calendar.go`
- Create: `internal/gameserver/calendar_test.go`

**Context:** The existing `GameClock` in `internal/gameserver/clock.go` advances hour 0–23, broadcasts `GameHour` to `map[chan<- GameHour]struct{}` under a `sync.Mutex`. `GameHour` has `.Period()` and `.String()` methods. Mirror that pattern exactly. Run tests with `go test ./internal/gameserver/...`.

- [ ] **Step 1: Write failing tests for `FormatDate`**

Create `internal/gameserver/calendar_test.go`:

```go
package gameserver

import (
	"fmt"
	"testing"
	"time"
)

func TestFormatDate_OrdinalSuffixes(t *testing.T) {
	cases := []struct {
		month, day int
		want       string
	}{
		{1, 1, "January 1st"},
		{2, 2, "February 2nd"},
		{3, 3, "March 3rd"},
		{4, 4, "April 4th"},
		{11, 11, "November 11th"},
		{12, 12, "December 12th"},
		{5, 13, "May 13th"},
		{6, 21, "June 21st"},
		{7, 22, "July 22nd"},
		{8, 23, "August 23rd"},
		{9, 31, "September 31st"},
	}
	for _, c := range cases {
		got := FormatDate(c.month, c.day)
		if got != c.want {
			t.Errorf("FormatDate(%d, %d) = %q, want %q", c.month, c.day, got, c.want)
		}
	}
}
```

- [ ] **Step 2: Run test — expect compile error (FormatDate not defined)**

```bash
go test ./internal/gameserver/... -run TestFormatDate -v 2>&1 | head -20
```

- [ ] **Step 3: Create `internal/gameserver/calendar.go`**

```go
package gameserver

import (
	"fmt"
	"sync"
	"time"
)

// GameDateTime is the combined in-game date and time broadcast on every clock tick.
type GameDateTime struct {
	Hour  GameHour // 0–23
	Day   int      // 1–28/29/30/31
	Month int      // 1–12
}

// CalendarRepo persists the in-game day and month across server restarts.
//
// Precondition for Load: table may be empty (first boot).
// Postcondition for Load: returns (1, 1, nil) when no row exists.
// Postcondition for Save: upserts the single calendar row (id=1).
type CalendarRepo interface {
	Load() (day, month int, err error)
	Save(day, month int) error
}

// FormatDate returns the month name and day with correct ordinal suffix.
// e.g. FormatDate(1, 1) → "January 1st", FormatDate(11, 11) → "November 11th"
//
// Precondition: month in [1,12]; day in [1,31].
func FormatDate(month, day int) string {
	return fmt.Sprintf("%s %d%s", time.Month(month).String(), day, ordinalSuffix(day))
}

// ordinalSuffix returns "st", "nd", "rd", or "th" for a given day number.
func ordinalSuffix(day int) string {
	if day >= 11 && day <= 13 {
		return "th"
	}
	switch day % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	default:
		return "th"
	}
}

// GameCalendar subscribes to a GameClock and advances day/month on midnight ticks.
// Broadcasts GameDateTime to all subscribers on every tick.
// Thread-safe: subscriber map and day/month state are protected by mu.
type GameCalendar struct {
	clock       *GameClock
	mu          sync.Mutex
	day         int
	month       int
	repo        CalendarRepo
	logger      interface{ Warn(string, ...interface{}) }
	subscribers map[chan<- GameDateTime]struct{}
}

// NewGameCalendar creates a GameCalendar starting at the given day and month.
//
// Precondition: clock != nil; day in [1,31]; month in [1,12]; repo != nil.
// Postcondition: Returns a non-nil *GameCalendar ready to Start().
func NewGameCalendar(clock *GameClock, day, month int, repo CalendarRepo) *GameCalendar {
	return &GameCalendar{
		clock:       clock,
		day:         day,
		month:       month,
		repo:        repo,
		subscribers: make(map[chan<- GameDateTime]struct{}),
	}
}

// SetLogger attaches a warn-level logger for Save failures. Optional.
func (c *GameCalendar) SetLogger(l interface{ Warn(string, ...interface{}) }) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = l
}

// CurrentDateTime returns the current in-game date and time.
func (c *GameCalendar) CurrentDateTime() GameDateTime {
	c.mu.Lock()
	defer c.mu.Unlock()
	return GameDateTime{Hour: c.clock.CurrentHour(), Day: c.day, Month: c.month}
}

// Subscribe registers ch to receive a GameDateTime on each clock tick.
//
// Precondition: ch must not be nil.
func (c *GameCalendar) Subscribe(ch chan<- GameDateTime) {
	if ch == nil {
		panic("GameCalendar.Subscribe: ch must not be nil")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subscribers[ch] = struct{}{}
}

// Unsubscribe removes ch from the subscriber list.
func (c *GameCalendar) Unsubscribe(ch chan<- GameDateTime) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.subscribers, ch)
}

// Start subscribes to the GameClock and launches the calendar goroutine.
// Returns a stop function; calling it is idempotent.
//
// Precondition: Must not be called more than once per GameCalendar instance.
func (c *GameCalendar) Start() (stop func()) {
	clockCh := make(chan GameHour, 2)
	c.clock.Subscribe(clockCh)

	done := make(chan struct{})
	var once sync.Once
	go func() {
		for {
			select {
			case h, ok := <-clockCh:
				if !ok {
					return
				}
				c.mu.Lock()
				// Rollover at midnight: advance day/month BEFORE broadcast
				// so subscribers receive the new day at hour 0, not the old day.
				if h == 0 {
					next := time.Date(2001, time.Month(c.month), c.day+1, 0, 0, 0, 0, time.UTC)
					c.day, c.month = next.Day(), int(next.Month())
					day, month := c.day, c.month
					repo := c.repo
					logger := c.logger
					c.mu.Unlock()
					if err := repo.Save(day, month); err != nil && logger != nil {
						logger.Warn("GameCalendar: failed to save day/month", "error", err)
					}
					c.mu.Lock()
				}
				dt := GameDateTime{Hour: h, Day: c.day, Month: c.month}
				subs := make([]chan<- GameDateTime, 0, len(c.subscribers))
				for ch := range c.subscribers {
					subs = append(subs, ch)
				}
				c.mu.Unlock()
				for _, ch := range subs {
					select {
					case ch <- dt:
					default:
					}
				}
			case <-done:
				c.clock.Unsubscribe(clockCh)
				return
			}
		}
	}()
	return func() {
		once.Do(func() { close(done) })
	}
}
```

- [ ] **Step 4: Run `FormatDate` tests — expect PASS**

```bash
go test ./internal/gameserver/... -run TestFormatDate -v 2>&1
```

- [ ] **Step 5: Write failing tests for `GameCalendar` rollover and broadcast**

Append to `internal/gameserver/calendar_test.go`:

```go
func TestGameCalendar_BroadcastsEveryTick(t *testing.T) {
	clock := NewGameClock(6, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 1, 1, &noopRepo{})
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	select {
	case dt := <-ch:
		if dt.Day != 1 || dt.Month != 1 {
			t.Errorf("unexpected dt: %+v", dt)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for GameDateTime broadcast")
	}
}

func TestGameCalendar_DayAdvancesByOne(t *testing.T) {
	// Basic: Jan 5 → Jan 6 at midnight.
	clock := NewGameClock(23, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 5, 1, &noopRepo{})
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	var got GameDateTime
	for i := 0; i < 10; i++ {
		select {
		case got = <-ch:
			if got.Hour == 0 {
				goto check
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for midnight tick")
		}
	}
check:
	if got.Day != 6 || got.Month != 1 {
		t.Errorf("after Jan 5 midnight got day=%d month=%d, want day=6 month=1", got.Day, got.Month)
	}
}

func TestGameCalendar_JanRollover(t *testing.T) {
	// Jan 31 → Feb 1.
	clock := NewGameClock(23, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 31, 1, &noopRepo{})
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	var got GameDateTime
	for i := 0; i < 10; i++ {
		select {
		case got = <-ch:
			if got.Hour == 0 {
				goto check
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for midnight tick")
		}
	}
check:
	if got.Day != 1 || got.Month != 2 {
		t.Errorf("after Jan 31 midnight got day=%d month=%d, want day=1 month=2", got.Day, got.Month)
	}
}

func TestGameCalendar_FebRollover(t *testing.T) {
	// Feb 28 → Mar 1 (year 2001 is not a leap year).
	clock := NewGameClock(23, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 28, 2, &noopRepo{})
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	var got GameDateTime
	for i := 0; i < 10; i++ {
		select {
		case got = <-ch:
			if got.Hour == 0 {
				goto check
			}
		case <-time.After(500 * time.Millisecond):
			t.Fatal("timed out waiting for midnight tick")
		}
	}
check:
	if got.Day != 1 || got.Month != 3 {
		t.Errorf("after Feb 28 midnight got day=%d month=%d, want day=1 month=3", got.Day, got.Month)
	}
}

func TestGameCalendar_NoSubscribers_NoPanic(t *testing.T) {
	clock := NewGameClock(6, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 1, 1, &noopRepo{})
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()
	time.Sleep(120 * time.Millisecond)
}

func TestGameCalendar_SaveFailure_DoesNotStopBroadcast(t *testing.T) {
	clock := NewGameClock(23, 50*time.Millisecond)
	cal := NewGameCalendar(clock, 31, 1, &failRepo{})
	ch := make(chan GameDateTime, 4)
	cal.Subscribe(ch)
	stopCal := cal.Start()
	defer stopCal()
	stopClk := clock.Start()
	defer stopClk()

	select {
	case <-ch:
		// success — broadcast continued despite save failure
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out — broadcast stopped after Save failure")
	}
}

// noopRepo is a CalendarRepo stub that does nothing.
type noopRepo struct{}

func (r *noopRepo) Load() (int, int, error) { return 1, 1, nil }
func (r *noopRepo) Save(_, _ int) error     { return nil }

// failRepo is a CalendarRepo stub whose Save always fails.
type failRepo struct{}

func (r *failRepo) Load() (int, int, error) { return 1, 1, nil }
func (r *failRepo) Save(_, _ int) error     { return fmt.Errorf("db error") }
```

- [ ] **Step 6: Run all calendar tests — expect PASS**

```bash
go test ./internal/gameserver/... -run "TestFormatDate|TestGameCalendar" -v 2>&1
```

Expected: all PASS.

- [ ] **Step 7: Run full gameserver suite**

```bash
go test ./internal/gameserver/... 2>&1
```

Expected: `ok`

- [ ] **Step 8: Commit**

```bash
git add internal/gameserver/calendar.go internal/gameserver/calendar_test.go
git commit -m "feat: add GameCalendar, GameDateTime, CalendarRepo, FormatDate"
```

---

## Task 2: DB migration + `PostgresCalendarRepo`

**Spec refs:** REQ-CAL9, REQ-CAL10, REQ-CAL19

**Files:**
- Create: `migrations/030_world_calendar.up.sql`
- Create: `migrations/030_world_calendar.down.sql`
- Create: `internal/storage/postgres/calendar.go`
- Create: `internal/storage/postgres/calendar_test.go`

**Context:** Look at `internal/storage/postgres/character_hero_points_test.go` for the postgres integration test pattern (uses `testDB(t)`). Look at `internal/storage/postgres/automap.go` for the upsert pattern. Migration `030` follows the existing `029_innate_uses_remaining`.

- [ ] **Step 1: Create migration files**

`migrations/030_world_calendar.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS world_calendar (
    id    INTEGER PRIMARY KEY DEFAULT 1,
    day   INTEGER NOT NULL,
    month INTEGER NOT NULL
);
```

`migrations/030_world_calendar.down.sql`:
```sql
DROP TABLE IF EXISTS world_calendar;
```

- [ ] **Step 2: Write failing repo tests**

Create `internal/storage/postgres/calendar_test.go`:

```go
package postgres_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestCalendarRepo_Load_EmptyTable_ReturnsDefault(t *testing.T) {
	db := testDB(t)
	repo := postgres.NewCalendarRepo(db)
	day, month, err := repo.Load()
	require.NoError(t, err)
	assert.Equal(t, 1, day)
	assert.Equal(t, 1, month)
}

func TestCalendarRepo_SaveAndLoad_RoundTrip(t *testing.T) {
	db := testDB(t)
	repo := postgres.NewCalendarRepo(db)
	require.NoError(t, repo.Save(15, 7))
	day, month, err := repo.Load()
	require.NoError(t, err)
	assert.Equal(t, 15, day)
	assert.Equal(t, 7, month)
}

func TestCalendarRepo_Save_Upserts(t *testing.T) {
	db := testDB(t)
	repo := postgres.NewCalendarRepo(db)
	require.NoError(t, repo.Save(1, 1))
	require.NoError(t, repo.Save(28, 2))
	day, month, err := repo.Load()
	require.NoError(t, err)
	assert.Equal(t, 28, day)
	assert.Equal(t, 2, month)
}
```

- [ ] **Step 3: Run tests — expect compile error**

```bash
go test ./internal/storage/postgres/... -run TestCalendarRepo -v 2>&1 | head -20
```

- [ ] **Step 4: Create `internal/storage/postgres/calendar.go`**

```go
package postgres

import (
	"database/sql"
	"fmt"
)

// CalendarRepo persists the in-game day and month to the world_calendar table.
type CalendarRepo struct {
	db *sql.DB
}

// NewCalendarRepo creates a CalendarRepo backed by db.
//
// Precondition: db != nil and the world_calendar migration has been applied.
func NewCalendarRepo(db *sql.DB) *CalendarRepo {
	return &CalendarRepo{db: db}
}

// Load returns the persisted day and month.
// Returns (1, 1, nil) when no row exists (first boot).
func (r *CalendarRepo) Load() (day, month int, err error) {
	row := r.db.QueryRow(`SELECT day, month FROM world_calendar WHERE id = 1`)
	if scanErr := row.Scan(&day, &month); scanErr != nil {
		if scanErr == sql.ErrNoRows {
			return 1, 1, nil
		}
		return 0, 0, fmt.Errorf("calendar load: %w", scanErr)
	}
	return day, month, nil
}

// Save upserts the single calendar row (id=1) with day and month.
//
// Precondition: day in [1,31], month in [1,12].
func (r *CalendarRepo) Save(day, month int) error {
	_, err := r.db.Exec(`
		INSERT INTO world_calendar (id, day, month)
		VALUES (1, $1, $2)
		ON CONFLICT (id) DO UPDATE SET day = EXCLUDED.day, month = EXCLUDED.month
	`, day, month)
	if err != nil {
		return fmt.Errorf("calendar save: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run repo tests — expect PASS**

```bash
go test ./internal/storage/postgres/... -run TestCalendarRepo -v 2>&1
```

- [ ] **Step 6: Commit**

```bash
git add migrations/030_world_calendar.up.sql migrations/030_world_calendar.down.sql \
    internal/storage/postgres/calendar.go internal/storage/postgres/calendar_test.go
git commit -m "feat: add world_calendar migration and PostgresCalendarRepo"
```

---

## Task 3: Extend proto + wire `GameCalendar` into server

**Spec refs:** REQ-CAL11, REQ-CAL15, REQ-CAL16

**Files:**
- Modify: `api/proto/game/v1/game.proto`
- Regenerate: `internal/gameserver/gamev1/game.pb.go` (via `make proto`)
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `cmd/gameserver/main.go`

**Context:** The gRPC stream carries a `TimeOfDayEvent` with `hour` and `period` fields. Day/month must be added to the same message so the frontend receives them without a new event type. Proto generation: `make proto`. The `GameServiceServer` struct field `clock *GameClock` is at line 176 of `grpc_service.go`. The session clock subscription block starts at line 1101. The `NewGameServiceServer` call in `cmd/gameserver/main.go` is around line 565 — add `gameCalendar` as the **last** parameter before the final closing parenthesis.

- [ ] **Step 1: Extend `TimeOfDayEvent` in the proto**

In `api/proto/game/v1/game.proto`, find the `TimeOfDayEvent` message (around line 285):
```proto
message TimeOfDayEvent {
  int32  hour   = 1;
  string period = 2;
}
```
Change to:
```proto
message TimeOfDayEvent {
  int32  hour   = 1;
  string period = 2;
  int32  day    = 3;
  int32  month  = 4;
}
```

- [ ] **Step 2: Regenerate proto bindings**

```bash
make proto 2>&1
```

Expected: no errors; `internal/gameserver/gamev1/game.pb.go` updated with `Day` and `Month` fields on `TimeOfDayEvent`.

- [ ] **Step 3: Add `calendar` field to `GameServiceServer` and update constructor**

In `internal/gameserver/grpc_service.go`:

Add the field after `clock *GameClock` (line ~176):
```go
calendar *GameCalendar
```

Add `calendar *GameCalendar` as a new parameter to `NewGameServiceServer` (insert after the `clock *GameClock` parameter). Assign in the constructor body:
```go
calendar: calendar,
```

- [ ] **Step 4: Switch session subscription from `GameClock` to `GameCalendar`**

In the session handler in `internal/gameserver/grpc_service.go`, replace the block at lines ~1101–1106:
```go
// Old:
var clockCh chan GameHour
if s.clock != nil {
    clockCh = make(chan GameHour, 2)
    s.clock.Subscribe(clockCh)
    defer s.clock.Unsubscribe(clockCh)
}
```
With:
```go
// New:
var calCh chan GameDateTime
if s.calendar != nil {
    calCh = make(chan GameDateTime, 2)
    s.calendar.Subscribe(calCh)
    defer s.calendar.Unsubscribe(calCh)
}
```

Then find the goroutine that reads `clockCh` (around line 1123) and update it to read `calCh` and send the richer event:
```go
// Old goroutine condition: if clockCh != nil {
// Old channel read: case h, ok := <-clockCh:
// Old event build: TimeOfDayEvent{Hour: int32(h), Period: string(period)}

// New goroutine condition: if calCh != nil {
if calCh != nil {
    wg.Add(1)
    go func() {
        defer wg.Done()
        lastPeriod := TimePeriod(roomView.GetPeriod())
        for {
            select {
            case dt, ok := <-calCh:
                if !ok {
                    return
                }
                period := dt.Hour.Period()
                evt := &gamev1.ServerEvent{
                    Payload: &gamev1.ServerEvent_TimeOfDay{
                        TimeOfDay: &gamev1.TimeOfDayEvent{
                            Hour:   int32(dt.Hour),
                            Period: string(period),
                            Day:    int32(dt.Day),
                            Month:  int32(dt.Month),
                        },
                    },
                }
                if err := stream.Send(evt); err != nil {
                    return
                }
                if period != lastPeriod {
                    lastPeriod = period
                    if sess, ok := s.sessions.GetPlayer(uid); ok {
                        if room, ok := s.world.GetRoom(sess.RoomID); ok {
                            rv := s.worldH.buildRoomView(uid, room)
                            rvEvt := &gamev1.ServerEvent{
                                Payload: &gamev1.ServerEvent_RoomView{RoomView: rv},
                            }
                            if data, err := proto.Marshal(rvEvt); err == nil {
                                _ = sess.Entity.Push(data)
                            }
                        }
                    }
                }
            case <-ctx.Done():
                return
            }
        }
    }()
}
```

- [ ] **Step 5: Wire calendar in `cmd/gameserver/main.go`**

After the existing `gameClock` construction block (around line 522):
```go
// Load calendar state and start GameCalendar.
calendarRepo := postgres.NewCalendarRepo(pool.DB())
calDay, calMonth, err := calendarRepo.Load()
if err != nil {
    logger.Fatal("loading calendar state", zap.Error(err))
}
gameCalendar := gameserver.NewGameCalendar(gameClock, calDay, calMonth, calendarRepo)
stopCalendar := gameCalendar.Start()
defer stopCalendar()
```

Pass `gameCalendar` to `NewGameServiceServer` — add it as the **last** argument in the existing long call (before the closing `)`).

- [ ] **Step 6: Build to verify**

```bash
go build ./cmd/gameserver/... 2>&1
```

Expected: no errors.

- [ ] **Step 7: Run full test suites**

```bash
go test ./internal/gameserver/... ./internal/storage/postgres/... 2>&1
```

Expected: `ok` for both.

- [ ] **Step 8: Commit**

```bash
git add api/proto/game/v1/game.proto internal/gameserver/gamev1/game.pb.go \
    internal/gameserver/grpc_service.go cmd/gameserver/main.go
git commit -m "feat: extend TimeOfDayEvent proto with day/month; wire GameCalendar into server"
```

---

## Task 4: Update `RenderRoomView` — add date/time to room header

**Spec refs:** REQ-CAL12, REQ-CAL20

**Files:**
- Modify: `internal/frontend/handlers/text_renderer.go`
- Modify: `internal/frontend/handlers/text_renderer_test.go`

**Context:** `RenderRoomView` is at line 18 of `text_renderer.go`. The `handlers` package is a gRPC client package — it must import `"github.com/cory-johannsen/mud/internal/gameserver"` for `GameDateTime` and `FormatDate`. This import does NOT create a cycle because `gameserver` does not import `frontend`.

The test file `text_renderer_test.go` has **14 `RenderRoomView` call sites** at the following lines: 28, 45, 68, 108, 367, 389 (×2 in the same expression — both deleted), 863, 883, 904, 914, 932, 960. All 3-argument calls become 4-argument calls. The test `TestRenderRoomView_TimeFields_NoExtraOutput` (lines 374–391) is entirely replaced.

- [ ] **Step 1: Update `RenderRoomView` signature and room title line**

In `internal/frontend/handlers/text_renderer.go`:

Add import `"github.com/cory-johannsen/mud/internal/gameserver"` and `"fmt"` (if not already present).

Change line 18 from:
```go
func RenderRoomView(rv *gamev1.RoomView, width int, maxLines int) string {
```
To:
```go
func RenderRoomView(rv *gamev1.RoomView, width int, maxLines int, dt gameserver.GameDateTime) string {
```

Change lines 22–24 (the title block) from:
```go
if rv.Title != "" {
    lines = append(lines, telnet.Colorize(telnet.BrightYellow, rv.Title))
}
```
To:
```go
if rv.Title != "" {
    dateStr := gameserver.FormatDate(dt.Month, dt.Day)
    periodStr := string(dt.Hour.Period())
    hourStr := dt.Hour.String()
    header := fmt.Sprintf("%s — %s %s %s", rv.Title, dateStr, periodStr, hourStr)
    lines = append(lines, telnet.Colorize(telnet.BrightYellow, header))
}
```

- [ ] **Step 2: Update all `RenderRoomView` call sites in `text_renderer_test.go`**

Add import `"github.com/cory-johannsen/mud/internal/gameserver"` to `text_renderer_test.go`.

Define a package-level helper at the top of the test file (after imports) for convenience:
```go
var testDT = gameserver.GameDateTime{Hour: 7, Day: 1, Month: 1}
```

Update every call site — each `RenderRoomView(rv, W, N)` becomes `RenderRoomView(rv, W, N, testDT)`:
- Line 28: `RenderRoomView(rv, 80, 0, testDT)`
- Line 45: `RenderRoomView(rv, 80, 0, testDT)`
- Line 68: `RenderRoomView(rv, 80, 0, testDT)`
- Line 108: `RenderRoomView(rv, 80, 0, testDT)`
- Line 367: `RenderRoomView(rv, 200, 0, testDT)` (note: width 200, not 80)
- Lines 863, 883, 904, 914, 932, 960: each `RenderRoomView(rv, 80, 0, testDT)`

Replace `TestRenderRoomView_TimeFields_NoExtraOutput` (lines 374–391, the entire function) with:

```go
func TestRenderRoomView_DateTimeInHeader(t *testing.T) {
	rv := &gamev1.RoomView{
		Title:       "Test Room",
		Description: "A plain room.",
	}
	dt := gameserver.GameDateTime{Hour: 7, Day: 15, Month: 6}
	rendered := RenderRoomView(rv, 80, 0, dt)
	if !strings.Contains(rendered, "June 15th") {
		t.Errorf("room header must contain date, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Morning") {
		t.Errorf("room header must contain period, got:\n%s", rendered)
	}
	if !strings.Contains(rendered, "07:00") {
		t.Errorf("room header must contain hour, got:\n%s", rendered)
	}
}
```

Also update `TestRenderRoomView_WithTimeFields_DescriptionPreserved` (line 357). It tests that the description is preserved regardless of `Hour`/`Period` in the `RoomView` proto. After the change, description is always rendered; the test just needs a `dt` argument:

```go
func TestRenderRoomView_WithTimeFields_DescriptionPreserved(t *testing.T) {
	rv := &gamev1.RoomView{
		Title:       "Neon Alley",
		Description: "A rain-slicked alley.",
		Hour:        17,
		Period:      "Dusk",
	}
	dt := gameserver.GameDateTime{Hour: 17, Day: 1, Month: 1}
	rendered := RenderRoomView(rv, 200, 0, dt)
	if !strings.Contains(rendered, "rain-slicked alley") {
		t.Errorf("description must appear in output, got:\n%s", rendered)
	}
}
```

- [ ] **Step 3: Run text_renderer tests — expect PASS**

```bash
go test ./internal/frontend/handlers/... -run TestRenderRoomView -v 2>&1
```

Expected: all PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/frontend/handlers/text_renderer.go internal/frontend/handlers/text_renderer_test.go
git commit -m "feat: add date/time to room header in RenderRoomView"
```

---

## Task 5: Update `BuildPrompt` + `game_bridge.go` — remove time, use `GameDateTime` for display

**Spec refs:** REQ-CAL13, REQ-CAL14, REQ-CAL15, REQ-CAL20

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go`
- Modify: `internal/frontend/handlers/game_bridge_test.go`

**Context:** `BuildPrompt` is at line 34 of `game_bridge.go`. The `forwardServerEvents` method is on `*AuthHandler` (line 558) — not `GameHandler`, there is no `GameHandler`. The calendar date arrives via the gRPC stream as a `TimeOfDayEvent` with new `Day`/`Month` fields (added in Task 3). The current flow stores `*gamev1.TimeOfDayEvent` in `currentTime atomic.Value` for both the prompt and `RenderMessage`. After this change: the prompt drops time, a new `currentDT atomic.Value` (storing `*gameserver.GameDateTime`) drives `RenderRoomView`, and `currentTime` is kept only to feed `period` to `RenderMessage`.

`game_bridge_test.go` has these `BuildPrompt` call sites that must all be updated:
- Line 20 (`TestBuildPrompt_Format`)
- Line 41 (`TestBuildPrompt_HealthColors`)
- Line 63 (`TestBuildPrompt_AllPeriods` — also remove `period`/`hour` loop variables)
- Line 79 (property test `TestProperty_BuildPrompt_AlwaysEndsWithPromptSuffix` — remove `period` and `hour` draws)
- Line 302 (`TestBuildPrompt_NoConditions_FormatUnchanged`)
- Line 315 (`TestBuildPrompt_OneCondition`)
- Line 325 (`TestBuildPrompt_MultipleConditions`)
- Line 363 (`TestProperty_BuildPrompt_ConditionsAllPresent` — remove `period` draw)
- Line 389 (`TestBuildPrompt_ConditionAppliedThenRemoved` — two call sites inside the test)

- [ ] **Step 1: Change `BuildPrompt` signature — remove `period` and `hour`**

In `internal/frontend/handlers/game_bridge.go`:

Change the signature from:
```go
func BuildPrompt(name, period, hour string, currentHP, maxHP int32, conditions []string) string {
```
To:
```go
func BuildPrompt(name string, currentHP, maxHP int32, conditions []string) string {
```

Remove the precondition comment mentioning `period` and `hour`.

Remove the time segment: delete the `var timeColor string`, the `switch period { ... }` block, and the `timeSeg := ...` line.

In `parts := []string{nameSeg, timeSeg, hpSeg}`, remove `timeSeg`:
```go
parts := []string{nameSeg, hpSeg}
```

- [ ] **Step 2: Add `currentDT` atomic and update `buildCurrentPrompt`**

Add import `"github.com/cory-johannsen/mud/internal/gameserver"` to `game_bridge.go`.

In the `gameBridge` method (around line 188), after the existing `currentTime` atomic initialization, add:
```go
// currentDT tracks the latest GameDateTime for RenderRoomView.
var currentDT atomic.Value
currentDT.Store(&gameserver.GameDateTime{Hour: 6, Day: 1, Month: 1})
```

Change `buildCurrentPrompt` (around line 207) from:
```go
buildCurrentPrompt := func() string {
    tod := currentTime.Load().(*gamev1.TimeOfDayEvent)
    ...
    return BuildPrompt(char.Name, tod.Period, fmt.Sprintf("%02d:00", tod.Hour), currentHP.Load(), maxHP.Load(), names)
}
```
To:
```go
buildCurrentPrompt := func() string {
    condMu.Lock()
    sortedIDs := slices.Sorted(maps.Keys(activeConditions))
    names := make([]string, 0, len(sortedIDs))
    for _, id := range sortedIDs {
        names = append(names, activeConditions[id])
    }
    condMu.Unlock()
    return BuildPrompt(char.Name, currentHP.Load(), maxHP.Load(), names)
}
```

Update the initial `RenderRoomView` call (line ~249) to pass `currentDT`. Also seed `currentDT` from the initial room view's `Hour` field (the existing code at line ~260 seeds `currentTime` from `rv.GetPeriod()` — mirror this for `currentDT`):
```go
// Seed currentDT from the initial RoomView so the first render shows the correct hour.
if rv.GetPeriod() != "" {
    currentDT.Store(&gameserver.GameDateTime{
        Hour:  gameserver.GameHour(rv.GetHour()),
        Day:   1,
        Month: 1,
    })
}
dt := currentDT.Load().(*gameserver.GameDateTime)
renderedRoom := RenderRoomView(rv, w, telnet.RoomRegionRows, *dt)
```

(The day/month will be 1/1 until the first `TimeOfDayEvent` arrives with the real day/month from the server. This is correct first-boot behavior per REQ-CAL11.)

- [ ] **Step 3: Update `forwardServerEvents` signature to accept `currentDT`**

The `forwardServerEvents` method signature (line 558) currently takes `currentTime *atomic.Value`. Add `currentDT *atomic.Value` as a new parameter after `currentTime`. The complete updated signature becomes:
```go
func (h *AuthHandler) forwardServerEvents(ctx context.Context, stream gamev1.GameService_SessionClient, conn *telnet.Conn, charName string, currentRoom *atomic.Value, currentTime *atomic.Value, currentDT *atomic.Value, currentHP *atomic.Int32, maxHP *atomic.Int32, lastRoomView *atomic.Value, buildPrompt func() string, condMu *sync.Mutex, activeConditions map[string]string) {
```

Update the call site (line ~337) from:
```go
h.forwardServerEvents(streamCtx, stream, conn, char.Name, &currentRoom, &currentTime, &currentHP, &maxHP, &lastRoomView, buildCurrentPrompt, &condMu, activeConditions)
```
To:
```go
h.forwardServerEvents(streamCtx, stream, conn, char.Name, &currentRoom, &currentTime, &currentDT, &currentHP, &maxHP, &lastRoomView, buildCurrentPrompt, &condMu, activeConditions)
```

In the `case *gamev1.ServerEvent_TimeOfDay:` branch (line ~617):
```go
case *gamev1.ServerEvent_TimeOfDay:
    currentTime.Store(p.TimeOfDay)
    // Update currentDT with the richer GameDateTime from the extended event.
    dt := &gameserver.GameDateTime{
        Hour:  gameserver.GameHour(p.TimeOfDay.GetHour()),
        Day:   int(p.TimeOfDay.GetDay()),
        Month: int(p.TimeOfDay.GetMonth()),
    }
    currentDT.Store(dt)
    if conn.IsSplitScreen() {
        _ = conn.WritePromptSplit(buildPrompt())
    } else {
        _ = conn.WritePrompt(buildPrompt())
    }
    continue
```

In the `case *gamev1.ServerEvent_RoomView:` branch, update `RenderRoomView` calls (lines ~629–632) to use `currentDT`:
```go
case *gamev1.ServerEvent_RoomView:
    if roomID := p.RoomView.GetRoomId(); roomID != "" {
        currentRoom.Store(roomID)
    }
    if p.RoomView.GetPeriod() != "" {
        currentTime.Store(&gamev1.TimeOfDayEvent{Hour: p.RoomView.GetHour(), Period: p.RoomView.GetPeriod()})
        // If a RoomView arrives without day/month (e.g. push from period-change watcher),
        // keep the existing currentDT day/month, update only the hour.
        if existing, ok := currentDT.Load().(*gameserver.GameDateTime); ok && existing != nil {
            currentDT.Store(&gameserver.GameDateTime{
                Hour:  gameserver.GameHour(p.RoomView.GetHour()),
                Day:   existing.Day,
                Month: existing.Month,
            })
        }
    }
    w, _ := conn.Dimensions()
    dt := currentDT.Load().(*gameserver.GameDateTime)
    text = RenderRoomView(p.RoomView, w, telnet.RoomRegionRows, *dt)
    lastRoomView.Store(p.RoomView)
```

Update the resize handler (line ~595) similarly:
```go
if rv, ok := lastRoomView.Load().(*gamev1.RoomView); ok && rv != nil {
    dt := currentDT.Load().(*gameserver.GameDateTime)
    _ = conn.WriteRoom(RenderRoomView(rv, rw, telnet.RoomRegionRows, *dt))
}
```

Keep `currentTime` and the `case *gamev1.ServerEvent_Message:` branch (line ~636) unchanged — `RenderMessage` still uses `period` from `currentTime`.

- [ ] **Step 4: Update `game_bridge_test.go` — fix all `BuildPrompt` call sites**

For each call site listed in the context, remove the `period` and `hour` string arguments:

`TestBuildPrompt_Format` (line 20):
```go
// Old: handlers.BuildPrompt("Thorald", "Dusk", "17:00", 45, 60, nil)
got := handlers.BuildPrompt("Thorald", 45, 60, nil)
// Update assertion: no time segment, so expect 2 bracket groups not 3.
```

`TestBuildPrompt_HealthColors` (line 41):
```go
got := handlers.BuildPrompt("Thorald", 60, 60, nil)
```

`TestBuildPrompt_AllPeriods` (line 57): Remove the `periods` slice and the loop variable `p`. Simplify to a single call:
```go
func TestBuildPrompt_AllPeriods(t *testing.T) {
    got := handlers.BuildPrompt("X", 10, 10, nil)
    if got == "" {
        t.Error("BuildPrompt returned empty string")
    }
    if !strings.HasSuffix(got, "> ") {
        t.Errorf("prompt must end with '> ', got %q", got)
    }
}
```

`TestProperty_BuildPrompt_AlwaysEndsWithPromptSuffix` (property test, ~line 74): Remove `period` and `hour` Draw calls. The test becomes:
```go
rapid.Check(t, func(rt *rapid.T) {
    name := rapid.StringMatching(`[A-Za-z]{1,20}`).Draw(rt, "name")
    maxHP := rapid.Int32Range(1, 1000).Draw(rt, "maxHP")
    currentHP := rapid.Int32Range(0, maxHP).Draw(rt, "currentHP")
    got := handlers.BuildPrompt(name, currentHP, maxHP, nil)
    if !strings.HasSuffix(got, "> ") {
        rt.Errorf("BuildPrompt must end with '> ', got %q", got)
    }
    if !strings.Contains(got, name) {
        rt.Errorf("BuildPrompt must contain name %q, got %q", name, got)
    }
})
```

`TestBuildPrompt_NoConditions_FormatUnchanged` (line 300): Update call and fix bracket count — prompt now has 2 groups `[Name]` and `[HP/MaxHPhp]`, not 3:
```go
func TestBuildPrompt_NoConditions_FormatUnchanged(t *testing.T) {
    got := handlers.BuildPrompt("Thorald", 50, 60, nil)
    if !strings.HasSuffix(got, "> ") {
        t.Errorf("prompt must end with '> ', got %q", got)
    }
    stripped := telnet.StripANSI(got)
    // Should have exactly 2 bracket groups: [Name] and [HP/MaxHPhp]
    if strings.Count(stripped, "[") != 2 {
        t.Errorf("expected exactly 2 bracket groups (no conditions, no time), got %q", stripped)
    }
}
```

`TestBuildPrompt_OneCondition` (line 313):
```go
got := handlers.BuildPrompt("Thorald", 50, 60, []string{"Panicked"})
```

`TestBuildPrompt_MultipleConditions` (line 323):
```go
got := handlers.BuildPrompt("Thorald", 50, 60, []string{"Panicked", "Grabbed"})
```

`TestProperty_BuildPrompt_ConditionsAllPresent` (line 338): Remove `period` draw and usage:
```go
got := handlers.BuildPrompt(name, curHP, maxHP, uniqueConds)
```

`TestBuildPrompt_ConditionAppliedThenRemoved` (line 372): The inner `buildPrompt` lambda has two `handlers.BuildPrompt(...)` calls — update both:
```go
return handlers.BuildPrompt("Hero", 10, 10, names)
```

- [ ] **Step 5: Run all frontend tests**

```bash
go test ./internal/frontend/... 2>&1
```

Expected: `ok`

- [ ] **Step 6: Run full required test suite**

```bash
go test ./internal/gameserver/... ./internal/storage/postgres/... ./internal/frontend/... 2>&1
```

Expected: all `ok`

- [ ] **Step 7: Commit**

```bash
git add internal/frontend/handlers/game_bridge.go internal/frontend/handlers/game_bridge_test.go
git commit -m "feat: remove time from prompt, pass GameDateTime to RenderRoomView"
```

---

## Task 6: Final verification + feature doc update

**Files:**
- Modify: `docs/features/persistent-calendar.md`
- Modify: `docs/features/index.yaml`

- [ ] **Step 1: Run the full required test command**

```bash
go test ./internal/gameserver/... ./internal/storage/postgres/... ./internal/frontend/... 2>&1
```

Expected: all `ok`, zero failures.

- [ ] **Step 2: Build all server binaries**

```bash
go build ./cmd/... 2>&1
```

Expected: no errors.

- [ ] **Step 3: Mark feature complete**

In `docs/features/persistent-calendar.md`, mark all `[ ]` items as `[x]`.

In `docs/features/index.yaml`, change the `persistent-calendar` entry:
```yaml
status: done
effort: "-"
```

- [ ] **Step 4: Commit**

```bash
git add docs/features/persistent-calendar.md docs/features/index.yaml
git commit -m "docs: mark persistent-calendar feature done"
```
