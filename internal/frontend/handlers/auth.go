// Package handlers provides Telnet session handling and command processing.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// AccountStore defines the account persistence operations required by AuthHandler.
type AccountStore interface {
	Create(ctx context.Context, username, password string) (postgres.Account, error)
	Authenticate(ctx context.Context, username, password string) (postgres.Account, error)
}

// CharacterStore defines the character persistence operations required by AuthHandler.
type CharacterStore interface {
	ListByAccount(ctx context.Context, accountID int64) ([]*character.Character, error)
	Create(ctx context.Context, c *character.Character) (*character.Character, error)
	GetByID(ctx context.Context, id int64) (*character.Character, error)
}

const welcomeBanner = `
` + telnet.Bold + telnet.BrightCyan + `
  ██████╗ ██╗   ██╗███╗   ██╗ ██████╗██╗  ██╗███████╗████████╗███████╗
 ██╔════╝ ██║   ██║████╗  ██║██╔════╝██║  ██║██╔════╝╚══██╔══╝██╔════╝
 ██║  ███╗██║   ██║██╔██╗ ██║██║     ███████║█████╗     ██║   █████╗
 ██║   ██║██║   ██║██║╚██╗██║██║     ██╔══██║██╔══╝     ██║   ██╔══╝
 ╚██████╔╝╚██████╔╝██║ ╚████║╚██████╗██║  ██║███████╗   ██║   ███████╗
  ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝   ╚══════╝` + telnet.Reset + `

` + telnet.BrightYellow + `  Post-Collapse Portland, OR — A Dystopian Sci-Fi MUD` + telnet.Reset + `

  Type ` + telnet.Green + `login <username> <password>` + telnet.Reset + ` to connect.
  Type ` + telnet.Green + `register <username> <password>` + telnet.Reset + ` to create an account.
  Type ` + telnet.Green + `quit` + telnet.Reset + ` to disconnect.
`

// AuthHandler implements telnet.SessionHandler and processes the
// authentication loop for a connected client.
type AuthHandler struct {
	accounts       AccountStore
	characters     CharacterStore
	regions        []*ruleset.Region
	teams          []*ruleset.Team
	jobs           []*ruleset.Job
	logger         *zap.Logger
	gameServerAddr string
}

// NewAuthHandler creates an AuthHandler backed by the given account and character stores.
//
// Precondition: accounts, characters, and logger must be non-nil. gameServerAddr must be a valid "host:port" address.
// Postcondition: Returns an AuthHandler ready to handle sessions.
func NewAuthHandler(
	accounts AccountStore,
	characters CharacterStore,
	regions []*ruleset.Region,
	teams []*ruleset.Team,
	jobs []*ruleset.Job,
	logger *zap.Logger,
	gameServerAddr string,
) *AuthHandler {
	return &AuthHandler{
		accounts:       accounts,
		characters:     characters,
		regions:        regions,
		teams:          teams,
		jobs:           jobs,
		logger:         logger,
		gameServerAddr: gameServerAddr,
	}
}

