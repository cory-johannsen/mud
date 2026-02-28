package handlers

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

// RandomNames is a list of post-apocalyptic themed character names suitable for
// random selection during character creation. All names are 2-32 characters and
// are not equal to "cancel" or "random" (case-insensitive).
var RandomNames = []string{
	"Raze", "Vex", "Cinder", "Sable", "Grit",
	"Ash", "Flint", "Thorn", "Kael", "Dusk",
	"Riven", "Scar", "Nox", "Wren", "Jace",
	"Brix", "Colt", "Ember", "Slate", "Pike",
}

// RandomizeRemaining selects a region, team, and job for character creation,
// using fixed values where provided and random selection otherwise.
//
// Precondition: regions, teams, and allJobs must each be non-empty; returns error if any is empty or no compatible jobs exist.
// Postcondition: returned job.Team is "" or equals returned team.ID; err is non-nil if no compatible jobs exist.
func RandomizeRemaining(
	regions []*ruleset.Region, fixedRegion *ruleset.Region,
	teams []*ruleset.Team, fixedTeam *ruleset.Team,
	allJobs []*ruleset.Job,
) (region *ruleset.Region, team *ruleset.Team, job *ruleset.Job, err error) {
	if len(regions) == 0 {
		err = fmt.Errorf("RandomizeRemaining: regions must be non-empty")
		return
	}
	if len(teams) == 0 {
		err = fmt.Errorf("RandomizeRemaining: teams must be non-empty")
		return
	}
	if len(allJobs) == 0 {
		err = fmt.Errorf("RandomizeRemaining: allJobs must be non-empty")
		return
	}
	if fixedRegion != nil {
		region = fixedRegion
	} else {
		region = regions[rand.Intn(len(regions))]
	}

	if fixedTeam != nil {
		team = fixedTeam
	} else {
		team = teams[rand.Intn(len(teams))]
	}

	var compatible []*ruleset.Job
	for _, j := range allJobs {
		if j.Team == "" || j.Team == team.ID {
			compatible = append(compatible, j)
		}
	}
	if len(compatible) == 0 {
		err = fmt.Errorf("RandomizeRemaining: no jobs compatible with team %q", team.ID)
		return
	}
	job = compatible[rand.Intn(len(compatible))]
	return region, team, job, nil
}

// IsRandomInput reports whether the player's input at a list step requests random selection.
// Blank input, "r", and "random" (all case-insensitive) are treated as random.
// Exported for testing.
func IsRandomInput(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	return lower == "" || lower == "r" || lower == "random"
}

// characterFlow runs the character selection/creation UI after login.
// It exits by calling gameBridge with the selected or newly created character.
//
// Precondition: acct.ID must be > 0; conn must be open.
// Postcondition: Calls gameBridge on success; returns non-nil error on fatal failure.
func (h *AuthHandler) characterFlow(ctx context.Context, conn *telnet.Conn, acct postgres.Account) error {
	for {
		chars, err := h.characters.ListByAccount(ctx, acct.ID)
		if err != nil {
			return fmt.Errorf("listing characters: %w", err)
		}

		if len(chars) == 0 {
			_ = conn.WriteLine(telnet.Colorize(telnet.BrightYellow,
				"\r\nYou have no characters. Let's create one."))
			c, err := h.characterCreationFlow(ctx, conn, acct.ID)
			if err != nil {
				return err
			}
			if c == nil {
				continue // user cancelled — loop again
			}
			return h.gameBridge(ctx, conn, acct, c)
		}

		// Show character list
		_ = conn.WriteLine(telnet.Colorize(telnet.BrightWhite, "\r\nYour characters:"))
		for i, c := range chars {
			_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s",
				telnet.Green, i+1, telnet.Reset,
				FormatCharacterSummary(c, h.regionDisplayName(c.Region))))
		}
		_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. Create a new character",
			telnet.Green, len(chars)+1, telnet.Reset))
		_ = conn.WriteLine(fmt.Sprintf("  %squit%s. Disconnect",
			telnet.Green, telnet.Reset))

		_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select [1-%d]: ", len(chars)+1))
		line, err := conn.ReadLine()
		if err != nil {
			return fmt.Errorf("reading character selection: %w", err)
		}
		line = strings.TrimSpace(line)

		if strings.ToLower(line) == "quit" || strings.ToLower(line) == "exit" {
			_ = conn.WriteLine(telnet.Colorize(telnet.Cyan, "Goodbye."))
			return nil
		}

		choice := 0
		if _, err := fmt.Sscanf(line, "%d", &choice); err != nil || choice < 1 || choice > len(chars)+1 {
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
			continue
		}

		if choice == len(chars)+1 {
			c, err := h.characterCreationFlow(ctx, conn, acct.ID)
			if err != nil {
				return err
			}
			if c != nil {
				return h.gameBridge(ctx, conn, acct, c)
			}
			continue
		}

		selected := chars[choice-1]
		return h.gameBridge(ctx, conn, acct, selected)
	}
}

