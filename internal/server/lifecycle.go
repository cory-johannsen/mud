// Package server provides application lifecycle management including
// graceful startup and shutdown with signal handling.
package server

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
)

// Service represents a long-running component that can be started and stopped.
type Service interface {
	// Start begins the service. It should block until the service is stopped
	// or an error occurs.
	Start() error
	// Stop gracefully stops the service.
	Stop()
}

// FuncService adapts a start/stop function pair into the Service interface.
type FuncService struct {
	StartFn func() error
	StopFn  func()
}

// Start calls the underlying start function.
func (f *FuncService) Start() error { return f.StartFn() }

// Stop calls the underlying stop function.
func (f *FuncService) Stop() { f.StopFn() }

// Lifecycle manages the startup and shutdown of multiple services.
// Services are started in order and stopped in reverse order.
type Lifecycle struct {
	logger   *zap.Logger
	services []namedService
	mu       sync.Mutex
}

type namedService struct {
	name    string
	service Service
}

// NewLifecycle creates a new Lifecycle manager.
//
// Precondition: logger must be non-nil.
func NewLifecycle(logger *zap.Logger) *Lifecycle {
	return &Lifecycle{
		logger: logger,
	}
}

// Add registers a named service for lifecycle management.
// Services are started in the order they are added.
//
// Precondition: name must be non-empty; svc must be non-nil.
func (l *Lifecycle) Add(name string, svc Service) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.services = append(l.services, namedService{name: name, service: svc})
}

// Run starts all services and blocks until a termination signal is received
// (SIGINT or SIGTERM). On signal, services are stopped in reverse order.
//
// Postcondition: All services are stopped when this method returns.
func (l *Lifecycle) Run(ctx context.Context) error {
	start := time.Now()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// Start services
	errCh := make(chan error, len(l.services))
	for _, ns := range l.services {
		ns := ns
		go func() {
			l.logger.Info("starting service",
				zap.String("service", ns.name),
			)
			svcStart := time.Now()
			if err := ns.service.Start(); err != nil {
				l.logger.Error("service failed",
					zap.String("service", ns.name),
					zap.Error(err),
					zap.Duration("uptime", time.Since(svcStart)),
				)
				errCh <- fmt.Errorf("service %s: %w", ns.name, err)
				cancel()
			}
		}()
	}

	l.logger.Info("all services started",
		zap.Int("count", len(l.services)),
		zap.Duration("startup", time.Since(start)),
	)

	// Wait for signal or context cancellation
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		l.logger.Info("received signal, shutting down",
			zap.String("signal", sig.String()),
		)
	case err := <-errCh:
		l.logger.Error("service error, shutting down",
			zap.Error(err),
		)
	case <-ctx.Done():
		l.logger.Info("context cancelled, shutting down")
	}

	// Stop services in reverse order
	l.shutdown()

	l.logger.Info("shutdown complete",
		zap.Duration("total_uptime", time.Since(start)),
	)
	return nil
}

func (l *Lifecycle) shutdown() {
	shutdownStart := time.Now()
	for i := len(l.services) - 1; i >= 0; i-- {
		ns := l.services[i]
		svcStart := time.Now()
		l.logger.Info("stopping service",
			zap.String("service", ns.name),
		)
		ns.service.Stop()
		l.logger.Info("service stopped",
			zap.String("service", ns.name),
			zap.Duration("elapsed", time.Since(svcStart)),
		)
	}
	l.logger.Info("all services stopped",
		zap.Duration("shutdown_elapsed", time.Since(shutdownStart)),
	)
}
