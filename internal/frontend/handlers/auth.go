// Package handlers provides Telnet session handling and command processing.
package handlers

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"
	"sync"
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
	DeleteByAccountAndName(ctx context.Context, accountID int64, name string) error
}

//go:embed splash/ak47.txt
var ak47ArtFile string

//go:embed splash/machete.txt
var macheteArtFile string

// artRows splits an embedded art file into non-empty lines, stripping the
// trailing newline that text editors append after the last row.
// Precondition: s is a newline-delimited string.
// Postcondition: returns a slice of raw art rows with no trailing empty entry.
func artRows(s string) []string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines
}

// buildWelcomeBanner returns the connection banner with the current version embedded.
//
// Layout (top to bottom):
//  1. Horizontal AK-47 ASCII art (BrightGreen) — loaded from splash/ak47.txt
//  2. GUNCHETE Unicode block-letter title (Bold + BrightCyan, per-row)
//  3. Horizontal machete ASCII art (BrightYellow) — loaded from splash/machete.txt
//  4. Subtitle, version, instructions (unchanged)
//
// Each row is independently wrapped: color + row + Reset.
// For title rows: Bold + BrightCyan + row + Reset (single Reset clears both).
// Precondition: none.
// Postcondition: returns a complete, non-empty banner string.
func buildWelcomeBanner() string {
	// Weapon art loaded from embedded files — edit splash/ak47.txt or
	// splash/machete.txt and rebuild to update.
	ak47 := artRows(ak47ArtFile)
	machete := artRows(macheteArtFile)

	// GUNCHETE Unicode block-letter title — original art, per-row.
	// Each row wrapped independently so TestBannerContainsBrightCyanAsciiArt
	// counts ≥ 4 distinct BrightCyan segments.
	// Width measured in runes (TestBannerLineWidthMax80 uses utf8.RuneCountInString).
	title := []string{
		`  ██████╗ ██╗   ██╗███╗   ██╗ ██████╗██╗  ██╗███████╗████████╗███████╗`,
		` ██╔════╝ ██║   ██║████╗  ██║██╔════╝██║  ██║██╔════╝╚══██╔══╝██╔════╝`,
		` ██║  ███╗██║   ██║██╔██╗ ██║██║     ███████║█████╗     ██║   █████╗`,
		` ██║   ██║██║   ██║██║╚██╗██║██║     ██╔══██║██╔══╝     ██║   ██╔══╝`,
		` ╚██████╔╝╚██████╔╝██║ ╚████║╚██████╗██║  ██║███████╗   ██║   ███████╗`,
		`  ╚═════╝  ╚═════╝ ╚═╝  ╚═══╝ ╚═════╝╚═╝  ╚═╝╚══════╝   ╚═╝   ╚══════╝`,
	}

	var sb strings.Builder
	sb.WriteString("\n")

	// AK-47 block: each row independently colorized.
	for _, row := range ak47 {
		sb.WriteString(telnet.BrightGreen + row + telnet.Reset + "\n")
	}

	sb.WriteString("\n")

	// Title block: Bold + BrightCyan per row. Single Reset clears both attributes.
	for _, row := range title {
		sb.WriteString(telnet.Bold + telnet.BrightCyan + row + telnet.Reset + "\n")
	}

	sb.WriteString("\n")

	// Machete block: each row independently colorized.
	for _, row := range machete {
		sb.WriteString(telnet.BrightYellow + row + telnet.Reset + "\n")
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
	GetAll(ctx context.Context, characterID int64) ([]string, error)
	SetAll(ctx context.Context, characterID int64, featureIDs []string) error
}

// AuthHandler implements telnet.SessionHandler and processes the
// authentication loop for a connected client.
// maxFailedLogins is the number of consecutive failed login attempts before an account is locked out.
const maxFailedLogins = 5

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
	// loginFailures tracks consecutive failed password attempts per username.
	loginFailuresMu sync.Mutex
	loginFailures   map[string]int
	// seedAuthorized is the set of usernames permitted to authenticate over
	// the headless / debug surface (REQ-TD-3b — telnet-deprecation #325).
	// These match the accounts created by cmd/seed-claude-accounts.
	seedAuthorized map[string]struct{}
}

