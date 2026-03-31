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
	Tick  int64    // monotonically-increasing game-hour counter; 0 on first boot
}

// CalendarRepo persists the in-game day, month, hour, and tick across server restarts.
//
// Precondition for Load: table may be empty (first boot).
// Postcondition for Load: returns (6, 1, 1, 0, nil) when no row exists (hour defaults to 6).
// Postcondition for Save: upserts the single calendar row (id=1).
type CalendarRepo interface {
	Load() (hour, day, month int, tick int64, err error)
	Save(hour, day, month int, tick int64) error
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
// Thread-safe: subscriber map and day/month/tick state are protected by mu.
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

// SetLogger attaches a warn-level logger for Save failures. Optional.
func (c *GameCalendar) SetLogger(l interface{ Warnw(string, ...interface{}) }) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logger = l
}

// CurrentDateTime returns the current in-game date and time.
func (c *GameCalendar) CurrentDateTime() GameDateTime {
	c.mu.Lock()
	defer c.mu.Unlock()
	return GameDateTime{Hour: c.clock.CurrentHour(), Day: c.day, Month: c.month, Tick: c.tick}
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
				// Advance day/month at midnight, increment tick, then save and broadcast.
				// Mutex is held only while mutating state and copying subscribers;
				// Save() is called outside the lock to avoid holding it during I/O.
				c.mu.Lock()
				if h == 0 {
					// Use year 2001 (non-leap) so February always has 28 days in this game calendar.
					next := time.Date(2001, time.Month(c.month), c.day+1, 0, 0, 0, 0, time.UTC)
					c.day, c.month = next.Day(), int(next.Month())
				}
				c.tick++
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
	return func() {
		once.Do(func() { close(done) })
	}
}
