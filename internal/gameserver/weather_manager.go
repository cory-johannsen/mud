package gameserver

import (
	"context"
	"math/rand"
	"sync"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
	"github.com/cory-johannsen/mud/internal/game/world"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// WeatherBroadcaster sends a ServerEvent to all connected player sessions.
type WeatherBroadcaster interface {
	BroadcastAll(ev *gamev1.ServerEvent)
}

// WeatherRepo is the persistence interface for weather events.
// Re-exported alias so callers in other packages can use the interface without importing postgres.
type WeatherRepo = postgres.WeatherRepo

// WeatherManager subscribes to GameCalendar ticks and manages random weather events.
//
// Precondition: repo and weatherTypes must not be nil; chancePerTick in [0.0, 1.0].
// Postcondition: OnTick is safe to call concurrently; broadcaster may be nil.
type WeatherManager struct {
	repo          WeatherRepo
	weatherTypes  []WeatherType
	chancePerTick float64
	broadcaster   WeatherBroadcaster

	mu          sync.RWMutex
	activeName  string
	endTick     int64
	cooldownEnd int64
}

// NewWeatherManager creates a WeatherManager with the given persistence repo, weather type definitions,
// per-tick chance of a new event starting, and optional broadcaster.
//
// Precondition: repo != nil; weatherTypes must not be empty for events to fire.
// Postcondition: Returns a non-nil *WeatherManager ready to call LoadState and OnTick.
func NewWeatherManager(repo WeatherRepo, weatherTypes []WeatherType, chancePerTick float64, broadcaster WeatherBroadcaster) *WeatherManager {
	return &WeatherManager{
		repo:          repo,
		weatherTypes:  weatherTypes,
		chancePerTick: chancePerTick,
		broadcaster:   broadcaster,
	}
}

// LoadState reads persisted weather state from the repo into memory.
//
// Precondition: ctx must not be nil.
// Postcondition: wm.activeName, wm.endTick, and wm.cooldownEnd reflect the DB state.
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

// OnTick is called on each game calendar tick. It ends expired events, clears expired
// cooldowns, and may start a new weather event based on chancePerTick.
//
// Precondition: dt.Month must be in [1,12].
// Postcondition: At most one event is started or ended per call.
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

	// If no cooldown loaded in memory, check the repo once to hydrate.
	if wm.cooldownEnd == 0 {
		if endTick, found, err := wm.repo.GetCooldownEnd(ctx); err == nil && found {
			wm.cooldownEnd = endTick
		}
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

// endEvent marks the current event as ended, sets a cooldown, and broadcasts the end.
// Caller must hold wm.mu.
func (wm *WeatherManager) endEvent(ctx context.Context, currentTick int64) {
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

// sampleWeatherType selects a random WeatherType for the given season using weighted sampling.
// Returns nil if no eligible types exist for the season.
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

// ActiveEffects returns the RoomEffect slice for the currently active weather event.
// Returns nil for indoor rooms or when no event is active.
//
// Precondition: none.
// Postcondition: Returns nil when indoor is true or no event is active.
func (wm *WeatherManager) ActiveEffects(indoor bool) []world.RoomEffect {
	if indoor {
		return nil
	}
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	if wm.activeName == "" {
		return nil
	}
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

// ActiveWeatherName returns the name of the currently active weather event, or empty string if none.
func (wm *WeatherManager) ActiveWeatherName() string {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.activeName
}
