# Seasonal Weather Effects Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement season-weighted random weather events that apply conditions to all outdoor players simultaneously, with on-screen indicators for both the telnet and web clients, persisted across server restarts.

**Architecture:** A `WeatherManager` subscribes to `GameCalendar` ticks, rolls for new events based on season weights, persists the active event and cooldown to a `weather_events` DB table, and exposes `ActiveEffects(indoor bool)` to the existing room-effect application hooks. A new `WeatherEvent` proto message broadcasts state changes to all clients. The telnet client shows a centered banner in the room region; the web client shows a toolbar badge.

**Tech Stack:** Go, PostgreSQL, pgx/v5, protobuf/buf, React 18/TypeScript, Vite

---

## File Map

| Action | Path |
|--------|------|
| Modify | `internal/gameserver/calendar.go` |
| Modify | `internal/storage/postgres/calendar.go` |
| Create | `migrations/056_calendar_tick.up.sql` |
| Create | `migrations/056_calendar_tick.down.sql` |
| Modify | `internal/config/config.go` |
| Modify | `configs/dev.yaml` |
| Create | `content/weather.yaml` |
| Create | `internal/gameserver/weather_loader.go` |
| Modify | `internal/game/world/model.go` |
| Modify | `internal/game/world/loader.go` |
| Create | `migrations/057_weather_events.up.sql` |
| Create | `migrations/057_weather_events.down.sql` |
| Create | `internal/storage/postgres/weather_repo.go` |
| Create | `internal/gameserver/weather_manager.go` |
| Create | `internal/gameserver/weather_manager_test.go` |
| Modify | `api/proto/game/v1/game.proto` |
| Modify | `internal/gameserver/grpc_service.go` |
| Modify | `internal/gameserver/combat_handler.go` |
| Modify | `cmd/gameserver/wire.go` |
| Modify | `internal/frontend/handlers/game_bridge.go` |
| Modify | `internal/frontend/handlers/text_renderer.go` |
| Modify | `cmd/webclient/ui/src/game/GameContext.tsx` |
| Modify | `cmd/webclient/ui/src/pages/GamePage.tsx` |

---

### Task 1: Calendar Tick Counter

**Files:**
- Modify: `internal/gameserver/calendar.go`
- Modify: `internal/storage/postgres/calendar.go`
- Create: `migrations/056_calendar_tick.up.sql`
- Create: `migrations/056_calendar_tick.down.sql`

- [ ] **Step 1: Write migration files**

`migrations/056_calendar_tick.up.sql`:
```sql
ALTER TABLE world_calendar ADD COLUMN IF NOT EXISTS tick BIGINT NOT NULL DEFAULT 0;
```

`migrations/056_calendar_tick.down.sql`:
```sql
ALTER TABLE world_calendar DROP COLUMN IF EXISTS tick;
```

- [ ] **Step 2: Update `GameDateTime` to include `Tick`**

In `internal/gameserver/calendar.go`, replace the `GameDateTime` struct:

```go
// GameDateTime is the combined in-game date and time broadcast on every clock tick.
type GameDateTime struct {
	Hour  GameHour // 0–23
	Day   int      // 1–28/29/30/31
	Month int      // 1–12
	Tick  int64    // monotonically-increasing game-hour counter; 0 on first boot
}
```

- [ ] **Step 3: Add `tick` field to `GameCalendar` and update constructor**

In `internal/gameserver/calendar.go`, add `tick int64` to the struct and constructor:

```go
type GameCalendar struct {
	clock       *GameClock
	mu          sync.Mutex
	day         int
	month       int
	tick        int64
	repo        CalendarRepo
	logger      interface{ Warnw(string, ...interface{}) }
	subscribers map[chan<- GameDateTime]struct{}
}

// NewGameCalendar creates a GameCalendar starting at the given day, month, and tick.
//
// Precondition: clock != nil; day in [1,31]; month in [1,12]; tick >= 0; repo != nil.
// Postcondition: Returns a non-nil *GameCalendar ready to Start().
func NewGameCalendar(clock *GameClock, day, month int, tick int64, repo CalendarRepo) *GameCalendar {
	if clock == nil {
		panic("NewGameCalendar: clock must not be nil")
	}
	if repo == nil {
		panic("NewGameCalendar: repo must not be nil")
	}
	if day < 1 || day > 31 {
		panic(fmt.Sprintf("NewGameCalendar: day %d out of range [1,31]", day))
	}
	if month < 1 || month > 12 {
		panic(fmt.Sprintf("NewGameCalendar: month %d out of range [1,12]", month))
	}
	if tick < 0 {
		panic(fmt.Sprintf("NewGameCalendar: tick %d must be >= 0", tick))
	}
	return &GameCalendar{
		clock:       clock,
		day:         day,
		month:       month,
		tick:        tick,
		repo:        repo,
		subscribers: make(map[chan<- GameDateTime]struct{}),
	}
}
```

- [ ] **Step 4: Increment tick in `Start()` goroutine and include in broadcasts**

Replace the goroutine body in `Start()` to increment tick and include it in `GameDateTime`:

```go
go func() {
	for {
		select {
		case h, ok := <-clockCh:
			if !ok {
				return
			}
			c.mu.Lock()
			c.tick++
			if h == 0 {
				next := time.Date(2001, time.Month(c.month), c.day+1, 0, 0, 0, 0, time.UTC)
				c.day, c.month = next.Day(), int(next.Month())
			}
			hour, day, month, tick := int(h), c.day, c.month, c.tick
			repo := c.repo
			logger := c.logger
			dt := GameDateTime{Hour: h, Day: c.day, Month: c.month, Tick: c.tick}
			subs := make([]chan<- GameDateTime, 0, len(c.subscribers))
			for ch := range c.subscribers {
				subs = append(subs, ch)
			}
			c.mu.Unlock()
			if err := repo.Save(hour, day, month, tick); err != nil && logger != nil {
				logger.Warnw("GameCalendar: failed to save calendar state", "error", err)
			}
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
```

Also update `CurrentDateTime()`:
```go
func (c *GameCalendar) CurrentDateTime() GameDateTime {
	c.mu.Lock()
	defer c.mu.Unlock()
	return GameDateTime{Hour: c.clock.CurrentHour(), Day: c.day, Month: c.month, Tick: c.tick}
}
```

