// Package handlers provides Telnet session handling and command processing.
package handlers

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/config"
	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
	"github.com/cory-johannsen/mud/internal/version"
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
	SaveGender(ctx context.Context, id int64, gender string) error
}

// buildWelcomeBanner returns the connection banner with the current version embedded.
// Layout: AK-47 (14 cols, bright green) | GUNCHETE title (52 cols, bright cyan) | machete (14 cols, bright yellow)
// Total visible width per art row: exactly 80 characters.
func buildWelcomeBanner() string {
	// AK-47 ASCII art — each row must be exactly 14 visible characters.
	// Verify with: len(telnet.StripANSI(row)) == 14
	ak47 := []string{
		` _________    `,
		`|  _  |   |=> `,
		`|_| |_|===|   `,
		`  | |  \__/   `,
		`  |_|         `,
		` _| |_        `,
		`|_____|       `,
	}

	// GUNCHETE medium ASCII title — each row must be exactly 52 visible characters.
	// Verify with: len(telnet.StripANSI(row)) == 52
	// Note: row 2 contains a backtick and requires string concatenation to embed in Go source.
	title := []string{
		`  ___  _   _ _  _  ___  _  _  ___  ___ ___          `,
		` / __|| | | | \| |/ __|| || || __||_  |_  |         `,
		`| (_ || |_| | .` + "`" + `| | (__ | __ || _|  / /  / /        `,
		` \___/ \___/|_|\_|\___||_||_||___| /_/  /_/         `,
		`                                                    `,
		`                                                    `,
		`                                                    `,
	}

	// Machete ASCII art — each row must be exactly 14 visible characters.
	// Verify with: len(telnet.StripANSI(row)) == 14
	machete := []string{
		`         _    `,
		`        / \   `,
		`  _____/   \  `,
		` /__________\ `,
		`      |       `,
		`      |       `,
		`     (_)      `,
	}

	// Normalize column heights by padding shorter slices with blank rows at the bottom.
	maxRows := len(ak47)
	if len(title) > maxRows {
		maxRows = len(title)
	}
	if len(machete) > maxRows {
		maxRows = len(machete)
	}
	pad := func(col []string, width, count int) []string {
		blank := strings.Repeat(" ", width)
		for len(col) < count {
			col = append(col, blank)
		}
		return col
	}
	ak47 = pad(ak47, 14, maxRows)
	title = pad(title, 52, maxRows)
	machete = pad(machete, 14, maxRows)

	// Build banner rows. Each row: BrightGreen+ak47+Reset | BrightCyan+title+Reset | BrightYellow+machete+Reset
	var sb strings.Builder
	sb.WriteString("\n")
	for i := 0; i < maxRows; i++ {
		sb.WriteString(telnet.BrightGreen + ak47[i] + telnet.Reset)
		sb.WriteString(telnet.BrightCyan + title[i] + telnet.Reset)
		sb.WriteString(telnet.BrightYellow + machete[i] + telnet.Reset)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	sb.WriteString(telnet.BrightYellow + `  Post-Collapse Portland, OR — A Dystopian Sci-Fi MUD` + telnet.Reset + "\n")
	sb.WriteString(telnet.Dim + `  ` + version.Version + telnet.Reset + "\n")
	sb.WriteString("\n")
	sb.WriteString(`  Type ` + telnet.Green + `login` + telnet.Reset + ` to connect.` + "\n")
	sb.WriteString(`  Type ` + telnet.Green + `register` + telnet.Reset + ` to create an account.` + "\n")
	sb.WriteString(`  Type ` + telnet.Green + `quit` + telnet.Reset + ` to disconnect.` + "\n")

	return sb.String()
}

// CharacterSkillsSetter defines the skill persistence operations required by AuthHandler.
type CharacterSkillsSetter interface {
	HasSkills(ctx context.Context, characterID int64) (bool, error)
	SetAll(ctx context.Context, characterID int64, skills map[string]string) error
}

// CharacterFeatsSetter defines feat persistence operations required by AuthHandler.
// HasFeats is intentionally omitted: ensureFeats uses GetAll to load the stored
// feat list and computes per-pool deficits directly, so a separate boolean check
// is not needed. The concrete type still provides HasFeats; it is simply not
// required by this interface.
type CharacterFeatsSetter interface {
	GetAll(ctx context.Context, characterID int64) ([]string, error)
	SetAll(ctx context.Context, characterID int64, feats []string) error
}

// CharacterClassFeaturesSetter defines class feature persistence operations required by AuthHandler.
type CharacterClassFeaturesSetter interface {
	HasClassFeatures(ctx context.Context, characterID int64) (bool, error)
	SetAll(ctx context.Context, characterID int64, featureIDs []string) error
}

// AuthHandler implements telnet.SessionHandler and processes the
// authentication loop for a connected client.
type AuthHandler struct {
	accounts       AccountStore
	characters     CharacterStore
	regions        []*ruleset.Region
	teams          []*ruleset.Team
	jobs           []*ruleset.Job
	archetypes     []*ruleset.Archetype
	jobRegistry    *ruleset.JobRegistry
	allSkills       []*ruleset.Skill
	characterSkills CharacterSkillsSetter
	characterFeats  CharacterFeatsSetter
	allFeats               []*ruleset.Feat
	featRegistry           *ruleset.FeatRegistry
	characterClassFeatures CharacterClassFeaturesSetter
	allClassFeatures       []*ruleset.ClassFeature
	classFeatureRegistry   *ruleset.ClassFeatureRegistry
	logger                 *zap.Logger
	gameServerAddr string
	telnetCfg      config.TelnetConfig
}

// NewAuthHandler creates an AuthHandler backed by the given account and character stores.
//
// Precondition: accounts, characters, and logger must be non-nil. gameServerAddr must be a valid "host:port" address.
// allSkills may be nil (skill selection will be skipped). characterSkills may be nil (skills will not be persisted).
// Postcondition: Returns an AuthHandler ready to handle sessions with a populated JobRegistry.
func NewAuthHandler(
	accounts AccountStore,
	characters CharacterStore,
	regions []*ruleset.Region,
	teams []*ruleset.Team,
	jobs []*ruleset.Job,
	archetypes []*ruleset.Archetype,
	logger *zap.Logger,
	gameServerAddr string,
	telnetCfg config.TelnetConfig,
	allSkills []*ruleset.Skill,
	characterSkills CharacterSkillsSetter,
	allFeats []*ruleset.Feat,
	characterFeats CharacterFeatsSetter,
	allClassFeatures []*ruleset.ClassFeature,
	characterClassFeatures CharacterClassFeaturesSetter,
) *AuthHandler {
	reg := ruleset.NewJobRegistry()
	for _, j := range jobs {
		reg.Register(j)
	}
	var featReg *ruleset.FeatRegistry
	if len(allFeats) > 0 {
		featReg = ruleset.NewFeatRegistry(allFeats)
	}
	var cfReg *ruleset.ClassFeatureRegistry
	if len(allClassFeatures) > 0 {
		cfReg = ruleset.NewClassFeatureRegistry(allClassFeatures)
	}
	return &AuthHandler{
		accounts:        accounts,
		characters:      characters,
		regions:         regions,
		teams:           teams,
		jobs:            jobs,
		archetypes:      archetypes,
		jobRegistry:     reg,
		allSkills:       allSkills,
		characterSkills: characterSkills,
		characterFeats:         characterFeats,
		allFeats:               allFeats,
		featRegistry:           featReg,
		characterClassFeatures: characterClassFeatures,
		allClassFeatures:       allClassFeatures,
		classFeatureRegistry:   cfReg,
		logger:                 logger,
		gameServerAddr:  gameServerAddr,
		telnetCfg:       telnetCfg,
	}
}

// HandleSession implements telnet.SessionHandler. It shows the welcome banner
// and processes authentication commands until the player logs in or quits.
//
// Postcondition: Returns nil on clean quit, or an error if the session ended abnormally.
func (h *AuthHandler) HandleSession(ctx context.Context, conn *telnet.Conn) error {
	start := time.Now()
	addr := conn.RemoteAddr().String()

	if err := conn.Write([]byte(buildWelcomeBanner())); err != nil {
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

// handleLogin authenticates a player interactively.
// Username is taken from args[0] if provided, otherwise prompted.
// Password is always prompted with echo suppressed.
//
// Postcondition: Returns (acct, nil) on success, (postgres.Account{}, nil) if the
// user should retry, or (postgres.Account{}, error) on fatal connection errors.
func (h *AuthHandler) handleLogin(ctx context.Context, conn *telnet.Conn, args []string) (postgres.Account, error) {
	var username string
	if len(args) > 0 {
		username = args[0]
	}

	if username == "" {
		_ = conn.WritePrompt(telnet.Colorize(telnet.White, "Username: "))
		var err error
		username, err = conn.ReadLine()
		if err != nil {
			return postgres.Account{}, fmt.Errorf("reading username: %w", err)
		}
		username = strings.TrimSpace(username)
	}

	if username == "" {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Username cannot be empty."))
		return postgres.Account{}, nil
	}

	_ = conn.WritePrompt(telnet.Colorize(telnet.White, "Password: "))
	password, err := conn.ReadPassword()
	if err != nil {
		return postgres.Account{}, fmt.Errorf("reading password: %w", err)
	}

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
		"Logged in as %s [%s] (account #%d) [%s]",
		acct.Username, acct.Role, acct.ID, elapsed,
	))
	return acct, nil
}

// handleRegister creates a new account interactively.
// Username is taken from args[0] if provided, otherwise prompted.
// Password is prompted twice with echo suppressed; both entries must match.
func (h *AuthHandler) handleRegister(ctx context.Context, conn *telnet.Conn, args []string) error {
	var username string
	if len(args) > 0 {
		username = args[0]
	}

	if username == "" {
		_ = conn.WritePrompt(telnet.Colorize(telnet.White, "Username: "))
		var err error
		username, err = conn.ReadLine()
		if err != nil {
			return fmt.Errorf("reading username: %w", err)
		}
		username = strings.TrimSpace(username)
	}

	if len(username) < 3 || len(username) > 32 {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Username must be 3-32 characters."))
		return nil
	}

	_ = conn.WritePrompt(telnet.Colorize(telnet.White, "Password: "))
	password, err := conn.ReadPassword()
	if err != nil {
		return fmt.Errorf("reading password: %w", err)
	}
	if len(password) < 6 {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Password must be at least 6 characters."))
		return nil
	}

	_ = conn.WritePrompt(telnet.Colorize(telnet.White, "Confirm password: "))
	confirm, err := conn.ReadPassword()
	if err != nil {
		return fmt.Errorf("reading password confirmation: %w", err)
	}
	if password != confirm {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Passwords do not match."))
		return nil
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
		telnet.Colorize(telnet.Green, "  login [username]") + "    — Log in to your account\r\n" +
		telnet.Colorize(telnet.Green, "  register [username]") + " — Create a new account\r\n" +
		telnet.Colorize(telnet.Green, "  help") + "                — Show this help\r\n" +
		telnet.Colorize(telnet.Green, "  quit") + "                — Disconnect\r\n"
	_ = conn.Write([]byte(help))
}

