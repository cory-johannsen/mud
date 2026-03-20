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
	logger      interface{ Warnw(string, ...interface{}) }
	subscribers map[chan<- GameDateTime]struct{}
}

// NewGameCalendar creates a GameCalendar starting at the given day and month.
//
// Precondition: clock != nil; day in [1,31]; month in [1,12]; repo != nil.
// Postcondition: Returns a non-nil *GameCalendar ready to Start().
func NewGameCalendar(clock *GameClock, day, month int, repo CalendarRepo) *GameCalendar {
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
	return &GameCalendar{
		clock:       clock,
		day:         day,
		month:       month,
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
				// For the midnight path: mutex is released before Save() to avoid holding the lock
				// during I/O, then re-acquired for the broadcast. Both paths hold the mutex
				// when constructing dt and copying subscribers.
				c.mu.Lock()
				// Rollover at midnight: advance day/month BEFORE broadcast
				// so subscribers receive the new day at hour 0, not the old day.
				if h == 0 {
					// Use year 2001 (non-leap) so February always has 28 days in this game calendar.
					next := time.Date(2001, time.Month(c.month), c.day+1, 0, 0, 0, 0, time.UTC)
					c.day, c.month = next.Day(), int(next.Month())
					day, month := c.day, c.month
					repo := c.repo
					logger := c.logger
					c.mu.Unlock()
					if err := repo.Save(day, month); err != nil && logger != nil {
						logger.Warnw("GameCalendar: failed to save day/month", "error", err)
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