- [ ] **Step 5: Update `CalendarRepo` interface and postgres implementation**

In `internal/gameserver/calendar.go`, update the interface:
```go
// CalendarRepo persists the in-game day, month, hour, and tick across server restarts.
//
// Precondition for Load: table may be empty (first boot).
// Postcondition for Load: returns (6, 1, 1, 0, nil) when no row exists.
// Postcondition for Save: upserts the single calendar row (id=1).
type CalendarRepo interface {
	Load() (hour, day, month int, tick int64, err error)
	Save(hour, day, month int, tick int64) error
}
```

Replace `internal/storage/postgres/calendar.go` entirely:
```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CalendarRepo persists the in-game hour, day, month, and tick to the world_calendar table.
//
// Precondition: db != nil and migration 056 has been applied.
type CalendarRepo struct {
	db *pgxpool.Pool
}

// NewCalendarRepo creates a CalendarRepo backed by db.
func NewCalendarRepo(db *pgxpool.Pool) *CalendarRepo {
	return &CalendarRepo{db: db}
}

// Load returns the persisted hour, day, month, and tick.
// Returns (6, 1, 1, 0, nil) when no row exists (first boot).
func (r *CalendarRepo) Load() (hour, day, month int, tick int64, err error) {
	row := r.db.QueryRow(context.Background(),
		`SELECT hour, day, month, tick FROM world_calendar WHERE id = 1`)
	if scanErr := row.Scan(&hour, &day, &month, &tick); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return 6, 1, 1, 0, nil
		}
		return 0, 0, 0, 0, fmt.Errorf("calendar load: %w", scanErr)
	}
	return hour, day, month, tick, nil
}

// Save upserts the single calendar row (id=1).
//
// Precondition: hour in [0,23], day in [1,31], month in [1,12], tick >= 0.
func (r *CalendarRepo) Save(hour, day, month int, tick int64) error {
	if hour < 0 || hour > 23 {
		return fmt.Errorf("calendar save: hour %d out of range [0, 23]", hour)
	}
	if day < 1 || day > 31 {
		return fmt.Errorf("calendar save: day %d out of range [1,31]", day)
	}
	if month < 1 || month > 12 {
		return fmt.Errorf("calendar save: month %d out of range [1,12]", month)
	}
	if tick < 0 {
		return fmt.Errorf("calendar save: tick %d must be >= 0", tick)
	}
	_, err := r.db.Exec(context.Background(), `
		INSERT INTO world_calendar (id, hour, day, month, tick)
		VALUES (1, $1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE
		  SET hour = EXCLUDED.hour,
		      day  = EXCLUDED.day,
		      month = EXCLUDED.month,
		      tick = EXCLUDED.tick
	`, hour, day, month, tick)
	if err != nil {
		return fmt.Errorf("calendar save: %w", err)
	}
	return nil
}
```

- [ ] **Step 6: Find all callers of `NewGameCalendar` and update them**

```bash
grep -rn "NewGameCalendar" /home/cjohannsen/src/mud --include="*.go"
```

For each call site, the caller must now pass `tick int64` loaded from `repo.Load()`. The fourth return value of the updated `Load()` is the tick. Update accordingly.

- [ ] **Step 7: Build to verify no compile errors**

```bash
cd /home/cjohannsen/src/mud && mise run go build ./...
```

Expected: exits 0.

- [ ] **Step 8: Run existing calendar tests**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./internal/gameserver/... -run Calendar -v
```

Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add migrations/056_calendar_tick.up.sql migrations/056_calendar_tick.down.sql \
  internal/gameserver/calendar.go internal/storage/postgres/calendar.go
git commit -m "feat: add monotonic tick counter to GameCalendar (weather prerequisite)"
```

---

### Task 2: WeatherConfig, content/weather.yaml, and WeatherLoader

**Files:**
- Modify: `internal/config/config.go`
- Modify: `configs/dev.yaml`
- Create: `content/weather.yaml`
- Create: `internal/gameserver/weather_loader.go`

- [ ] **Step 1: Add `WeatherConfig` to `internal/config/config.go`**

After the `WebConfig` struct, add:
```go
// WeatherConfig holds weather engine settings.
type WeatherConfig struct {
	// ChancePerTick is the probability (0–1) of a new weather event starting each game-hour tick.
	// Default: 0.05 (5% per hour).
	ChancePerTick float64 `mapstructure:"chance_per_tick"`
	// ContentFile is the path to content/weather.yaml.
	ContentFile string `mapstructure:"content_file"`
}
```

Add `Weather WeatherConfig` to the `Config` struct:
```go
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	Database   DatabaseConfig   `mapstructure:"database"`
	Telnet     TelnetConfig     `mapstructure:"telnet"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	GameServer GameServerConfig `mapstructure:"gameserver"`
	Web        WebConfig        `mapstructure:"web"`
	Weather    WeatherConfig    `mapstructure:"weather"`
}
```

In `setDefaults()` (find the function in config.go and add):
```go
viper.SetDefault("weather.chance_per_tick", 0.05)
viper.SetDefault("weather.content_file", "content/weather.yaml")
```

- [ ] **Step 2: Add weather section to `configs/dev.yaml`**

Append to `configs/dev.yaml`:
```yaml
weather:
  chance_per_tick: 0.05
  content_file: content/weather.yaml