// SeedAuthorizedAccounts is the canonical list of usernames the headless
// surface treats as seed-authorized. Mirror of the set created by the
// seed-claude-accounts CLI tool.
var SeedAuthorizedAccounts = []string{
	"claude_player",
	"claude_editor",
	"claude_admin",
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
	seedSet := make(map[string]struct{}, len(SeedAuthorizedAccounts))
	for _, name := range SeedAuthorizedAccounts {
		seedSet[name] = struct{}{}
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
		loginFailures:   make(map[string]int),
		seedAuthorized:  seedSet,
	}
}

// SetSeedAuthorized replaces the set of usernames permitted on the headless
// surface. Test-only. Postcondition: subsequent headless login attempts use
// the new set.
func (h *AuthHandler) SetSeedAuthorized(usernames []string) {
	set := make(map[string]struct{}, len(usernames))
	for _, n := range usernames {
		set[n] = struct{}{}
	}
	h.seedAuthorized = set
}

// isSeedAuthorized reports whether the given username is in the seed-bootstrap
// list (REQ-TD-3b). Returns true when no set is configured (graceful fallback
// for unit-test mocks that bypass NewAuthHandler).
func (h *AuthHandler) isSeedAuthorized(username string) bool {
	if h.seedAuthorized == nil {
		return true
	}
	_, ok := h.seedAuthorized[username]
	return ok
}

