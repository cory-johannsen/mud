package server

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

type mockService struct {
	started atomic.Bool
	stopped atomic.Bool
	startFn func() error
}

func (m *mockService) Start() error {
	m.started.Store(true)
	if m.startFn != nil {
		return m.startFn()
	}
	// Block until stopped
	for !m.stopped.Load() {
		time.Sleep(10 * time.Millisecond)
	}
	return nil
}

func (m *mockService) Stop() {
	m.stopped.Store(true)
}

func TestLifecycleStartsAndStopsServices(t *testing.T) {
	logger := zaptest.NewLogger(t)
	lc := NewLifecycle(logger)

	svc1 := &mockService{}
	svc2 := &mockService{}

	lc.Add("svc1", svc1)
	lc.Add("svc2", svc2)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- lc.Run(ctx)
	}()

	// Wait for services to start
	deadline := time.After(2 * time.Second)
	for {
		if svc1.started.Load() && svc2.started.Load() {
			break
		}
		select {
		case <-deadline:
			t.Fatal("services did not start in time")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	assert.True(t, svc1.started.Load())
	assert.True(t, svc2.started.Load())

	// Trigger shutdown
	cancel()

	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("lifecycle did not shut down in time")
	}

	assert.True(t, svc1.stopped.Load())
	assert.True(t, svc2.stopped.Load())
}

func TestFuncService(t *testing.T) {
	started := false
	stopped := false

	svc := &FuncService{
		StartFn: func() error {
			started = true
			return nil
		},
		StopFn: func() {
			stopped = true
		},
	}

	err := svc.Start()
	assert.NoError(t, err)
	assert.True(t, started)

	svc.Stop()
	assert.True(t, stopped)
}