```

- [ ] **Step 3: Create `content/weather.yaml`**

```yaml
# Weather event type definitions.
# seasons: spring (3-5), summer (6-8), fall (9-11), winter (12,1,2)
# weight: relative probability within eligible seasons (higher = more frequent)
# conditions: condition IDs applied to outdoor players while event is active
types:
  - id: rain
    name: Rain
    announce: "Rain begins to fall across Portland."
    end_announce: "The rain has stopped."
    seasons: [spring, fall, winter]
    weight: 10
    conditions: [reduced_visibility]

  - id: heavy_rain
    name: Heavy Rain
    announce: "Heavy rain lashes the streets of Portland."
    end_announce: "The heavy rain has eased."
    seasons: [spring, fall, winter]
    weight: 6
    conditions: [reduced_visibility, terrain_flooded]

  - id: fog
    name: Dense Fog
    announce: "A thick fog rolls in across Portland."
    end_announce: "The fog has lifted."
    seasons: [spring, fall, winter]
    weight: 7
    conditions: [reduced_visibility]

  - id: thunderstorm
    name: Thunderstorm
    announce: "A violent thunderstorm sweeps across Portland!"
    end_announce: "The thunderstorm has passed."
    seasons: [spring, summer, fall]
    weight: 5
    conditions: [reduced_visibility, terrain_flooded]

  - id: blizzard
    name: Blizzard
    announce: "A blizzard is blanketing Portland in snow and ice!"
    end_announce: "The blizzard has passed."
    seasons: [winter]
    weight: 3
    conditions: [reduced_visibility, terrain_ice, terrain_mud]

  - id: ice_storm
    name: Ice Storm
    announce: "An ice storm coats every surface in Portland with a treacherous glaze."
    end_announce: "The ice storm has ended."
    seasons: [winter]
    weight: 4
    conditions: [terrain_ice, reduced_visibility]

  - id: sleet
    name: Sleet
    announce: "Sleet hammers down across Portland."
    end_announce: "The sleet has stopped."
    seasons: [winter, spring]
    weight: 5
    conditions: [terrain_ice, reduced_visibility]

  - id: hailstorm
    name: Hailstorm
    announce: "Hailstones the size of marbles pelt down across Portland!"
    end_announce: "The hailstorm has ended."
    seasons: [spring, summer, fall]
    weight: 3
    conditions: [reduced_visibility]

  - id: windstorm
    name: Windstorm
    announce: "Gale-force winds tear through Portland's streets."
    end_announce: "The windstorm has died down."
    seasons: [spring, fall]
    weight: 4
    conditions: [reduced_visibility]

  - id: extreme_heat
    name: Extreme Heat
    announce: "An oppressive heat wave descends on Portland."
    end_announce: "The heat wave has broken."
    seasons: [summer]
    weight: 5
    conditions: [dazzled]
```

- [ ] **Step 4: Create `internal/gameserver/weather_loader.go`**

```go
package gameserver

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// WeatherType defines a single weather event type loaded from content/weather.yaml.
type WeatherType struct {
	ID          string   `yaml:"id"`
	Name        string   `yaml:"name"`
	Announce    string   `yaml:"announce"`
	EndAnnounce string   `yaml:"end_announce"`
	Seasons     []string `yaml:"seasons"`
	Weight      int      `yaml:"weight"`
	Conditions  []string `yaml:"conditions"`
}

type weatherFile struct {
	Types []WeatherType `yaml:"types"`
}

// LoadWeatherTypes reads and parses the weather type definitions from path.
//
// Precondition: path points to a valid weather YAML file.
// Postcondition: Returns a non-empty slice of WeatherType on success.
func LoadWeatherTypes(path string) ([]WeatherType, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("weather loader: read %q: %w", path, err)
	}
	var wf weatherFile
	if err := yaml.Unmarshal(data, &wf); err != nil {
		return nil, fmt.Errorf("weather loader: parse %q: %w", path, err)
	}
	if len(wf.Types) == 0 {
		return nil, fmt.Errorf("weather loader: %q defines no weather types", path)
	}
	return wf.Types, nil
}

// SeasonForMonth returns the season name for a calendar month (1–12).
//
// Precondition: month in [1,12].
// Postcondition: returns one of "spring", "summer", "fall", "winter".
func SeasonForMonth(month int) string {
	switch month {
	case 3, 4, 5:
		return "spring"
	case 6, 7, 8:
		return "summer"
	case 9, 10, 11:
		return "fall"
	default: // 12, 1, 2
		return "winter"
	}
}
```

- [ ] **Step 5: Write tests for `SeasonForMonth` and `LoadWeatherTypes`**

Create `internal/gameserver/weather_loader_test.go`:
```go
package gameserver_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/cory-johannsen/mud/internal/gameserver"
)

func TestSeasonForMonth(t *testing.T) {
	cases := []struct {
		month  int
		season string
	}{
		{1, "winter"}, {2, "winter"}, {12, "winter"},
		{3, "spring"}, {4, "spring"}, {5, "spring"},
		{6, "summer"}, {7, "summer"}, {8, "summer"},
		{9, "fall"}, {10, "fall"}, {11, "fall"},
	}
	for _, tc := range cases {
		got := gameserver.SeasonForMonth(tc.month)
		if got != tc.season {
			t.Errorf("SeasonForMonth(%d) = %q, want %q", tc.month, got, tc.season)
		}
	}
}

