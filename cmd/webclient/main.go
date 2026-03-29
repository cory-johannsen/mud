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
	"github.com/cory-johannsen/mud/internal/game/technology"
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
	featsFile     := flag.String("feats-file", "content/feats.yaml", "path to feats YAML file")
	skillsFile    := flag.String("skills-file", "content/skills.yaml", "path to skills YAML file")
	techDir       := flag.String("tech-dir", "content/technologies", "path to technology YAML directory")
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

	// Character creation choice persistence repositories.
	abilityBoostsRepo := postgres.NewCharacterAbilityBoostsRepository(pool.DB())
	skillsRepo := postgres.NewCharacterSkillsRepository(pool.DB())
	featsRepo := postgres.NewCharacterFeatsRepository(pool.DB())
	hwTechRepo := postgres.NewCharacterHardwiredTechRepository(pool.DB())
	spontTechRepo := postgres.NewCharacterSpontaneousTechRepository(pool.DB())
	preparedTechRepo := postgres.NewCharacterPreparedTechRepository(pool.DB())

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
	feats, featsErr := ruleset.LoadFeats(*featsFile)
	if featsErr != nil {
		logger.Warn("loading feats for character wizard", zap.Error(featsErr))
	}
	skills, skillsErr := ruleset.LoadSkills(*skillsFile)
	if skillsErr != nil {
		logger.Warn("loading skills for character wizard", zap.Error(skillsErr))
	}
	techRegistry, techErr := technology.Load(*techDir)
	if techErr != nil {
		logger.Warn("loading technology for character wizard", zap.Error(techErr))
	}

	var charOpts *handlers.CharacterOptions
	if jobs != nil && regions != nil && archetypes != nil && teams != nil {
		charOpts = &handlers.CharacterOptions{
			Jobs:         jobs,
			Regions:      regions,
			Archetypes:   archetypes,
			Teams:        teams,
			Feats:        feats,
			Skills:       skills,
			TechRegistry: techRegistry,
		}
	}

	creationRepos := &charCreationRepos{
		abilityBoosts: abilityBoostsRepo,
		skills:        skillsRepo,
		feats:         featsRepo,
		hwTech:        hwTechRepo,
		spontTech:     spontTechRepo,
		preparedTech:  preparedTechRepo,
	}

	srv, err := New(cfg.Web, cfg.GameServer.Addr(), accountRepo, charRepo, charOpts, creationRepos, logger)
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
