// Package main provides the web HTTP server for the MUD browser client.
package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/cory-johannsen/mud/cmd/webclient/eventbus"
	"github.com/cory-johannsen/mud/cmd/webclient/handlers"
	"github.com/cory-johannsen/mud/cmd/webclient/middleware"
	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	gamev1 "github.com/cory-johannsen/mud/internal/gameserver/gamev1"
)

// Server is the web HTTP server.
//
// Invariant: grpcConn is non-nil after New returns without error.
type Server struct {
	cfg         config.WebConfig
	httpServer  *http.Server
	grpcConn    *grpc.ClientConn
	accountRepo *postgres.AccountRepository
	charRepo    *postgres.CharacterRepository
	gameClient  gamev1.GameServiceClient
	bus         *eventbus.EventBus
	logger      *zap.Logger
}

// New constructs a Server, establishes the gRPC connection, and registers routes.
//
// Precondition: cfg.Port must be > 0; cfg.JWTSecret must be non-empty.
// Precondition: gameserverAddr must be a valid "host:port" string.
// Postcondition: Returns a ready-to-serve Server or a non-nil error.
func New(
	cfg config.WebConfig,
	gameserverAddr string,
	accountRepo *postgres.AccountRepository,
	charRepo *postgres.CharacterRepository,
	logger *zap.Logger,
) (*Server, error) {
	conn, err := grpc.NewClient(gameserverAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("dialing gameserver: %w", err)
	}

	bus := eventbus.New(256)

	s := &Server{
		cfg:         cfg,
		grpcConn:    conn,
		accountRepo: accountRepo,
		charRepo:    charRepo,
		gameClient:  gamev1.NewGameServiceClient(conn),
		bus:         bus,
		logger:      logger,
	}

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	port := cfg.Port
	if port == 0 {
		port = 8080
	}
	s.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	return s, nil
}

// ListenAndServe starts the HTTP server. It blocks until the server is stopped.
//
// Postcondition: Returns http.ErrServerClosed on graceful shutdown.
func (s *Server) ListenAndServe() error {
	s.logger.Info("web server listening", zap.String("addr", s.httpServer.Addr))
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server within the given deadline.
//
// Postcondition: In-flight requests are drained; gRPC connection is closed.
func (s *Server) Shutdown(ctx context.Context) error {
	if err := s.httpServer.Shutdown(ctx); err != nil {
		return fmt.Errorf("http shutdown: %w", err)
	}
	if err := s.grpcConn.Close(); err != nil {
		return fmt.Errorf("grpc close: %w", err)
	}
	return nil
}

// authMiddleware wraps a handler with JWT authentication, injecting account_id into context.
//
// Precondition: handler must not be nil.
// Postcondition: Claims are available via middleware.ClaimsFromContext and handlers.AccountIDFromContext.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return middleware.RequireJWT(s.cfg.JWTSecret, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := middleware.ClaimsFromContext(r.Context())
		if !ok {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ctx := handlers.WithAccountID(r.Context(), claims.AccountID)
		next.ServeHTTP(w, r.WithContext(ctx))
	}))
}

// adminMiddleware wraps a handler with both JWT auth and admin role enforcement.
func (s *Server) adminMiddleware(next http.Handler) http.Handler {
	return middleware.RequireJWT(s.cfg.JWTSecret, middleware.RequireAdminRole(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := middleware.ClaimsFromContext(r.Context())
			if !ok {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
			ctx := handlers.WithAccountID(r.Context(), claims.AccountID)
			next.ServeHTTP(w, r.WithContext(ctx))
		}),
	))
}

// registerRoutes wires all HTTP routes onto mux.
//
// Postcondition: /api/auth/* routes are registered; JWT middleware protects all
// other /api/* routes; admin routes require role admin or moderator.
func (s *Server) registerRoutes(mux *http.ServeMux) {
	authHandler := handlers.NewAuthHandler(s.accountRepo, s.cfg.JWTSecret)

	// Public auth routes — no JWT required.
	mux.HandleFunc("POST /api/auth/login", authHandler.Login)
	mux.HandleFunc("POST /api/auth/register", authHandler.Register)

	// Protected auth routes.
	mux.Handle("GET /api/auth/me",
		middleware.RequireJWT(s.cfg.JWTSecret, http.HandlerFunc(authHandler.Me)))

	// Character API — all protected by JWT.
	charHandler := handlers.NewCharacterHandler(s.charRepo, s.charRepo, s.charRepo)
	mux.Handle("GET /api/characters", s.authMiddleware(http.HandlerFunc(charHandler.ListCharacters)))
	mux.Handle("POST /api/characters", s.authMiddleware(http.HandlerFunc(charHandler.CreateCharacter)))
	mux.Handle("GET /api/characters/options", s.authMiddleware(http.HandlerFunc(charHandler.ListOptions)))
	mux.Handle("GET /api/characters/check-name", s.authMiddleware(http.HandlerFunc(charHandler.CheckName)))

	// WebSocket session — JWT validated inline by WSHandler.
	wsHandler := handlers.NewWSHandler(s.cfg.JWTSecret, s.gameClient, s.charRepo).
		WithLogger(s.logger).
		WithEventBus(s.bus)
	mux.Handle("GET /ws", http.HandlerFunc(wsHandler.ServeHTTP))

	// Admin API — all protected by JWT + RequireAdminRole.
	// WorldEditor and SessionManager are not yet wired from the game server;
	// a no-op implementation is used so routes are live and testable.
	adminHandler := handlers.NewAdminHandler(
		handlers.NewNoOpSessionManager(),
		s.accountRepo,
		handlers.NewNoOpWorldEditor(),
		s.bus,
	)
	mux.Handle("GET /api/admin/players", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleListPlayers)))
	mux.Handle("POST /api/admin/players/{char_id}/kick", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleKickPlayer)))
	mux.Handle("POST /api/admin/players/{char_id}/message", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleMessagePlayer)))
	mux.Handle("POST /api/admin/players/{char_id}/teleport", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleTeleportPlayer)))
	mux.Handle("GET /api/admin/accounts", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleSearchAccounts)))
	mux.Handle("PUT /api/admin/accounts/{id}", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleUpdateAccount)))
	mux.Handle("GET /api/admin/zones", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleListZones)))
	mux.Handle("GET /api/admin/zones/{zone_id}/rooms", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleListRooms)))
	mux.Handle("PUT /api/admin/rooms/{room_id}", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleUpdateRoom)))
	mux.Handle("GET /api/admin/npcs", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleListNPCs)))
	mux.Handle("POST /api/admin/rooms/{room_id}/spawn-npc", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleSpawnNPC)))
	mux.Handle("GET /api/admin/events", s.adminMiddleware(http.HandlerFunc(adminHandler.HandleAdminEvents)))

	// Static files + SPA fallback (implemented in static.go).
	mux.HandleFunc("/", s.serveIndex)
}

// serveIndex is registered on "/" and handles SPA routing + static assets.
func (s *Server) serveIndex(w http.ResponseWriter, r *http.Request) {
	staticDir := getEnv("WEB_STATIC_DIR", "")
	buildStaticHandler(staticDir).ServeHTTP(w, r)
}