func TestLoadWeatherTypes_Valid(t *testing.T) {
	content := `
types:
  - id: rain
    name: Rain
    announce: "It rains."
    end_announce: "Rain stopped."
    seasons: [spring, fall]
    weight: 5
    conditions: [reduced_visibility]
`
	path := filepath.Join(t.TempDir(), "weather.yaml")
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	types, err := gameserver.LoadWeatherTypes(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(types) != 1 {
		t.Fatalf("expected 1 type, got %d", len(types))
	}
	if types[0].ID != "rain" {
		t.Errorf("expected id=rain, got %q", types[0].ID)
	}
}

func TestLoadWeatherTypes_Empty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "weather.yaml")
	if err := os.WriteFile(path, []byte("types: []\n"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := gameserver.LoadWeatherTypes(path)
	if err == nil {
		t.Fatal("expected error for empty types, got nil")
	}
}
```

- [ ] **Step 6: Run tests**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./internal/gameserver/... -run "TestSeasonForMonth|TestLoadWeatherTypes" -v
```

Expected: all pass.

- [ ] **Step 7: Commit**

```bash
git add internal/config/config.go configs/dev.yaml content/weather.yaml \
  internal/gameserver/weather_loader.go internal/gameserver/weather_loader_test.go
git commit -m "feat: add WeatherConfig, weather type content, and SeasonForMonth"
```

---

### Task 3: Room Indoor Flag

**Files:**
- Modify: `internal/game/world/model.go`
- Modify: `internal/game/world/loader.go`

- [ ] **Step 1: Add `Indoor` field to `Room` struct**

In `internal/game/world/model.go`, add `Indoor bool` to the `Room` struct after `BossRoom`:
```go
type Room struct {
	ID          string
	ZoneID      string
	Title       string
	Description string
	Exits       []Exit
	Properties  map[string]string
	Spawns      []RoomSpawnConfig
	Equipment   []RoomEquipmentConfig
	MapX, MapY  int
	SkillChecks []skillcheck.TriggerDef
	Effects     []RoomEffect
	Terrain     string
	DangerLevel string
	RoomTrapChance, CoverTrapChance *int
	Traps       []RoomTrapConfig
	BossRoom    bool
	Indoor      bool // true = protected from weather effects
	Hazards     []HazardDef
	MinFactionTierID string
}
```

- [ ] **Step 2: Parse `indoor` field in the YAML loader**

In `internal/game/world/loader.go`, find the `yamlRoom` struct and add:
```go
Indoor bool `yaml:"indoor"`
```

Find where `yamlRoom` fields are mapped to `Room` fields (look for where `BossRoom` is assigned) and add:
```go
room.Indoor = yr.Indoor
```

- [ ] **Step 3: Build to verify**

```bash
cd /home/cjohannsen/src/mud && mise run go build ./...
```

Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add internal/game/world/model.go internal/game/world/loader.go
git commit -m "feat: add indoor flag to Room for weather effect shielding"
```

---

### Task 4: DB Migration and WeatherRepo

**Files:**
- Create: `migrations/057_weather_events.up.sql`
- Create: `migrations/057_weather_events.down.sql`
- Create: `internal/storage/postgres/weather_repo.go`

- [ ] **Step 1: Create migration files**

`migrations/057_weather_events.up.sql`:
```sql
CREATE TABLE IF NOT EXISTS weather_events (
    id               SERIAL PRIMARY KEY,
    weather_type     TEXT   NOT NULL,
    end_tick         BIGINT NOT NULL,
    cooldown_end_tick BIGINT NOT NULL DEFAULT 0,
    active           BOOL   NOT NULL DEFAULT TRUE
);
```

`migrations/057_weather_events.down.sql`:
```sql
DROP TABLE IF EXISTS weather_events;
```

- [ ] **Step 2: Write failing tests for WeatherRepo**

Create `internal/storage/postgres/weather_repo_test.go`:
```go
package postgres_test

import (
	"context"
	"testing"

	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

func TestWeatherRepo_RoundTrip(t *testing.T) {
	db := testDB(t) // uses existing test helper
	repo := postgres.NewWeatherRepo(db)
	ctx := context.Background()

	// Initially no active event
	ev, err := repo.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if ev != nil {
		t.Fatalf("expected nil active event, got %+v", ev)
	}

	// Start an event
	if err := repo.StartEvent(ctx, "blizzard", 500); err != nil {
		t.Fatalf("StartEvent: %v", err)
	}

	// Retrieve active event
	ev, err = repo.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive: %v", err)
	}
	if ev == nil {
		t.Fatal("expected active event, got nil")
	}
	if ev.WeatherType != "blizzard" || ev.EndTick != 500 {
		t.Errorf("unexpected event: %+v", ev)
	}

	// End the event (with cooldown)
	if err := repo.EndEvent(ctx, 600); err != nil {
		t.Fatalf("EndEvent: %v", err)
	}

	// No active event after ending
	ev, err = repo.GetActive(ctx)
	if err != nil {
		t.Fatalf("GetActive after end: %v", err)
	}
	if ev != nil {
		t.Fatalf("expected nil after end, got %+v", ev)
	}

	// Cooldown is set
	endTick, found, err := repo.GetCooldownEnd(ctx)
	if err != nil {
		t.Fatalf("GetCooldownEnd: %v", err)
	}
	if !found {
		t.Fatal("expected cooldown to be found")
	}
	if endTick != 600 {
		t.Errorf("expected cooldown endTick=600, got %d", endTick)
	}

	// Clear expired
	if err := repo.ClearExpired(ctx); err != nil {
		t.Fatalf("ClearExpired: %v", err)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./internal/storage/postgres/... -run TestWeatherRepo -v
```

Expected: FAIL — `NewWeatherRepo` not defined yet.

- [ ] **Step 4: Implement `internal/storage/postgres/weather_repo.go`**

```go
package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ActiveWeatherEvent holds the state of a currently active weather event.
type ActiveWeatherEvent struct {
	WeatherType string
	EndTick     int64
}

// WeatherRepo is the interface for persisting weather event state.
//
// Precondition: migration 057 has been applied.
type WeatherRepo interface {
	GetActive(ctx context.Context) (*ActiveWeatherEvent, error)
	GetCooldownEnd(ctx context.Context) (endTick int64, found bool, err error)
	StartEvent(ctx context.Context, weatherType string, endTick int64) error
	EndEvent(ctx context.Context, cooldownEndTick int64) error
	ClearExpired(ctx context.Context) error
}

// PostgresWeatherRepo implements WeatherRepo backed by PostgreSQL.
type PostgresWeatherRepo struct {
	db *pgxpool.Pool
}

// NewWeatherRepo creates a WeatherRepo backed by db.
//
// Precondition: db != nil and migration 057 has been applied.
func NewWeatherRepo(db *pgxpool.Pool) *PostgresWeatherRepo {
	return &PostgresWeatherRepo{db: db}
}

// GetActive returns the currently active weather event, or nil if none is active.
func (r *PostgresWeatherRepo) GetActive(ctx context.Context) (*ActiveWeatherEvent, error) {
	row := r.db.QueryRow(ctx,
		`SELECT weather_type, end_tick FROM weather_events WHERE active = TRUE LIMIT 1`)
	var ev ActiveWeatherEvent
	if err := row.Scan(&ev.WeatherType, &ev.EndTick); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("weather repo GetActive: %w", err)
	}
	return &ev, nil
}

// GetCooldownEnd returns the cooldown end tick from the most recent ended event.
// Returns found=false if no cooldown row exists.
func (r *PostgresWeatherRepo) GetCooldownEnd(ctx context.Context) (int64, bool, error) {
	row := r.db.QueryRow(ctx,
		`SELECT cooldown_end_tick FROM weather_events WHERE active = FALSE ORDER BY id DESC LIMIT 1`)
	var endTick int64
	if err := row.Scan(&endTick); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("weather repo GetCooldownEnd: %w", err)
	}
	return endTick, true, nil
}

// StartEvent inserts a new active weather event row.
//
// Precondition: no other active event exists.
func (r *PostgresWeatherRepo) StartEvent(ctx context.Context, weatherType string, endTick int64) error {
	_, err := r.db.Exec(ctx,
		`INSERT INTO weather_events (weather_type, end_tick, cooldown_end_tick, active)
		 VALUES ($1, $2, 0, TRUE)`,
		weatherType, endTick)
	if err != nil {
		return fmt.Errorf("weather repo StartEvent: %w", err)
	}
	return nil
}

// EndEvent marks the active event as inactive and sets the cooldown end tick.
func (r *PostgresWeatherRepo) EndEvent(ctx context.Context, cooldownEndTick int64) error {
	_, err := r.db.Exec(ctx,
		`UPDATE weather_events SET active = FALSE, cooldown_end_tick = $1 WHERE active = TRUE`,
		cooldownEndTick)
	if err != nil {
		return fmt.Errorf("weather repo EndEvent: %w", err)
	}
	return nil
}

// ClearExpired deletes all inactive (ended) weather event rows.
func (r *PostgresWeatherRepo) ClearExpired(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `DELETE FROM weather_events WHERE active = FALSE`)
	if err != nil {
		return fmt.Errorf("weather repo ClearExpired: %w", err)
	}
	return nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./internal/storage/postgres/... -run TestWeatherRepo -v
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add migrations/057_weather_events.up.sql migrations/057_weather_events.down.sql \
  internal/storage/postgres/weather_repo.go internal/storage/postgres/weather_repo_test.go
git commit -m "feat: weather_events DB migration and WeatherRepo"
```

---

### Task 5: WeatherManager

**Files:**
- Create: `internal/gameserver/weather_manager.go`
- Create: `internal/gameserver/weather_manager_test.go`

- [ ] **Step 1: Write property-based tests first**

Create `internal/gameserver/weather_manager_test.go`:
```go
package gameserver_test

import (
	"context"
	"sync"
	"testing"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/internal/gameserver"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// stubWeatherRepo is an in-memory WeatherRepo for testing.
type stubWeatherRepo struct {
	mu           sync.Mutex
	active       *postgres.ActiveWeatherEvent
	cooldownEnd  int64
	hasCooldown  bool
}

func (s *stubWeatherRepo) GetActive(_ context.Context) (*postgres.ActiveWeatherEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active, nil
}
func (s *stubWeatherRepo) GetCooldownEnd(_ context.Context) (int64, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cooldownEnd, s.hasCooldown, nil
}
func (s *stubWeatherRepo) StartEvent(_ context.Context, wt string, endTick int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = &postgres.ActiveWeatherEvent{WeatherType: wt, EndTick: endTick}
	return nil
}
func (s *stubWeatherRepo) EndEvent(_ context.Context, cooldown int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.active = nil
	s.cooldownEnd = cooldown
	s.hasCooldown = true
	return nil
}
func (s *stubWeatherRepo) ClearExpired(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.hasCooldown = false
	return nil
}

var testWeatherTypes = []gameserver.WeatherType{
	{ID: "rain", Name: "Rain", Announce: "It rains.", EndAnnounce: "Stopped.", Seasons: []string{"spring"}, Weight: 1, Conditions: []string{"reduced_visibility"}},
}

func newTestWeatherManager(repo gameserver.WeatherRepo) *gameserver.WeatherManager {
	return gameserver.NewWeatherManager(repo, testWeatherTypes, 1.0, nil) // 100% roll rate for tests
}

func TestProperty_WeatherManager_NeverFiresDuringCooldown(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		repo := &stubWeatherRepo{hasCooldown: true, cooldownEnd: 1000}
		wm := newTestWeatherManager(repo)
		// Tick at time before cooldown expires
		wm.OnTick(gameserver.GameDateTime{Month: 4, Day: 1, Hour: 6, Tick: 500})
		// No event should have started
		ev, _ := repo.GetActive(context.Background())
		if ev != nil {
			t.Fatalf("event started during cooldown: %+v", ev)
		}
	})
}

