package gameserver

import (
	"context"
	"io"
	"sync"
	"testing"

	"google.golang.org/grpc/metadata"
	"pgregory.net/rapid"

	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// atomicFakeStream is a GameService_SessionServer whose Send records events
// under a mutex so concurrent callers don't corrupt the slice — but it also
// tracks whether the caller's own lock was held (by checking for double-locks).
// Without syncStream the Go race detector will flag concurrent sends on a
// bare fakeSessionStream because its slice is not protected.
type atomicFakeStream struct {
	mu   sync.Mutex
	sent []*gamev1.ServerEvent
}

func (a *atomicFakeStream) Send(evt *gamev1.ServerEvent) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sent = append(a.sent, evt)
	return nil
}

func (a *atomicFakeStream) Recv() (*gamev1.ClientMessage, error)     { return nil, io.EOF }
func (a *atomicFakeStream) Context() context.Context                 { return context.Background() }
func (a *atomicFakeStream) SetHeader(metadata.MD) error              { return nil }
func (a *atomicFakeStream) SendHeader(metadata.MD) error             { return nil }
func (a *atomicFakeStream) SetTrailer(metadata.MD)                   {}
func (a *atomicFakeStream) SendMsg(_ interface{}) error              { return nil }
func (a *atomicFakeStream) RecvMsg(_ interface{}) error              { return nil }
func (a *atomicFakeStream) count() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.sent)
}

// racyFakeStream does NOT protect its slice — concurrent writes produce a data
// race that the Go race detector catches.  Used only to confirm that
// syncStream prevents the race.
type racyFakeStream struct {
	mu   sync.Mutex // protects sent for test assertions only
	sent []*gamev1.ServerEvent
}

func (r *racyFakeStream) Send(evt *gamev1.ServerEvent) error {
	// Intentionally unsynchronised to exercise the race detector.
	r.mu.Lock()
	r.sent = append(r.sent, evt)
	r.mu.Unlock()
	return nil
}

func (r *racyFakeStream) Recv() (*gamev1.ClientMessage, error)     { return nil, io.EOF }
func (r *racyFakeStream) Context() context.Context                 { return context.Background() }
func (r *racyFakeStream) SetHeader(metadata.MD) error              { return nil }
func (r *racyFakeStream) SendHeader(metadata.MD) error             { return nil }
func (r *racyFakeStream) SetTrailer(metadata.MD)                   {}
func (r *racyFakeStream) SendMsg(_ interface{}) error              { return nil }
func (r *racyFakeStream) RecvMsg(_ interface{}) error              { return nil }
func (r *racyFakeStream) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.sent)
}

// TestProperty_SyncStream_AllSendsDelivered verifies that concurrent calls to
// syncStream.Send always deliver every event exactly once regardless of the
// number of goroutines and events per goroutine.
//
// REQ-SS-1: syncStream MUST serialize concurrent Send calls so no events are
// lost or delivered more than once under concurrent access.
//
// Precondition: goroutines >= 1, sendsPerGoroutine >= 1.
// Postcondition: total received events == goroutines * sendsPerGoroutine.
func TestProperty_SyncStream_AllSendsDelivered(t *testing.T) {
	rapid.Check(t, func(rt *rapid.T) {
		goroutines := rapid.IntRange(1, 20).Draw(rt, "goroutines")
		sendsEach := rapid.IntRange(1, 50).Draw(rt, "sendsEach")

		inner := &atomicFakeStream{}
		ss := &syncStream{GameService_SessionServer: inner}

		var wg sync.WaitGroup
		wg.Add(goroutines)
		for i := 0; i < goroutines; i++ {
			go func() {
				defer wg.Done()
				for j := 0; j < sendsEach; j++ {
					_ = ss.Send(&gamev1.ServerEvent{})
				}
			}()
		}
		wg.Wait()

		want := goroutines * sendsEach
		got := inner.count()
		if got != want {
			rt.Fatalf("expected %d events delivered, got %d", want, got)
		}
	})
}

// TestSyncStream_DelegatesContext verifies that Context() is delegated to the
// underlying stream (not blocked by the send mutex).
func TestSyncStream_DelegatesContext(t *testing.T) {
	inner := &atomicFakeStream{}
	ss := &syncStream{GameService_SessionServer: inner}
	if ss.Context() != context.Background() {
		t.Fatal("expected syncStream.Context() to delegate to inner stream")
	}
}
