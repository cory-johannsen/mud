package gameserver_test

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"pgregory.net/rapid"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
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

// stubBroadcaster captures all BroadcastAll calls for assertions.
type stubBroadcaster struct {
	mu     sync.Mutex
	events []*gamev1.ServerEvent
}

func (b *stubBroadcaster) BroadcastAll(ev *gamev1.ServerEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.events = append(b.events, ev)
}

func (b *stubBroadcaster) messages() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []string
	for _, ev := range b.events {
		if m := ev.GetMessage(); m != nil {
			out = append(out, m.Content)
		}
	}
	return out
}

// TestWeatherManager_AnnounceTextBroadcastOnStart verifies that when a weather event starts
// the Announce text is broadcast as a MessageEvent in addition to the WeatherEvent.
func TestWeatherManager_AnnounceTextBroadcastOnStart(t *testing.T) {
	repo := &stubWeatherRepo{}
	bc := &stubBroadcaster{}
	wm := gameserver.NewWeatherManager(repo, testWeatherTypes, 1.0, bc)

	// Tick with spring month at tick 1 — 100% roll rate will fire.
	wm.OnTick(gameserver.GameDateTime{Month: 4, Day: 1, Hour: 6, Tick: 1})

	msgs := bc.messages()
	require.Len(t, msgs, 1, "expected exactly one message broadcast on weather start")
	assert.Equal(t, "It rains.", msgs[0])
}

// TestWeatherManager_EndAnnounceTextBroadcastOnEnd verifies that when a weather event ends
// the EndAnnounce text is broadcast as a MessageEvent.
func TestWeatherManager_EndAnnounceTextBroadcastOnEnd(t *testing.T) {
	repo := &stubWeatherRepo{}
	bc := &stubBroadcaster{}
	wm := gameserver.NewWeatherManager(repo, testWeatherTypes, 1.0, bc)

	// Start event ending at tick 10.
	require.NoError(t, repo.StartEvent(context.Background(), "rain", 10))
	require.NoError(t, wm.LoadState(context.Background()))

	// Tick at endTick — should trigger endEvent.
	wm.OnTick(gameserver.GameDateTime{Month: 4, Tick: 10})

	msgs := bc.messages()
	require.Len(t, msgs, 1, "expected exactly one message broadcast on weather end")
	assert.Equal(t, "Stopped.", msgs[0])
}

// TestProperty_WeatherManager_AnnounceBroadcastMatchesActiveEvent verifies that the
// announce text is always broadcast when a new event starts.
func TestProperty_WeatherManager_AnnounceBroadcastMatchesActiveEvent(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		repo := &stubWeatherRepo{}
		bc := &stubBroadcaster{}
		wm := gameserver.NewWeatherManager(repo, testWeatherTypes, 1.0, bc)

		wm.OnTick(gameserver.GameDateTime{Month: 4, Tick: 1})

		ev, _ := repo.GetActive(context.Background())
		if ev == nil {
			// Shouldn't happen at 100% chance but be safe
			return
		}

		msgs := bc.messages()
		if len(msgs) != 1 {
			t.Fatalf("expected 1 announce message, got %d: %v", len(msgs), msgs)
		}
		if msgs[0] == "" {
			t.Fatal("broadcast announce text was empty")
		}
	})
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