func TestProperty_WeatherManager_ActiveEffectsEmptyForIndoor(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		repo := &stubWeatherRepo{}
		wm := newTestWeatherManager(repo)
		// Force an active event
		_ = repo.StartEvent(context.Background(), "rain", 999)
		wm.OnTick(gameserver.GameDateTime{Month: 4, Day: 1, Hour: 6, Tick: 1})
		effects := wm.ActiveEffects(true) // indoor
		if len(effects) != 0 {
			t.Fatalf("indoor room got weather effects: %v", effects)
		}
	})
}

func TestProperty_WeatherManager_EventEndsAtOrAfterEndTick(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		endTick := rapid.Int64Range(10, 200).Draw(t, "endTick")
		repo := &stubWeatherRepo{}
		wm := newTestWeatherManager(repo)
		_ = repo.StartEvent(context.Background(), "rain", endTick)
		wm.LoadState(context.Background())
		// Tick at endTick - 1: event still active
		wm.OnTick(gameserver.GameDateTime{Month: 4, Tick: endTick - 1})
		ev, _ := repo.GetActive(context.Background())
		if ev == nil {
			t.Fatalf("event ended before endTick (endTick=%d, tick=%d)", endTick, endTick-1)
		}
		// Tick at endTick: event ends
		wm.OnTick(gameserver.GameDateTime{Month: 4, Tick: endTick})
		ev, _ = repo.GetActive(context.Background())
		if ev != nil {
			t.Fatalf("event still active at endTick=%d", endTick)
		}
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./internal/gameserver/... -run "TestProperty_WeatherManager" -v
```

Expected: FAIL — `NewWeatherManager` not defined.

- [ ] **Step 3: Implement `internal/gameserver/weather_manager.go`**

```go
package gameserver

import (
	"context"
	"math/rand"
	"sync"

	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// WeatherBroadcaster sends a ServerEvent to all connected player sessions.
type WeatherBroadcaster interface {
	BroadcastAll(ev *gamev1.ServerEvent)
}

// WeatherRepo is the persistence interface for weather events.
// Re-exported alias so callers in other packages can use the interface without importing postgres.
type WeatherRepo = postgres.WeatherRepo

// WeatherManager subscribes to GameCalendar ticks and manages random weather events.
// It persists the active event and cooldown via WeatherRepo and broadcasts WeatherEvent
// proto messages to all connected sessions.
//
// Thread-safe.
type WeatherManager struct {
	repo          WeatherRepo
	weatherTypes  []WeatherType
	chancePerTick float64
	broadcaster   WeatherBroadcaster // may be nil (no broadcast in tests)

	mu          sync.RWMutex
	activeName  string // empty if no active event
	endTick     int64
	cooldownEnd int64 // 0 if no cooldown
}

// NewWeatherManager creates a WeatherManager.
//
// Precondition: repo != nil; weatherTypes non-empty; chancePerTick in (0, 1].
// broadcaster may be nil (used in tests).
func NewWeatherManager(repo WeatherRepo, weatherTypes []WeatherType, chancePerTick float64, broadcaster WeatherBroadcaster) *WeatherManager {
	return &WeatherManager{
		repo:          repo,
		weatherTypes:  weatherTypes,
		chancePerTick: chancePerTick,
		broadcaster:   broadcaster,
	}
}

// LoadState loads persisted event and cooldown state from the DB.
// Must be called at startup before OnTick is first invoked.
func (wm *WeatherManager) LoadState(ctx context.Context) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	ev, err := wm.repo.GetActive(ctx)
	if err != nil {
		return err
	}
	if ev != nil {
		wm.activeName = ev.WeatherType
		wm.endTick = ev.EndTick
	}

	endTick, found, err := wm.repo.GetCooldownEnd(ctx)
	if err != nil {
		return err
	}
	if found {
		wm.cooldownEnd = endTick
	}
	return nil
}

// OnTick is called on every GameCalendar tick. It manages event lifecycle:
// ending expired events, enforcing cooldowns, and rolling for new events.
func (wm *WeatherManager) OnTick(dt GameDateTime) {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	ctx := context.Background()

	// End expired active event.
	if wm.activeName != "" && dt.Tick >= wm.endTick {
		wm.endEvent(ctx, dt.Tick)
		return
	}

	// Skip if active event is ongoing.
	if wm.activeName != "" {
		return
	}

	// Skip if cooling down.
	if wm.cooldownEnd > 0 && dt.Tick < wm.cooldownEnd {
		return
	}

	// Clear expired cooldown row.
	if wm.cooldownEnd > 0 && dt.Tick >= wm.cooldownEnd {
		_ = wm.repo.ClearExpired(ctx)
		wm.cooldownEnd = 0
	}

	// Roll for a new event.
	if rand.Float64() >= wm.chancePerTick {
		return
	}

	season := SeasonForMonth(dt.Month)
	wt := wm.sampleWeatherType(season)
	if wt == nil {
		return
	}

	// Random duration: 2–168 game hours.
	durationHours := int64(2 + rand.Intn(167))
	endTick := dt.Tick + durationHours

	if err := wm.repo.StartEvent(ctx, wt.ID, endTick); err != nil {
		return
	}
	wm.activeName = wt.Name
	wm.endTick = endTick

	if wm.broadcaster != nil {
		wm.broadcaster.BroadcastAll(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Weather{
				Weather: &gamev1.WeatherEvent{
					WeatherName: wt.Name,
					Active:      true,
				},
			},
		})
	}
}

