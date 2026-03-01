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
	hour         int32
	tickInterval time.Duration
	mu           sync.Mutex
	subscribers  map[chan<- GameHour]struct{}
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