// HandleSession implements telnet.SessionHandler. It shows the welcome banner
// and processes authentication commands until the player logs in or quits.
//
// In headless mode (conn.Headless == true) the banner and command loop are
// skipped; the session goes directly to username/password prompts so that
// automated clients can authenticate without sending a "login" command first.
//
// Telnet-deprecation gate (#325): when conn is not headless and the operator
// has not opted into the legacy player flow via telnet.allow_game_commands,
// the auth handler refuses login attempts with a redirect-to-web-client
// narrative. This is a belt-and-suspenders gate; the acceptor layer SHOULD
// already have substituted the Rejector handler for the player port.
//
// Postcondition: Returns nil on clean quit, or an error if the session ended abnormally.
func (h *AuthHandler) HandleSession(ctx context.Context, conn *telnet.Conn) error {
	if conn.Headless {
		return h.handleHeadlessSession(ctx, conn)
	}

	if !h.telnetCfg.AllowGameCommands {
		// REQ-TD-2a / REQ-TD-2b: telnet player surface retired.
		_ = conn.WriteLine(telnet.Colorize(telnet.Yellow,
			"The telnet player surface has been retired."))
		_ = conn.WriteLine(telnet.Colorize(telnet.BrightWhite,
			"Please use the web client for gameplay; this port now serves debug only."))
		return nil
	}

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

// handleHeadlessSession is the headless-mode entry point for HandleSession.
// It skips the welcome banner and command loop, going directly to username/
// password prompts. Sending "quit" or "exit" as the username cleanly
// disconnects with a "Goodbye!" message.
//
// Role-specific post-auth behavior:
//   - editor/admin: auto-selects or auto-creates a character then enters the
//     game directly so automated clients can issue game commands immediately.
//   - player: emits a bare "> " confirmation line first (allowing automated
//     clients to sync), then runs the normal character-selection flow.
//
// Postcondition: Returns nil on clean quit, or an error on fatal connection failure.
func (h *AuthHandler) handleHeadlessSession(ctx context.Context, conn *telnet.Conn) error {
	for {
		select {
		case <-ctx.Done():
			_ = conn.WriteLine(telnet.Colorize(telnet.Yellow, "Server shutting down. Goodbye!"))
			return ctx.Err()
		default:
		}

		// Prompt for username directly — no banner, no command loop.
		_ = conn.WritePrompt(telnet.Colorize(telnet.White, "Username: "))
		username, err := conn.ReadLine()
		if err != nil {
			return fmt.Errorf("reading username: %w", err)
		}
		username = strings.TrimSpace(username)

		cmd := strings.ToLower(username)
		if cmd == "quit" || cmd == "exit" {
			_ = conn.WriteLine(telnet.Colorize(telnet.Cyan, "Goodbye!"))
			return nil
		}

		if username == "" {
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Username cannot be empty."))
			continue
		}

		// REQ-TD-3b: only seed-bootstrapped accounts may authenticate over
		// the headless / debug surface. Reject other usernames before any
		// password prompt is shown so unauthorized callers learn nothing
		// about account existence.
		if !h.isSeedAuthorized(username) {
			_ = conn.WriteLine("not seed-authorized")
			h.logger.Info("rejected non-seeded headless login",
				zap.String("username", username),
				zap.String("remote_addr", conn.RemoteAddr().String()),
			)
			return nil
		}

		// Pass username as arg so handleLogin skips its own username prompt
		// and goes directly to the password prompt.
		acct, err := h.handleLogin(ctx, conn, []string{username})
		if err != nil {
			return err
		}
		if acct.ID == 0 {
			// Authentication failed — retry.
			continue
		}

		// Route post-auth flow by role.
		if acct.Role == "editor" || acct.Role == "admin" {
			return h.handleHeadlessEditorSession(ctx, conn, acct)
		}
		return h.handleHeadlessPlayerSession(ctx, conn, acct)
	}
}

// handleHeadlessEditorSession handles post-auth flow for editor/admin accounts in
// headless mode. It auto-creates a minimal character if the account has none, then
// enters the game directly (bypassing interactive ensure* prompts) so that automated
// clients can issue game commands immediately after loginAs returns.
//
// Precondition: acct must have role editor or admin.
// Postcondition: Returns nil on clean disconnect; non-nil error on fatal failure.
func (h *AuthHandler) handleHeadlessEditorSession(ctx context.Context, conn *telnet.Conn, acct postgres.Account) error {
	chars, err := h.characters.ListByAccount(ctx, acct.ID)
	if err != nil {
		return fmt.Errorf("listing characters for headless editor: %w", err)
	}

	var selected *character.Character
	if len(chars) == 0 {
		// Auto-create a minimal character so the editor can enter the game.
		c := &character.Character{
			AccountID: acct.ID,
			Name:      "HeadlessEditor",
			Region:    "northeast",
			Class:     "gunslinger",
			Location:  "battle_infirmary",
			Level:     1,
			MaxHP:     20,
			CurrentHP: 20,
			Gender:    "they/them",
			Abilities: character.AbilityScores{
				Brutality: 10, Quickness: 10, Grit: 10,
				Reasoning: 10, Savvy: 10, Flair: 10,
			},
		}
		created, createErr := h.characters.Create(ctx, c)
		if createErr != nil {
			// If creation fails (e.g., duplicate name from a prior run), try listing again.
			chars, err = h.characters.ListByAccount(ctx, acct.ID)
			if err != nil || len(chars) == 0 {
				return fmt.Errorf("auto-creating headless editor character: %w", createErr)
			}
			selected = chars[0]
		} else {
			selected = created
		}
	} else {
		selected = chars[0]
	}

	// Pre-assign class features so the game server's promptFeatureChoice is skipped.
	if err := h.ensureClassFeatures(ctx, conn, selected); err != nil {
		return fmt.Errorf("assigning class features for headless editor: %w", err)
	}

	// Enter game directly — skip interactive ensure* prompts in headless mode.
	return h.gameBridge(ctx, conn, acct, selected)
}

// handleHeadlessPlayerSession handles post-auth flow for player accounts in
// headless mode. It emits a bare "> " sync line immediately so that automated
// clients (loginAs) can confirm authentication, then runs the normal character
// selection flow. The character list follows the "> " sync line so that
// selectCharacter can find character entries in the output buffer.
//
// Precondition: acct must have role player.
// Postcondition: Returns nil on clean disconnect; non-nil error on fatal failure.
func (h *AuthHandler) handleHeadlessPlayerSession(ctx context.Context, conn *telnet.Conn, acct postgres.Account) error {
	// Send the auth-sync marker before the character list so loginAs can
	// match "> " and return before selectCharacter needs the list.
	_ = conn.WriteLine("> ")
	return h.characterFlow(ctx, conn, acct)
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
			h.loginFailuresMu.Lock()
			h.loginFailures[username]++
			failures := h.loginFailures[username]
			h.loginFailuresMu.Unlock()
			if failures >= maxFailedLogins {
				_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Too many failed login attempts. Account temporarily locked."))
			} else {
				_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid password."))
			}
			return postgres.Account{}, nil
		default:
			h.logger.Error("authentication error", zap.Error(err), zap.Duration("elapsed", elapsed))
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "An internal error occurred. Please try again."))
			return postgres.Account{}, nil
		}
	}

	// Correct password: reset failure counter regardless of lockout state.
	h.loginFailuresMu.Lock()
	delete(h.loginFailures, username)
	h.loginFailuresMu.Unlock()

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