// endEvent ends the active event, sets the cooldown, and broadcasts.
// Caller must hold wm.mu.
func (wm *WeatherManager) endEvent(ctx context.Context, currentTick int64) {
	// Random cooldown: 24–72 game hours.
	cooldownHours := int64(24 + rand.Intn(49))
	cooldownEnd := currentTick + cooldownHours

	_ = wm.repo.EndEvent(ctx, cooldownEnd)

	name := wm.activeName
	wm.activeName = ""
	wm.endTick = 0
	wm.cooldownEnd = cooldownEnd

	if wm.broadcaster != nil {
		wm.broadcaster.BroadcastAll(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Weather{
				Weather: &gamev1.WeatherEvent{
					WeatherName: name,
					Active:      false,
				},
			},
		})
	}
}

// sampleWeatherType returns a season-weighted random WeatherType, or nil if none eligible.
// Caller must hold wm.mu (read is sufficient but we have write lock from OnTick).
func (wm *WeatherManager) sampleWeatherType(season string) *WeatherType {
	var eligible []WeatherType
	totalWeight := 0
	for _, wt := range wm.weatherTypes {
		for _, s := range wt.Seasons {
			if s == season {
				eligible = append(eligible, wt)
				totalWeight += wt.Weight
				break
			}
		}
	}
	if totalWeight == 0 {
		return nil
	}
	roll := rand.Intn(totalWeight)
	cumulative := 0
	for i := range eligible {
		cumulative += eligible[i].Weight
		if roll < cumulative {
			return &eligible[i]
		}
	}
	return &eligible[len(eligible)-1]
}

// ActiveEffects returns the conditions associated with the current weather event
// as RoomEffect entries, or nil if no event is active or the room is indoor.
//
// Precondition: none.
// Postcondition: returns nil for indoor rooms regardless of active event.
func (wm *WeatherManager) ActiveEffects(indoor bool) []world.RoomEffect {
	if indoor {
		return nil
	}
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	if wm.activeName == "" {
		return nil
	}
	// Find the active weather type by name.
	for _, wt := range wm.weatherTypes {
		if wt.Name == wm.activeName {
			effects := make([]world.RoomEffect, 0, len(wt.Conditions))
			for _, condID := range wt.Conditions {
				effects = append(effects, world.RoomEffect{
					Track:           condID,
					BaseDC:          12,
					CooldownMinutes: 60,
				})
			}
			return effects
		}
	}
	return nil
}

// ActiveWeatherName returns the name of the current weather event, or empty string.
func (wm *WeatherManager) ActiveWeatherName() string {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.activeName
}
```

- [ ] **Step 4: Run tests**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./internal/gameserver/... -run "TestProperty_WeatherManager" -v
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/gameserver/weather_manager.go internal/gameserver/weather_manager_test.go
git commit -m "feat: WeatherManager with seasonal event rolling and persistence"
```

---

### Task 6: Proto WeatherEvent

**Files:**
- Modify: `api/proto/game/v1/game.proto`