// characterCreationFlow guides the player through the interactive character builder.
// Returns (nil, nil) if the player cancels at any step.
//
// Precondition: accountID must be > 0; h.regions must be non-empty.
// Postcondition: Returns a persisted *character.Character or (nil, nil) on cancel.
func (h *AuthHandler) characterCreationFlow(ctx context.Context, conn *telnet.Conn, accountID int64) (*character.Character, error) {
	_ = conn.WriteLine(telnet.Colorize(telnet.BrightCyan, "\r\n=== Character Creation ==="))
	_ = conn.WriteLine("Type 'cancel' at any prompt to return to the character screen.\r\n")

	// Step 1: Character name
	_ = conn.WritePrompt(telnet.Colorize(telnet.BrightWhite,
		"Enter your character's name (or 'random'): "))
	nameLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading character name: %w", err)
	}
	nameLine = strings.TrimSpace(nameLine)
	if strings.ToLower(nameLine) == "cancel" {
		return nil, nil
	}
	if strings.ToLower(nameLine) == "random" {
		nameLine = RandomNames[rand.Intn(len(RandomNames))]
		_ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "Random name selected: %s", nameLine))
	}
	if len(nameLine) < 2 || len(nameLine) > 32 {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Name must be 2-32 characters."))
		return nil, nil
	}
	charName := nameLine

	// Step 2: Home region
	regions := h.regions
	_ = conn.WriteLine(telnet.Colorize(telnet.BrightYellow, "\r\nChoose your home region:"))
	for i, r := range regions {
		_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s\r\n     %s",
			telnet.Green, i+1, telnet.Reset,
			telnet.BrightWhite, r.Name, telnet.Reset,
			r.Description))
	}
	_ = conn.WriteLine(fmt.Sprintf("  %sR%s. Random (default)", telnet.Green, telnet.Reset))
	_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite,
		"Select region [1-%d/R, default=R]: ", len(regions)))
	regionLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading region selection: %w", err)
	}
	regionLine = strings.TrimSpace(regionLine)
	if strings.ToLower(regionLine) == "cancel" {
		return nil, nil
	}
	if IsRandomInput(regionLine) {
		region, team, job, err := RandomizeRemaining(regions, nil, h.teams, nil, h.jobs)
		if err != nil {
			h.logger.Error("randomizing character selections", zap.Error(err))
			_ = conn.WriteLine(telnet.Colorf(telnet.Red, "Error randomizing selections: %v", err))
			return nil, nil
		}
		_ = conn.WriteLine(telnet.Colorf(telnet.Cyan,
			"Random selections: Region=%s, Team=%s, Job=%s", region.Name, team.Name, job.Name))
		return h.buildAndConfirm(ctx, conn, accountID, charName, region, job, team)
	}
	regionChoice := 0
	if _, err := fmt.Sscanf(regionLine, "%d", &regionChoice); err != nil || regionChoice < 1 || regionChoice > len(regions) {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
		return nil, nil
	}
	selectedRegion := regions[regionChoice-1]

	// Step 3: Team selection
	teams := h.teams
	_ = conn.WriteLine(telnet.Colorize(telnet.BrightYellow, "\r\nChoose your team:"))
	for i, t := range teams {
		_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s\r\n     %s",
			telnet.Green, i+1, telnet.Reset,
			telnet.BrightWhite, t.Name, telnet.Reset,
			t.Description))
		for _, trait := range t.Traits {
			_ = conn.WriteLine(fmt.Sprintf("     %s[%s]%s %s",
				telnet.Yellow, trait.Name, telnet.Reset, trait.Effect))
		}
	}
	_ = conn.WriteLine(fmt.Sprintf("  %sR%s. Random (default)", telnet.Green, telnet.Reset))
	_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite,
		"Select team [1-%d/R, default=R]: ", len(teams)))
	teamLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading team selection: %w", err)
	}
	teamLine = strings.TrimSpace(teamLine)
	if strings.ToLower(teamLine) == "cancel" {
		return nil, nil
	}
	if IsRandomInput(teamLine) {
		_, team, job, err := RandomizeRemaining(regions, selectedRegion, teams, nil, h.jobs)
		if err != nil {
			h.logger.Error("randomizing team/job selections", zap.Error(err))
			_ = conn.WriteLine(telnet.Colorf(telnet.Red, "Error randomizing selections: %v", err))
			return nil, nil
		}
		_ = conn.WriteLine(telnet.Colorf(telnet.Cyan,
			"Random selections: Team=%s, Job=%s", team.Name, job.Name))
		return h.buildAndConfirm(ctx, conn, accountID, charName, selectedRegion, job, team)
	}
	teamChoice := 0
	if _, err := fmt.Sscanf(teamLine, "%d", &teamChoice); err != nil || teamChoice < 1 || teamChoice > len(teams) {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
		return nil, nil
	}
	selectedTeam := teams[teamChoice-1]

	// Step 4: Job selection — show jobs available to this team (general + team-exclusive)
	var availableJobs []*ruleset.Job
	for _, j := range h.jobs {
		if j.Team == "" || j.Team == selectedTeam.ID {
			availableJobs = append(availableJobs, j)
		}
	}
	if len(availableJobs) == 0 {
		h.logger.Error("no jobs available for team", zap.String("team", selectedTeam.ID))
		_ = conn.WriteLine(telnet.Colorf(telnet.Red, "No jobs available for team %s.", selectedTeam.Name))
		return nil, nil
	}
	_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow,
		"\r\nChoose your job (%s jobs available):", selectedTeam.Name))
	for i, j := range availableJobs {
		exclusive := ""
		if j.Team != "" {
			exclusive = telnet.Colorf(telnet.BrightRed, " [%s exclusive]", selectedTeam.Name)
		}
		_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s%s (HP/lvl: %d, Key: %s)\r\n     %s",
			telnet.Green, i+1, telnet.Reset,
			telnet.BrightWhite, j.Name, telnet.Reset, exclusive,
			j.HitPointsPerLevel, j.KeyAbility,
			j.Description))
	}
	_ = conn.WriteLine(fmt.Sprintf("  %sR%s. Random (default)", telnet.Green, telnet.Reset))
	_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite,
		"Select job [1-%d/R, default=R]: ", len(availableJobs)))
	jobLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading job selection: %w", err)
	}
	jobLine = strings.TrimSpace(jobLine)
	if strings.ToLower(jobLine) == "cancel" {
		return nil, nil
	}
	var selectedJob *ruleset.Job
	if IsRandomInput(jobLine) {
		selectedJob = availableJobs[rand.Intn(len(availableJobs))]
		_ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "Random job selected: %s", selectedJob.Name))
	} else {
		jobChoice := 0
		if _, err := fmt.Sscanf(jobLine, "%d", &jobChoice); err != nil || jobChoice < 1 || jobChoice > len(availableJobs) {
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
			return nil, nil
		}
		selectedJob = availableJobs[jobChoice-1]
	}

	return h.buildAndConfirm(ctx, conn, accountID, charName, selectedRegion, selectedJob, selectedTeam)
}

