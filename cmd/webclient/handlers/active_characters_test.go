package handlers_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
)

func TestActiveCharacterRegistry_RegisterAndIsActive(t *testing.T) {
	r := handlers.NewActiveCharacterRegistry()
	assert.False(t, r.IsActive(1), "newly created registry should report no active characters")

	r.Register(1)
	assert.True(t, r.IsActive(1), "registered character should be active")
	assert.False(t, r.IsActive(2), "unregistered character should not be active")
}

func TestActiveCharacterRegistry_Deregister(t *testing.T) {
	r := handlers.NewActiveCharacterRegistry()
	r.Register(42)
	assert.True(t, r.IsActive(42))

	r.Deregister(42)
	assert.False(t, r.IsActive(42), "deregistered character should no longer be active")
}

func TestActiveCharacterRegistry_DeregisterNonExistent(t *testing.T) {
	r := handlers.NewActiveCharacterRegistry()
	// Deregistering a character that was never registered must not panic.
	assert.NotPanics(t, func() { r.Deregister(999) })
	assert.False(t, r.IsActive(999))
}

func TestActiveCharacterRegistry_MultipleCharacters(t *testing.T) {
	r := handlers.NewActiveCharacterRegistry()
	r.Register(1)
	r.Register(2)
	r.Register(3)

	assert.True(t, r.IsActive(1))
	assert.True(t, r.IsActive(2))
	assert.True(t, r.IsActive(3))

	r.Deregister(2)
	assert.True(t, r.IsActive(1))
	assert.False(t, r.IsActive(2))
	assert.True(t, r.IsActive(3))
}

func TestActiveCharacterRegistry_ConcurrentAccess(t *testing.T) {
	r := handlers.NewActiveCharacterRegistry()
	var wg sync.WaitGroup
	const goroutines = 50

	// Concurrently register and deregister to detect data races.
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(id int64) {
			defer wg.Done()
			r.Register(id)
			_ = r.IsActive(id)
			r.Deregister(id)
		}(int64(i))
	}
	wg.Wait()
	// All should be deregistered by now.
	for i := 0; i < goroutines; i++ {
		assert.False(t, r.IsActive(int64(i)))
	}
}