// HandleSession implements telnet.SessionHandler. It shows the welcome banner
// and processes authentication commands until the player logs in or quits.
//
// Postcondition: Returns nil on clean quit, or an error if the session ended abnormally.
func (h *AuthHandler) HandleSession(ctx context.Context, conn *telnet.Conn) error {
	start := time.Now()
	addr := conn.RemoteAddr().String()

	if err := conn.Write([]byte(welcomeBanner)); err != nil {
		return fmt.Errorf("sending welcome: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			_ = conn.WriteLine(telnet.Colorize(telnet.Yellow, "Server shutting down. Goodbye!"))
			return ctx.Err()
		default:
		}

		if err := conn.WritePrompt(telnet.Colorize(telnet.BrightWhite, "> ")); err != nil {
			return fmt.Errorf("writing prompt: %w", err)
		}

		line, err := conn.ReadLine()
		if err != nil {
			return fmt.Errorf("reading input: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		cmd := strings.ToLower(parts[0])
		args := parts[1:]

		switch cmd {
		case "quit", "exit":
			_ = conn.WriteLine(telnet.Colorize(telnet.Cyan, "Goodbye!"))
			h.logger.Info("client quit",
				zap.String("remote_addr", addr),
				zap.Duration("session_duration", time.Since(start)),
			)
			return nil

		case "login":
			acct, err := h.handleLogin(ctx, conn, args)
			if err != nil {
				return err
			}
			if acct.ID == 0 {
				continue
			}
			h.logger.Info("player logged in",
				zap.String("remote_addr", addr),
				zap.String("username", acct.Username),
				zap.Duration("login_time", time.Since(start)),
			)
			if err := h.characterFlow(ctx, conn, acct); err != nil {
				return err
			}
			return nil

		case "register":
			if err := h.handleRegister(ctx, conn, args); err != nil {
				return err
			}

		case "help":
			h.showHelp(conn)

		default:
			_ = conn.WriteLine(telnet.Colorf(telnet.Red, "Unknown command: %s. Type 'help' for available commands.", cmd))
		}
	}
}

// handleLogin authenticates a player.
//
// Postcondition: Returns (acct, nil) on success, (postgres.Account{}, nil) if the error was
// shown to the user and the auth loop should continue, or (postgres.Account{}, error) on fatal errors.
func (h *AuthHandler) handleLogin(ctx context.Context, conn *telnet.Conn, args []string) (postgres.Account, error) {
	if len(args) < 2 {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Usage: login <username> <password>"))
		return postgres.Account{}, nil
	}

	username := args[0]
	password := args[1]

	start := time.Now()
	acct, err := h.accounts.Authenticate(ctx, username, password)
	elapsed := time.Since(start)

	if err != nil {
		switch {
		case errors.Is(err, postgres.ErrAccountNotFound):
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Account not found. Use 'register' to create one."))
			return postgres.Account{}, nil
		case errors.Is(err, postgres.ErrInvalidCredentials):
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid password."))
			return postgres.Account{}, nil
		default:
			h.logger.Error("authentication error", zap.Error(err), zap.Duration("elapsed", elapsed))
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "An internal error occurred. Please try again."))
			return postgres.Account{}, nil
		}
	}

	_ = conn.WriteLine(telnet.Colorf(telnet.BrightGreen,
		"Welcome back, %s! (account #%d) [%s]",
		acct.Username, acct.ID, elapsed,
	))
	return acct, nil
}

func (h *AuthHandler) handleRegister(ctx context.Context, conn *telnet.Conn, args []string) error {
	if len(args) < 2 {
		return conn.WriteLine(telnet.Colorize(telnet.Red, "Usage: register <username> <password>"))
	}

	username := args[0]
	password := args[1]

	if len(username) < 3 || len(username) > 32 {
		return conn.WriteLine(telnet.Colorize(telnet.Red, "Username must be 3-32 characters."))
	}
	if len(password) < 6 {
		return conn.WriteLine(telnet.Colorize(telnet.Red, "Password must be at least 6 characters."))
	}

	start := time.Now()
	acct, err := h.accounts.Create(ctx, username, password)
	elapsed := time.Since(start)

	if err != nil {
		if errors.Is(err, postgres.ErrAccountExists) {
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "That username is already taken."))
			return nil
		}
		h.logger.Error("registration error", zap.Error(err), zap.Duration("elapsed", elapsed))
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "An internal error occurred. Please try again."))
		return nil
	}

	_ = conn.WriteLine(telnet.Colorf(telnet.BrightGreen,
		"Account created: %s (#%d). You may now 'login'. [%s]",
		acct.Username, acct.ID, elapsed,
	))
	return nil
}

func (h *AuthHandler) showHelp(conn *telnet.Conn) {
	help := telnet.Colorize(telnet.BrightWhite, "Available commands:") + "\r\n" +
		telnet.Colorize(telnet.Green, "  login <username> <password>") + "    — Log in to your account\r\n" +
		telnet.Colorize(telnet.Green, "  register <username> <password>") + " — Create a new account\r\n" +
		telnet.Colorize(telnet.Green, "  help") + "                           — Show this help\r\n" +
		telnet.Colorize(telnet.Green, "  quit") + "                           — Disconnect\r\n"
	_ = conn.Write([]byte(help))
}

