package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/observability"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

func main() {
	start := time.Now()

	configPath    := flag.String("config", "configs/dev.yaml", "path to configuration file")
	jobsDir       := flag.String("jobs-dir", "content/jobs", "path to job YAML definitions")
	regionsDir    := flag.String("regions-dir", "content/regions", "path to region YAML definitions")
	archetypesDir := flag.String("archetypes-dir", "content/archetypes", "path to archetype YAML definitions")
	teamsDir      := flag.String("teams-dir", "content/teams", "path to team YAML definitions")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("loading config: %v", err)
	}

	logger, err := observability.NewLogger(cfg.Logging)
	if err != nil {
		log.Fatalf("initializing logger: %v", err)
	}
	defer logger.Sync()

	pool, err := postgres.NewPool(context.Background(), cfg.Database)
	if err != nil {
		logger.Fatal("connecting to database", zap.Error(err))
	}
	defer pool.Close()

	accountRepo := postgres.NewAccountRepository(pool.DB())
	charRepo := postgres.NewCharacterRepository(pool.DB())

	// Load character creation options from content directories.
	jobs, jobsErr := ruleset.LoadJobs(*jobsDir)
	if jobsErr != nil {
		logger.Warn("loading jobs for character wizard", zap.Error(jobsErr))
	}
	regions, regionsErr := ruleset.LoadRegions(*regionsDir)
	if regionsErr != nil {
		logger.Warn("loading regions for character wizard", zap.Error(regionsErr))
	}
	archetypes, archetypesErr := ruleset.LoadArchetypes(*archetypesDir)
	if archetypesErr != nil {
		logger.Warn("loading archetypes for character wizard", zap.Error(archetypesErr))
	}
	teams, teamsErr := ruleset.LoadTeams(*teamsDir)
	if teamsErr != nil {
		logger.Warn("loading teams for character wizard", zap.Error(teamsErr))
	}

	var charOpts *handlers.CharacterOptions
	if jobs != nil && regions != nil && archetypes != nil && teams != nil {
		charOpts = &handlers.CharacterOptions{
			Jobs:       jobs,
			Regions:    regions,
			Archetypes: archetypes,
			Teams:      teams,
		}
	}

	srv, err := New(cfg.Web, cfg.GameServer.Addr(), accountRepo, charRepo, charOpts, logger)
	if err != nil {
		logger.Fatal("initializing web server", zap.Error(err))
	}

	logger.Info("webclient starting",
		zap.Duration("startup", time.Since(start)),
		zap.String("gameserver_addr", cfg.GameServer.Addr()),
	)

	// Start server in background goroutine.
	errCh := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for OS signal or server error.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-quit:
		logger.Info("received signal, shutting down", zap.String("signal", sig.String()))
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", zap.Error(err))
		}
		return
	}

	// Graceful shutdown with 10-second drain timeout (REQ-WC-48).
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
	logger.Info("web server stopped")
}

// getEnv returns the value of the environment variable or the fallback.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
