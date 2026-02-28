package handlers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/cory-johannsen/mud/internal/frontend/telnet"
	"github.com/cory-johannsen/mud/internal/game/character"
	"github.com/cory-johannsen/mud/internal/game/ruleset"
	"github.com/cory-johannsen/mud/internal/storage/postgres"
)

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
				FormatCharacterSummary(c)))
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
	_ = conn.WritePrompt(telnet.Colorize(telnet.BrightWhite, "Enter your character's name: "))
	nameLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading character name: %w", err)
	}
	nameLine = strings.TrimSpace(nameLine)
	if strings.ToLower(nameLine) == "cancel" {
		return nil, nil
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
	_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select region [1-%d]: ", len(regions)))
	regionLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading region selection: %w", err)
	}
	regionLine = strings.TrimSpace(regionLine)
	if strings.ToLower(regionLine) == "cancel" {
		return nil, nil
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
	_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select team [1-%d]: ", len(teams)))
	teamLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading team selection: %w", err)
	}
	teamLine = strings.TrimSpace(teamLine)
	if strings.ToLower(teamLine) == "cancel" {
		return nil, nil
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
	_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow, "\r\nChoose your job (%s jobs available):", selectedTeam.Name))
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
	_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select job [1-%d]: ", len(availableJobs)))
	jobLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading job selection: %w", err)
	}
	jobLine = strings.TrimSpace(jobLine)
	if strings.ToLower(jobLine) == "cancel" {
		return nil, nil
	}
	jobChoice := 0
	if _, err := fmt.Sscanf(jobLine, "%d", &jobChoice); err != nil || jobChoice < 1 || jobChoice > len(availableJobs) {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
		return nil, nil
	}
	selectedJob := availableJobs[jobChoice-1]

	// Step 5: Preview + confirm
	newChar, err := character.BuildWithJob(charName, selectedRegion, selectedJob, selectedTeam)
	if err != nil {
		_ = conn.WriteLine(telnet.Colorf(telnet.Red, "Error building character: %v", err))
		return nil, nil
	}

	_ = conn.WriteLine(telnet.Colorize(telnet.BrightCyan, "\r\n--- Character Preview ---"))
	_ = conn.WriteLine(FormatCharacterStats(newChar))
	_ = conn.WritePrompt(telnet.Colorize(telnet.BrightWhite, "Create this character? [y/N]: "))

	confirm, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading confirmation: %w", err)
	}
	if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
		_ = conn.WriteLine(telnet.Colorize(telnet.Yellow, "Character creation cancelled."))
		return nil, nil
	}

	// Step 6: Persist
	newChar.AccountID = accountID
	start := time.Now()
	created, err := h.characters.Create(ctx, newChar)
	if err != nil {
		h.logger.Error("creating character", zap.String("name", newChar.Name), zap.Error(err))
		_ = conn.WriteLine(telnet.Colorf(telnet.Red, "Failed to create character: %v", err))
		return nil, nil
	}
	_ = conn.WriteLine(telnet.Colorf(telnet.BrightGreen,
		"Character %s created! [%s]", created.Name, time.Since(start)))
	return created, nil
}

// FormatCharacterSummary returns a one-line summary of a character for the selection list.
// Exported for testing.
//
// Precondition: c must be non-nil.
// Postcondition: Returns a non-empty human-readable string.
func FormatCharacterSummary(c *character.Character) string {
	return fmt.Sprintf("%s%s%s — Lvl %d %s from %s",
		telnet.BrightWhite, c.Name, telnet.Reset,
		c.Level, c.Class, c.Region)
}

// FormatCharacterStats returns a multi-line stats block for the character preview.
// Exported for testing.
//
// Precondition: c must be non-nil.
// Postcondition: Returns a formatted multi-line string with HP and all six ability scores.
func FormatCharacterStats(c *character.Character) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("  Name:   %s%s%s\r\n", telnet.BrightWhite, c.Name, telnet.Reset))
	sb.WriteString(fmt.Sprintf("  Region: %s   Class: %s   Level: %d\r\n", c.Region, c.Class, c.Level))
	sb.WriteString(fmt.Sprintf("  HP:     %d/%d\r\n", c.CurrentHP, c.MaxHP))
	sb.WriteString(fmt.Sprintf("  BRT:%2d  QCK:%2d  GRT:%2d  RSN:%2d  SAV:%2d  FLR:%2d\r\n",
		c.Abilities.Brutality, c.Abilities.Quickness, c.Abilities.Grit,
		c.Abilities.Reasoning, c.Abilities.Savvy, c.Abilities.Flair))
	return sb.String()
}