// buildAndConfirm builds a character from the given selections, shows the preview,
// prompts for confirmation, and persists on yes.
// Returns (nil, nil) if the player declines or cancels.
//
// Precondition: all pointer parameters must be non-nil; accountID must be > 0.
// Postcondition: returns persisted *character.Character or (nil, nil) on cancel, decline, build failure, or storage failure.
func (h *AuthHandler) buildAndConfirm(
	ctx context.Context,
	conn *telnet.Conn,
	accountID int64,
	charName string,
	region *ruleset.Region,
	job *ruleset.Job,
	team *ruleset.Team,
) (*character.Character, error) {
	newChar, err := character.BuildWithJob(charName, region, job, team)
	if err != nil {
		h.logger.Error("building character", zap.String("name", charName), zap.Error(err))
		_ = conn.WriteLine(telnet.Colorf(telnet.Red, "Error building character: %v", err))
		return nil, nil
	}

	_ = conn.WriteLine(telnet.Colorize(telnet.BrightCyan, "\r\n--- Character Preview ---"))
	_ = conn.WriteLine(FormatCharacterStats(newChar, region.DisplayName()))
	_ = conn.WritePrompt(telnet.Colorize(telnet.BrightWhite, "Create this character? [y/N]: "))

	confirm, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading confirmation: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
		_ = conn.WriteLine(telnet.Colorize(telnet.Yellow, "Character creation cancelled."))
		return nil, nil
	}

	newChar.AccountID = accountID
	start := time.Now()
	created, err := h.characters.Create(ctx, newChar)
	if err != nil {
		h.logger.Error("creating character", zap.String("name", newChar.Name), zap.Error(err))
		_ = conn.WriteLine(telnet.Colorf(telnet.Red, "Failed to create character: %v", err))
		return nil, nil
	}
	elapsed := time.Since(start)
	h.logger.Info("character created",
		zap.String("name", created.Name),
		zap.Int64("account_id", accountID),
		zap.Duration("duration", elapsed))
	_ = conn.WriteLine(telnet.Colorf(telnet.BrightGreen,
		"Character %s created! [%s]", created.Name, elapsed))
	return created, nil
}

