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
	mu          sync.Mutex
	active      *postgres.ActiveWeatherEvent
	cooldownEnd int64
	hasCooldown bool
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
		// Force an active event by directly inserting into stub
		_ = repo.StartEvent(context.Background(), "rain", 999)
		// Load state so wm knows about it
		_ = wm.LoadState(context.Background())
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
		_ = wm.LoadState(context.Background())
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