- [ ] **Step 1: Add `WeatherEvent` message to the proto**

In `api/proto/game/v1/game.proto`, add the new message (after the last message in the file):
```proto
// WeatherEvent broadcasts active weather state to all clients.
message WeatherEvent {
  string weather_name = 1; // display name of the weather event
  bool   active       = 2; // true = event started, false = event ended
}
```

Add field 32 to `ServerEvent.payload` oneof:
```proto
WeatherEvent weather = 32;
```

- [ ] **Step 2: Regenerate proto**

```bash
cd /home/cjohannsen/src/mud && buf generate
```

Expected: exits 0; Go and TypeScript generated files updated.

- [ ] **Step 3: Build to verify**

```bash
cd /home/cjohannsen/src/mud && mise run go build ./...
```

Expected: exits 0.

- [ ] **Step 4: Commit**

```bash
git add api/proto/game/v1/game.proto
# Add all generated proto files
git add internal/gameserver/gamev1/ cmd/webclient/ui/src/proto/
git commit -m "feat: add WeatherEvent proto message"
```

---

### Task 7: Gameserver Integration and Wire

**Files:**
- Modify: `internal/gameserver/grpc_service.go`
- Modify: `internal/gameserver/combat_handler.go`
- Modify: `cmd/gameserver/wire.go`

- [ ] **Step 1: Add `WeatherManager` to `GameServiceServer`**

In `internal/gameserver/grpc_service.go`, find the `GameServiceServer` struct definition and add:
```go
weatherMgr *WeatherManager
```

Find the `GameServiceServer` constructor (or the struct literal in a provider) and ensure `weatherMgr` is wired in. If there's a `NewGameServiceServer` function, add `weatherMgr *WeatherManager` as a parameter.

- [ ] **Step 2: Update `applyRoomEffectsOnEntry` to include weather effects**

Find `applyRoomEffectsOnEntry` in `internal/gameserver/grpc_service.go`. After the existing room effect loop setup, append weather effects before iterating:

```go
func (s *GameServiceServer) applyRoomEffectsOnEntry(
	sess *session.PlayerSession, uid string, room *world.Room, now int64,
) {
	effects := room.Effects
	if s.weatherMgr != nil {
		effects = append(effects, s.weatherMgr.ActiveEffects(room.Indoor)...)
	}
	for _, effect := range effects {
		// ... existing effect application logic (unchanged) ...
	}
}
```

**Important:** The existing loop must iterate `effects` (the combined slice), not `room.Effects` directly.

- [ ] **Step 3: Send `WeatherEvent` on session join**

In `grpc_service.go`, find the handler for `JoinWorldRequest` (or `handleJoinWorld`). After sending `RoomView` to the joining player, add:

```go
if s.weatherMgr != nil {
	if name := s.weatherMgr.ActiveWeatherName(); name != "" {
		_ = stream.Send(&gamev1.ServerEvent{
			Payload: &gamev1.ServerEvent_Weather{
				Weather: &gamev1.WeatherEvent{
					WeatherName: name,
					Active:      true,
				},
			},
		})
	}
}
```

- [ ] **Step 4: Update combat handler to include weather effects**

In `internal/gameserver/combat_handler.go`, find where `room.Effects` are applied during combat rounds (search for `room.Effects` or `applyRoomEffects`). Add weather effects to the slice:

```go
effects := room.Effects
if s.weatherMgr != nil {
	effects = append(effects, s.weatherMgr.ActiveEffects(room.Indoor)...)
}
// use effects (not room.Effects) in the loop
```

- [ ] **Step 5: Add `WeatherManager` to Wire App and subscribe to calendar**

In `cmd/gameserver/wire.go`:

1. Add `WeatherMgr *gameserver.WeatherManager` to the `App` struct.

2. In the `Initialize` wire.Build call, add the WeatherManager provider. First, add a provider function near the top of the file (before `Initialize`):

```go
// ProvideWeatherManager constructs and initializes the WeatherManager.
func ProvideWeatherManager(
	repo postgres.WeatherRepo,
	types []gameserver.WeatherType,
	cfg *AppConfig,
	cal *gameserver.GameCalendar,
	sess *session.Manager,
) (*gameserver.WeatherManager, error) {
	wm := gameserver.NewWeatherManager(repo, types, cfg.WeatherChancePerTick, sess)
	if err := wm.LoadState(context.Background()); err != nil {
		return nil, err
	}
	ch := make(chan gameserver.GameDateTime, 2)
	cal.Subscribe(ch)
	go func() {
		for dt := range ch {
			wm.OnTick(dt)
		}
	}()
	return wm, nil
}
```

3. Add `WeatherChancePerTick float64` and `WeatherFile string` to `AppConfig`, extracted from `config.WeatherConfig`.

4. Add `ProvideWeatherManager` to the `wire.Build` call.

5. Add `wire.Bind(new(gameserver.WeatherRepo), new(*postgres.PostgresWeatherRepo))` to the build.

6. Add `wire.Bind(new(gameserver.WeatherBroadcaster), new(*session.Manager))` (only if `session.Manager` implements `BroadcastAll`; otherwise add that method to `session.Manager` first).

- [ ] **Step 6: Ensure `session.Manager` implements `WeatherBroadcaster`**

Check if `session.Manager` has a `BroadcastAll(ev *gamev1.ServerEvent)` method:
```bash
grep -n "BroadcastAll\|BroadcastServerEvent" /home/cjohannsen/src/mud/internal/game/session/*.go
```

If not found, add to the session Manager (find the appropriate file and add):
```go
// BroadcastAll sends ev to every connected player session.
func (m *Manager) BroadcastAll(ev *gamev1.ServerEvent) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, sess := range m.sessions {
		if sess.Stream != nil {
			_ = sess.Stream.Send(ev)
		}
	}
}
```

Adapt field names to match the actual `Manager` struct.

- [ ] **Step 7: Add `NewWeatherRepo` to postgres StorageProviders**

In `internal/storage/postgres/providers.go` (or wherever `StorageProviders` is defined), add:
```go
NewWeatherRepo,
```

And add the interface binding to wire.go:
```go
wire.Bind(new(gameserver.WeatherRepo), new(*postgres.PostgresWeatherRepo)),
```

- [ ] **Step 8: Build and run tests**