// regionDisplayName returns the DisplayName for the region with the given id, or id itself if not found.
//
// Precondition: id must be non-empty.
// Postcondition: Returns a non-empty string.
func (h *AuthHandler) regionDisplayName(id string) string {
	for _, r := range h.regions {
		if r.ID == id {
			return r.DisplayName()
		}
	}
	return id
}

// FormatCharacterSummary returns a one-line summary of a character for the selection list.
// Exported for testing.
//
// Precondition: c must be non-nil; regionDisplay must be non-empty.
// Postcondition: Returns a non-empty human-readable string.
func FormatCharacterSummary(c *character.Character, regionDisplay string) string {
	return fmt.Sprintf("%s%s%s — Lvl %d %s from %s",
		telnet.BrightWhite, c.Name, telnet.Reset,
		c.Level, c.Class, regionDisplay)
}

// FormatCharacterStats returns a multi-line stats block for the character preview.
// Exported for testing.
//
// Precondition: c must be non-nil; regionDisplay must be non-empty.
// Postcondition: Returns a formatted multi-line string with HP and all six ability scores.
func FormatCharacterStats(c *character.Character, regionDisplay string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Name:   %s%s%s\r\n", telnet.BrightWhite, c.Name, telnet.Reset))
	sb.WriteString(fmt.Sprintf("  Region: %s   Class: %s   Level: %d\r\n", regionDisplay, c.Class, c.Level))
	sb.WriteString(fmt.Sprintf("  HP:     %d/%d\r\n", c.CurrentHP, c.MaxHP))
	sb.WriteString(fmt.Sprintf("  BRT:%2d  QCK:%2d  GRT:%2d  RSN:%2d  SAV:%2d  FLR:%2d\r\n",
		c.Abilities.Brutality, c.Abilities.Quickness, c.Abilities.Grit,
		c.Abilities.Reasoning, c.Abilities.Savvy, c.Abilities.Flair))
	return sb.String()
}
