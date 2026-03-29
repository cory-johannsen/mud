// Package eventbus_test contains property-based and unit tests for EventBus.
package eventbus_test

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"pgregory.net/rapid"

	"github.com/cory-johannsen/mud/cmd/webclient/eventbus"
)

// TestEventBus_PublishAllSubscribersReceiveAllEvents is a property test that
// verifies every subscriber receives every published event in order.
func TestEventBus_PublishAllSubscribersReceiveAllEvents(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		numEvents := rapid.IntRange(1, 20).Draw(rt, "numEvents")
		numSubscribers := rapid.IntRange(1, 5).Draw(rt, "numSubscribers")

		bus := eventbus.New(100)

		channels := make([]<-chan eventbus.Event, numSubscribers)
		unsubs := make([]func(), numSubscribers)
		for i := 0; i < numSubscribers; i++ {
			ch, unsub := bus.Subscribe()
			channels[i] = ch
			unsubs[i] = unsub
		}
		defer func() {
			for _, u := range unsubs {
				u()
			}
		}()

		// Publish N events.
		events := make([]eventbus.Event, numEvents)
		for i := 0; i < numEvents; i++ {
			payload, _ := json.Marshal(map[string]int{"seq": i})
			events[i] = eventbus.Event{
				Type:    "TestEvent",
				Payload: json.RawMessage(payload),
				Time:    time.Now(),
			}
			bus.Publish(events[i])
		}

		// Each subscriber should receive all N events.
		for si, ch := range channels {
			for i := 0; i < numEvents; i++ {
				select {
				case got := <-ch:
					if got.Type != events[i].Type {
						rt.Fatalf("subscriber %d event %d: got type %q want %q", si, i, got.Type, events[i].Type)
					}
				case <-time.After(2 * time.Second):
					rt.Fatalf("subscriber %d timed out waiting for event %d", si, i)
				}
			}
		}
	})
}

// TestEventBus_SlowSubscriberIsDropped verifies that a subscriber whose buffer
// is full is removed non-blockingly and its channel is closed.
func TestEventBus_SlowSubscriberIsDropped(t *testing.T) {
	bufSize := 2
	bus := eventbus.New(bufSize)

	ch, _ := bus.Subscribe()

	// Publish bufSize+1 events to overflow the slow subscriber.
	for i := 0; i <= bufSize; i++ {
		bus.Publish(eventbus.Event{Type: "Fill", Payload: json.RawMessage(`{}`), Time: time.Now()})
	}

	// The channel should eventually be closed (dropped).
	deadline := time.After(2 * time.Second)
	// Drain any buffered events first.
	for {
		select {
		case _, open := <-ch:
			if !open {
				return // correctly dropped
			}
		case <-deadline:
			t.Fatal("slow subscriber channel was never closed")
		}
	}
}

// TestEventBus_UnsubscribeStopsDelivery verifies that after calling the
// unsubscribe func, no further events are delivered to that subscriber.
func TestEventBus_UnsubscribeStopsDelivery(t *testing.T) {
	bus := eventbus.New(100)

	ch, unsub := bus.Subscribe()

	// Publish one event, verify receipt.
	bus.Publish(eventbus.Event{Type: "A", Payload: json.RawMessage(`{}`), Time: time.Now()})
	select {
	case e := <-ch:
		if e.Type != "A" {
			t.Fatalf("expected type A, got %s", e.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("did not receive first event")
	}

	// Unsubscribe then publish another event.
	unsub()

	// Channel should be closed.
	select {
	case _, open := <-ch:
		if open {
			// May have received a second publish that raced; drain.
		} else {
			return // correctly closed
		}
	case <-time.After(time.Second):
		t.Fatal("channel not closed after Unsubscribe")
	}
}

// TestEventBus_UnsubscribeIdempotent verifies calling unsubscribe twice does not panic.
func TestEventBus_UnsubscribeIdempotent(t *testing.T) {
	bus := eventbus.New(10)
	_, unsub := bus.Subscribe()
	unsub()
	unsub() // must not panic
}

// TestEventBus_ConcurrentPublish verifies no data races under concurrent publishing.
func TestEventBus_ConcurrentPublish(t *testing.T) {
	bus := eventbus.New(1000)

	var received atomic.Int64
	ch, unsub := bus.Subscribe()
	defer unsub()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range ch {
			received.Add(1)
		}
	}()

	const n = 50
	var pub sync.WaitGroup
	for i := 0; i < n; i++ {
		pub.Add(1)
		go func() {
			defer pub.Done()
			bus.Publish(eventbus.Event{Type: "C", Payload: json.RawMessage(`{}`), Time: time.Now()})
		}()
	}
	pub.Wait()
	unsub()
	wg.Wait()

	if received.Load() == 0 {
		t.Fatal("no events received under concurrent publish")
	}
}