```bash
cd /home/cjohannsen/src/mud && mise run go build ./... && mise run go test ./internal/gameserver/... -v -count=1
```

Expected: build exits 0; tests pass.

- [ ] **Step 9: Commit**

```bash
git add internal/gameserver/grpc_service.go internal/gameserver/combat_handler.go \
  cmd/gameserver/wire.go internal/game/session/
git commit -m "feat: wire WeatherManager into gameserver, apply weather effects on room entry and combat"
```

---

### Task 8: Telnet Client Weather Indicator

**Files:**
- Modify: `internal/frontend/handlers/game_bridge.go`
- Modify: `internal/frontend/handlers/text_renderer.go`

- [ ] **Step 1: Add `activeWeather` field to `gameBridge` and handle `WeatherEvent`**

In `internal/frontend/handlers/game_bridge.go`, find the struct that holds game bridge state (search for `gameBridge` or the struct containing `roomView`). Add:
```go
activeWeather string // empty when no weather is active
```

In `forwardServerEvents()`, find the `switch` on server event payload types and add a case:
```go
case *gamev1.ServerEvent_Weather:
	gb.mu.Lock()
	if p.Weather.Active {
		gb.activeWeather = p.Weather.WeatherName
	} else {
		gb.activeWeather = ""
	}
	gb.mu.Unlock()
	gb.requestRoomRedraw() // triggers a re-render of the room view
```

`requestRoomRedraw()` is whatever mechanism triggers a room view re-render (look for how `RoomView` events trigger it and replicate that pattern).

- [ ] **Step 2: Pass `activeWeather` to `RenderRoomView`**

`RenderRoomView` signature is:
```go
func RenderRoomView(rv *gamev1.RoomView, width int, maxLines int, dt gameserver.GameDateTime) string
```

Update it to:
```go
func RenderRoomView(rv *gamev1.RoomView, width int, maxLines int, dt gameserver.GameDateTime, activeWeather string) string
```

Find all call sites:
```bash
grep -rn "RenderRoomView" /home/cjohannsen/src/mud --include="*.go"
```

Update each call site to pass `activeWeather` (pass `""` at any call sites that don't have the weather state).

- [ ] **Step 3: Add weather banner as first line in `RenderRoomView`**

In `internal/frontend/handlers/text_renderer.go`, at the top of `RenderRoomView`, before the title line, add:

```go
if activeWeather != "" {
	banner := fmt.Sprintf("*** %s ***", strings.ToUpper(activeWeather))
	// Center the banner within width
	padLen := 0
	if width > len(banner) {
		padLen = (width - len(banner)) / 2
	}
	centered := strings.Repeat(" ", padLen) + banner
	lines = append(lines, telnet.Colorize(telnet.BrightCyan, centered))
}
```

- [ ] **Step 4: Build and test**

```bash
cd /home/cjohannsen/src/mud && mise run go build ./...
```

Expected: exits 0.

- [ ] **Step 5: Commit**

```bash
git add internal/frontend/handlers/game_bridge.go internal/frontend/handlers/text_renderer.go
git commit -m "feat: telnet weather banner in room view"
```

---

### Task 9: Web Client Weather Indicator

**Files:**
- Modify: `cmd/webclient/ui/src/game/GameContext.tsx`
- Modify: `cmd/webclient/ui/src/pages/GamePage.tsx`

- [ ] **Step 1: Add `activeWeather` to `GameState` and handle `WeatherEvent`**

In `cmd/webclient/ui/src/game/GameContext.tsx`, find the `GameState` interface and add:
```ts
activeWeather: string | null
```

Find the `initialState` (or default state object) and add:
```ts
activeWeather: null,
```

In `GameProvider`, find the `handleMessage` function (or wherever `ServerEvent` payloads are dispatched). Add a case for `WeatherEvent`:
```ts
case 'WeatherEvent': {
  const ev = msg.payload as WeatherEvent
  setGameState(prev => ({
    ...prev,
    activeWeather: ev.active ? ev.weatherName : null,
  }))
  break
}
```

The exact message type field depends on the generated TypeScript proto. Check `cmd/webclient/ui/src/proto/index.ts` for the `WeatherEvent` type name and field names.

- [ ] **Step 2: Add weather badge to the game toolbar**

In `cmd/webclient/ui/src/pages/GamePage.tsx`, find the toolbar/header bar at the top of the game layout. Import `useGame`:
```ts
import { useGame } from '../game/GameContext'
```

Inside the component, destructure `activeWeather`:
```ts
const { activeWeather } = useGame()
```

In the JSX, add the weather badge centered in the toolbar:
```tsx
{activeWeather && (
  <div style={{
    position: 'absolute',
    left: '50%',
    transform: 'translateX(-50%)',
    background: 'rgba(0,0,0,0.7)',
    color: '#f0a500',
    border: '1px solid #f0a500',
    borderRadius: '12px',
    padding: '2px 12px',
    fontSize: '0.8rem',
    fontFamily: 'monospace',
    fontWeight: 'bold',
    letterSpacing: '0.05em',
    whiteSpace: 'nowrap',
    pointerEvents: 'none',
  }}>
    ⛈ {activeWeather}
  </div>
)}
```

The toolbar container must have `position: 'relative'` for the absolute centering to work. Add that if not already present.

- [ ] **Step 3: Build UI**

```bash
cd /home/cjohannsen/src/mud/cmd/webclient/ui && npx tsc --noEmit && npm run build
```

Expected: exits 0, dist/ rebuilt.

- [ ] **Step 4: Commit**

```bash
git add cmd/webclient/ui/src/game/GameContext.tsx cmd/webclient/ui/src/pages/GamePage.tsx
git commit -m "feat: web client weather badge in game toolbar"
```

---

### Task 10: Run Full Test Suite and Deploy

- [ ] **Step 1: Run full Go test suite**

```bash
cd /home/cjohannsen/src/mud && mise run go test ./... -count=1 -timeout 120s
```

Expected: all tests pass, 0 failures.

- [ ] **Step 2: Build all binaries**

```bash
cd /home/cjohannsen/src/mud && make build
```

Expected: exits 0.

- [ ] **Step 3: Commit any final fixes, then deploy**

```bash
make k8s-redeploy
```

Expected: helm upgrade succeeds.
