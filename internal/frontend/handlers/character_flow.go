package handlers

import (
	"context"
	"errors"
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

// IsAlreadyLoggedIn returns true if err indicates the character is already connected.
//
// Precondition: err may be nil or non-nil.
// Postcondition: Returns true only when the error message contains "already connected".
func IsAlreadyLoggedIn(err error) bool {
	return err != nil && strings.Contains(err.Error(), "already connected")
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
			if err := h.ensureGender(ctx, conn, c); err != nil {
				return err
			}
			if err := h.ensureSkills(ctx, conn, c); err != nil {
				return err
			}
			if err := h.ensureFeats(ctx, conn, c); err != nil {
				return err
			}
			if err := h.ensureClassFeatures(ctx, conn, c); err != nil {
				return err
			}
			if err := h.gameBridge(ctx, conn, acct, c); err != nil {
				if errors.Is(err, ErrSwitchCharacter) {
					continue
				}
				if IsAlreadyLoggedIn(err) {
					_ = conn.WriteLine(telnet.Colorize(telnet.Red, "That character is already logged in."))
					continue
				}
				return err
			}
			return nil
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
		_ = conn.WriteLine(telnet.Colorize(telnet.Yellow, "  Type 'delete N' to permanently delete character N."))

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

		// Handle "delete N" command.
		if lower := strings.ToLower(line); strings.HasPrefix(lower, "delete ") {
			parts := strings.Fields(line)
			if len(parts) == 2 {
				var delIdx int
				if _, scanErr := fmt.Sscan(parts[1], &delIdx); scanErr == nil && delIdx >= 1 && delIdx <= len(chars) {
					target := chars[delIdx-1]
					_ = conn.WritePrompt(telnet.Colorf(telnet.Red,
						"Delete %q? This cannot be undone. Type 'yes' to confirm: ", target.Name))
					confirm, readErr := conn.ReadLine()
					if readErr != nil {
						return fmt.Errorf("reading delete confirmation: %w", readErr)
					}
					if strings.TrimSpace(strings.ToLower(confirm)) == "yes" {
						if delErr := h.characters.DeleteByAccountAndName(ctx, acct.ID, target.Name); delErr != nil {
							_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Failed to delete character."))
						} else {
							_ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "%q has been deleted.", target.Name))
						}
					} else {
						_ = conn.WriteLine(telnet.Colorize(telnet.Yellow, "Delete cancelled."))
					}
				} else {
					_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection for delete."))
				}
			} else {
				_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Usage: delete N"))
			}
			continue
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
				if err := h.ensureGender(ctx, conn, c); err != nil {
					return err
				}
				if err := h.ensureSkills(ctx, conn, c); err != nil {
					return err
				}
				if err := h.ensureFeats(ctx, conn, c); err != nil {
					return err
				}
				if err := h.ensureClassFeatures(ctx, conn, c); err != nil {
					return err
				}
				if err := h.gameBridge(ctx, conn, acct, c); err != nil {
					if errors.Is(err, ErrSwitchCharacter) {
						continue
					}
					if IsAlreadyLoggedIn(err) {
						_ = conn.WriteLine(telnet.Colorize(telnet.Red, "That character is already logged in."))
						continue
					}
					return err
				}
				return nil
			}
			continue
		}

		selected := chars[choice-1]
		if err := h.ensureGender(ctx, conn, selected); err != nil {
			return err
		}
		if err := h.ensureSkills(ctx, conn, selected); err != nil {
			return err
		}
		if err := h.ensureFeats(ctx, conn, selected); err != nil {
			return err
		}
		if err := h.ensureClassFeatures(ctx, conn, selected); err != nil {
			return err
		}
		if err := h.gameBridge(ctx, conn, acct, selected); err != nil {
			if errors.Is(err, ErrSwitchCharacter) {
				continue
			}
			if IsAlreadyLoggedIn(err) {
				_ = conn.WriteLine(telnet.Colorize(telnet.Red, "That character is already logged in."))
				continue
			}
			return err
		}
		return nil
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

	// Step 1b: Gender
	gender, err := PromptGenderStep(conn)
	if err != nil {
		return nil, fmt.Errorf("reading gender selection: %w", err)
	}
	if gender == "" {
		// player typed cancel
		return nil, nil
	}

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
			"Random selections: Region=%s, Team=%s, Archetype=%s, Job=%s", region.Name, team.Name, job.Archetype, job.Name))
		return h.buildAndConfirm(ctx, conn, accountID, charName, gender, region, job, team)
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
			"Random selections: Team=%s, Archetype=%s, Job=%s", team.Name, job.Archetype, job.Name))
		return h.buildAndConfirm(ctx, conn, accountID, charName, gender, selectedRegion, job, team)
	}
	teamChoice := 0
	if _, err := fmt.Sscanf(teamLine, "%d", &teamChoice); err != nil || teamChoice < 1 || teamChoice > len(teams) {
		_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
		return nil, nil
	}
	selectedTeam := teams[teamChoice-1]

	// Step 4: Archetype selection — show archetypes available for this team
	archetypeIDs := h.jobRegistry.ArchetypesForTeam(selectedTeam.ID)
	archetypeIDSet := make(map[string]bool, len(archetypeIDs))
	for _, id := range archetypeIDs {
		archetypeIDSet[id] = true
	}
	var availableArchetypes []*ruleset.Archetype
	for _, a := range h.archetypes {
		if archetypeIDSet[a.ID] {
			availableArchetypes = append(availableArchetypes, a)
		}
	}
	if len(availableArchetypes) == 0 {
		h.logger.Error("no archetypes available for team", zap.String("team", selectedTeam.ID))
		_ = conn.WriteLine(telnet.Colorf(telnet.Red, "No archetypes available for team %s.", selectedTeam.Name))
		return nil, nil
	}
	_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow, "\r\nChoose your archetype (%s):", selectedTeam.Name))
	_ = conn.Write([]byte(RenderArchetypeMenu(availableArchetypes)))
	_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select archetype [1-%d/R, default=R]: ", len(availableArchetypes)))
	archetypeLine, err := conn.ReadLine()
	if err != nil {
		return nil, fmt.Errorf("reading archetype selection: %w", err)
	}
	archetypeLine = strings.TrimSpace(archetypeLine)
	if strings.ToLower(archetypeLine) == "cancel" {
		return nil, nil
	}
	var selectedArchetype *ruleset.Archetype
	if IsRandomInput(archetypeLine) {
		selectedArchetype = availableArchetypes[rand.Intn(len(availableArchetypes))]
		_ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "Random archetype selected: %s", selectedArchetype.Name))
	} else {
		archetypeChoice := 0
		if _, err := fmt.Sscanf(archetypeLine, "%d", &archetypeChoice); err != nil || archetypeChoice < 1 || archetypeChoice > len(availableArchetypes) {
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection."))
			return nil, nil
		}
		selectedArchetype = availableArchetypes[archetypeChoice-1]
	}

	// Step 5: Job selection — show jobs available to this team and archetype
	availableJobs := h.jobRegistry.JobsForTeamAndArchetype(selectedTeam.ID, selectedArchetype.ID)
	if len(availableJobs) == 0 {
		h.logger.Error("no jobs available for team+archetype",
			zap.String("team", selectedTeam.ID),
			zap.String("archetype", selectedArchetype.ID))
		_ = conn.WriteLine(telnet.Colorf(telnet.Red, "No jobs available for %s / %s.", selectedTeam.Name, selectedArchetype.Name))
		return nil, nil
	}
	_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow,
		"\r\nChoose your job (%s / %s jobs available):", selectedTeam.Name, selectedArchetype.Name))
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

	return h.buildAndConfirm(ctx, conn, accountID, charName, gender, selectedRegion, selectedJob, selectedTeam)
}

// ensureSkills checks whether the character has skills recorded and, if not,
// runs the interactive selection and persists the result. It is called before
// gameBridge for both new and existing characters so backfill always prompts.
//
// In headless mode (conn.Headless) this function is a no-op: automated clients
// cannot respond to interactive prompts.
//
// Precondition: char must have a valid ID and Class set.
// Postcondition: character_skills rows exist for char; returns non-nil error only on fatal failure.
func (h *AuthHandler) ensureSkills(ctx context.Context, conn *telnet.Conn, char *character.Character) error {
	if conn.Headless {
		return nil
	}
	if h.characterSkills == nil || len(h.allSkills) == 0 {
		return nil
	}
	has, err := h.characterSkills.HasSkills(ctx, char.ID)
	if err != nil {
		h.logger.Warn("checking skills for character", zap.Int64("id", char.ID), zap.Error(err))
		return nil // non-fatal: enter game without skills
	}
	if has {
		return nil
	}

	job, ok := h.jobRegistry.Job(char.Class)
	if !ok {
		h.logger.Warn("unknown job for skill backfill", zap.String("class", char.Class))
		return nil
	}

	allSkillIDs := make([]string, len(h.allSkills))
	for i, sk := range h.allSkills {
		allSkillIDs[i] = sk.ID
	}

	var chosen []string
	if job.SkillGrants != nil && job.SkillGrants.Choices != nil && job.SkillGrants.Choices.Count > 0 {
		_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow,
			"\r\nYour character needs to select trained skills before entering the world."))
		chosen, err = h.skillChoiceLoop(ctx, conn, job.SkillGrants.Choices.Pool, job.SkillGrants.Choices.Count)
		if err != nil {
			return fmt.Errorf("skill backfill choice: %w", err)
		}
		// If cancelled, proceed with nil chosen — fixed skills still assigned.
	}

	skillMap := character.BuildSkillsFromJob(job, allSkillIDs, chosen)
	if err := h.characterSkills.SetAll(ctx, char.ID, skillMap); err != nil {
		h.logger.Error("persisting backfill skills", zap.Int64("id", char.ID), zap.Error(err))
		_ = conn.WriteLine(telnet.Colorf(telnet.Yellow, "Warning: skills could not be saved."))
	}
	return nil
}

// skillChoiceLoop prompts the player to pick `count` skills from the provided pool,
// one at a time. The pool is a list of skill IDs; h.allSkills is consulted for display names.
// Returns the chosen skill IDs or (nil, nil) if the player cancels.
//
// Precondition: count >= 1; pool must be non-empty and contain valid skill IDs present in h.allSkills.
// Postcondition: Returns exactly count chosen IDs, or (nil, nil) on cancel.
func (h *AuthHandler) skillChoiceLoop(ctx context.Context, conn *telnet.Conn, pool []string, count int) ([]string, error) {
	// Build name and description lookups from h.allSkills for display.
	nameFor := make(map[string]string, len(h.allSkills))
	descFor := make(map[string]string, len(h.allSkills))
	for _, sk := range h.allSkills {
		nameFor[sk.ID] = sk.Name
		descFor[sk.ID] = sk.Description
	}

	remaining := make([]string, len(pool))
	copy(remaining, pool)

	var chosen []string
	for len(chosen) < count && len(remaining) > 0 {
		left := count - len(chosen)
		_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow,
			"\r\nChoose a trained skill (%d remaining):", left))
		for i, id := range remaining {
			name := nameFor[id]
			if name == "" {
				name = id
			}
			desc := descFor[id]
			if desc != "" {
				_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%-14s%s - %s%s%s",
					telnet.Green, i+1, telnet.Reset,
					telnet.BrightWhite, name, telnet.Reset,
					telnet.Dim, desc, telnet.Reset))
			} else {
				_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s",
					telnet.Green, i+1, telnet.Reset,
					telnet.BrightWhite, name, telnet.Reset))
			}
		}
		_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select skill [1-%d]: ", len(remaining)))
		line, err := conn.ReadLine()
		if err != nil {
			return nil, fmt.Errorf("reading skill choice: %w", err)
		}
		line = strings.TrimSpace(line)
		if strings.ToLower(line) == "cancel" {
			return nil, nil
		}
		choice := 0
		if _, err := fmt.Sscanf(line, "%d", &choice); err != nil || choice < 1 || choice > len(remaining) {
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection. Please enter a number from the list."))
			continue
		}
		picked := remaining[choice-1]
		chosen = append(chosen, picked)
		// Remove chosen skill from remaining pool.
		remaining = append(remaining[:choice-1], remaining[choice:]...)
		_ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "Selected: %s", nameFor[picked]))
	}
	return chosen, nil
}

// featChoiceLoop prompts the player to pick `count` feats from pool, one at a time.
// pool is a list of feat IDs; h.featRegistry is consulted for display names and descriptions.
// Returns chosen feat IDs or (nil, nil) if the player cancels.
//
// Precondition: count >= 1; pool must be non-empty and contain valid feat IDs in h.featRegistry.
// Postcondition: Returns exactly count chosen IDs, or (nil, nil) on cancel.
func (h *AuthHandler) featChoiceLoop(ctx context.Context, conn *telnet.Conn, header string, pool []string, count int) ([]string, error) {
	remaining := make([]string, len(pool))
	copy(remaining, pool)

	var chosen []string
	for len(chosen) < count && len(remaining) > 0 {
		left := count - len(chosen)
		_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow, "\r\n%s (%d remaining):", header, left))
		for i, id := range remaining {
			name := id
			desc := ""
			if h.featRegistry != nil {
				if f, ok := h.featRegistry.Feat(id); ok {
					name = f.Name
					desc = f.Description
				}
			}
			if desc != "" {
				_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%-20s%s - %s%s%s",
					telnet.Green, i+1, telnet.Reset,
					telnet.BrightWhite, name, telnet.Reset,
					telnet.Dim, desc, telnet.Reset))
			} else {
				_ = conn.WriteLine(fmt.Sprintf("  %s%d%s. %s%s%s",
					telnet.Green, i+1, telnet.Reset,
					telnet.BrightWhite, name, telnet.Reset))
			}
		}
		_ = conn.WritePrompt(telnet.Colorf(telnet.BrightWhite, "Select feat [1-%d]: ", len(remaining)))
		line, err := conn.ReadLine()
		if err != nil {
			return nil, fmt.Errorf("reading feat choice: %w", err)
		}
		line = strings.TrimSpace(line)
		if strings.ToLower(line) == "cancel" {
			return nil, nil
		}
		choice := 0
		if _, err := fmt.Sscanf(line, "%d", &choice); err != nil || choice < 1 || choice > len(remaining) {
			_ = conn.WriteLine(telnet.Colorize(telnet.Red, "Invalid selection. Please enter a number from the list."))
			continue
		}
		picked := remaining[choice-1]
		chosen = append(chosen, picked)
		remaining = append(remaining[:choice-1], remaining[choice:]...)
		if h.featRegistry != nil {
			if f, ok := h.featRegistry.Feat(picked); ok {
				_ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "Selected: %s", f.Name))
			}
		}
	}
	return chosen, nil
}

// FeatPoolDeficit returns the number of feats still needed from pool.
// It counts how many feat IDs in pool are present in storedFeatIDs, then
// subtracts that from count. The result is clamped to zero from below.
// When pool is empty there are no feats available to satisfy count, so
// the deficit is always 0 (the loop cannot be filled).
//
// Precondition: count >= 0; pool may be nil or empty; storedFeatIDs may be nil
// (nil is treated identically to an empty map — no stored feats means full deficit).
// Postcondition: returns max(0, count - len(intersection(pool, storedFeatIDs)));
// returns 0 when pool is empty.
func FeatPoolDeficit(pool []string, storedFeatIDs map[string]bool, count int) int {
	if len(pool) == 0 {
		return 0
	}
	stored := 0
	for _, id := range pool {
		if storedFeatIDs[id] {
			stored++
		}
	}
	if stored >= count {
		return 0
	}
	return count - stored
}

// ensureFeats checks which feat pools still have a deficit for the character and,
// for each pool with a deficit, runs interactive selection for exactly the missing
// count. It is called before gameBridge for both new and existing characters so
// backfill always prompts for any un-filled pool.
//
// In headless mode (conn.Headless) this function is a no-op: automated clients
// cannot respond to interactive prompts.
//
// Precondition: char must have a valid ID, Class, and Skills populated.
// Postcondition: character_feats rows exist for char covering all granted pools;
// returns non-nil error only on fatal failure.
func (h *AuthHandler) ensureFeats(ctx context.Context, conn *telnet.Conn, char *character.Character) error {
	if conn.Headless {
		return nil
	}
	if h.characterFeats == nil || h.featRegistry == nil {
		return nil
	}

	job, ok := h.jobRegistry.Job(char.Class)
	if !ok {
		h.logger.Warn("unknown job for feat backfill", zap.String("class", char.Class))
		return nil
	}

	// Load already-stored feats to compute per-pool deficits.
	storedIDs, err := h.characterFeats.GetAll(ctx, char.ID)
	if err != nil {
		h.logger.Warn("loading stored feats for deficit check", zap.Int64("id", char.ID), zap.Error(err))
		return nil
	}
	storedSet := make(map[string]bool, len(storedIDs))
	for _, id := range storedIDs {
		storedSet[id] = true
	}

	// Determine whether any pool has a deficit.
	jobChoicesDeficit := 0
	if job.FeatGrants != nil && job.FeatGrants.Choices != nil && job.FeatGrants.Choices.Count > 0 {
		jobChoicesDeficit = FeatPoolDeficit(job.FeatGrants.Choices.Pool, storedSet, job.FeatGrants.Choices.Count)
	}

	generalPool := h.featRegistry.ByCategory("general")
	generalPoolIDs := make([]string, len(generalPool))
	for i, f := range generalPool {
		generalPoolIDs[i] = f.ID
	}
	generalDeficit := 0
	if job.FeatGrants != nil && job.FeatGrants.GeneralCount > 0 {
		generalDeficit = FeatPoolDeficit(generalPoolIDs, storedSet, job.FeatGrants.GeneralCount)
	}

	skillFeatPool := h.featRegistry.SkillFeatsForTrainedSkills(char.Skills)
	skillPoolIDs := make([]string, len(skillFeatPool))
	for i, f := range skillFeatPool {
		skillPoolIDs[i] = f.ID
	}
	skillDeficit := 0
	if len(skillPoolIDs) > 0 {
		skillDeficit = FeatPoolDeficit(skillPoolIDs, storedSet, 1)
	}

	// Determine whether any fixed feats are missing from the stored set.
	// Fixed feats are not part of any pool/choices deficit, so they must be
	// checked independently. A job whose FeatGrants contains only Fixed entries
	// (no Choices, no GeneralCount, no skill feats) would otherwise trigger the
	// early-exit below and never store its fixed feats.
	fixedMissing := false
	if job.FeatGrants != nil {
		for _, id := range job.FeatGrants.Fixed {
			if !storedSet[id] {
				fixedMissing = true
				break
			}
		}
	}

	// If all pools are fully satisfied AND all fixed feats are already stored,
	// nothing to do.
	if jobChoicesDeficit == 0 && generalDeficit == 0 && skillDeficit == 0 && !fixedMissing {
		return nil
	}

	_ = conn.WriteLine(telnet.Colorf(telnet.BrightYellow,
		"\r\n=== Step 2: Feats ==="))

	// Announce fixed job feats (informational only, already stored or will be added).
	var fixedNames []string
	if job.FeatGrants != nil {
		for _, id := range job.FeatGrants.Fixed {
			if f, ok := h.featRegistry.Feat(id); ok {
				fixedNames = append(fixedNames, f.Name)
			} else {
				fixedNames = append(fixedNames, id)
			}
		}
	}
	if len(fixedNames) > 0 {
		_ = conn.WriteLine(telnet.Colorf(telnet.Cyan, "Your job grants you the following feats:"))
		for _, n := range fixedNames {
			_ = conn.WriteLine(fmt.Sprintf("  %s- %s%s", telnet.BrightWhite, n, telnet.Reset))
		}
	}

	// Job feat choices — only prompt for the deficit count.
	var jobChosen []string
	if jobChoicesDeficit > 0 {
		jobChosen, err = h.featChoiceLoop(ctx, conn, "Choose a job feat", job.FeatGrants.Choices.Pool, jobChoicesDeficit)
		if err != nil {
			return fmt.Errorf("feat job choice: %w", err)
		}
	}

	// General feat choices — only prompt for the deficit count.
	var generalChosen []string
	if generalDeficit > 0 {
		generalChosen, err = h.featChoiceLoop(ctx, conn, "Choose a general feat", generalPoolIDs, generalDeficit)
		if err != nil {
			return fmt.Errorf("feat general choice: %w", err)
		}
	}

	// Skill feat choice — only prompt if deficit > 0.
	var skillChosen []string
	if skillDeficit > 0 {
		skillChosen, err = h.featChoiceLoop(ctx, conn, "Choose a skill feat", skillPoolIDs, skillDeficit)
		if err != nil {
			return fmt.Errorf("feat skill choice: %w", err)
		}
	}

	// Merge newly chosen feats with already-stored feats to produce the full set.
	newFeats := character.BuildFeatsFromJob(job, jobChosen, generalChosen, skillChosen)
	merged := make([]string, 0, len(storedIDs)+len(newFeats))
	merged = append(merged, storedIDs...)
	for _, id := range newFeats {
		if !storedSet[id] {
			merged = append(merged, id)
		}
	}

	if err := h.characterFeats.SetAll(ctx, char.ID, merged); err != nil {
		h.logger.Error("persisting backfill feats", zap.Int64("id", char.ID), zap.Error(err))
		_ = conn.WriteLine(telnet.Colorf(telnet.Yellow, "Warning: feats could not be saved."))
	}
	return nil
}

// ensureClassFeatures compares the character's stored class features against the full
// set granted by their job and adds any that are missing. This handles both first-time
// assignment (empty DB) and backfill when new features are added to a job after character
// creation. Existing features not present in the current job definition are preserved.
// Called before gameBridge for both new and existing characters.
// Safe to call in headless mode: no interactive prompts, purely automatic DB assignment.
//
// Precondition: char must have a valid ID and Class set.
// Postcondition: character_class_features rows include all job-granted features; returns
// non-nil error only on fatal failure.
func (h *AuthHandler) ensureClassFeatures(ctx context.Context, conn *telnet.Conn, char *character.Character) error {
	if h.characterClassFeatures == nil || h.classFeatureRegistry == nil {
		return nil
	}

	job, ok := h.jobRegistry.Job(char.Class)
	if !ok {
		h.logger.Warn("unknown job for class feature backfill", zap.String("class", char.Class))
		return nil
	}

	expected := character.BuildClassFeaturesFromJob(job)
	if len(expected) == 0 {
		return nil
	}

	current, err := h.characterClassFeatures.GetAll(ctx, char.ID)
	if err != nil {
		h.logger.Warn("fetching current class features for character", zap.Int64("id", char.ID), zap.Error(err))
		return nil
	}

	// Build set of already-stored feature IDs.
	stored := make(map[string]bool, len(current))
	for _, id := range current {
		stored[id] = true
	}

	// Find features granted by the job that are missing from the DB.
	var missing []string
	for _, id := range expected {
		if !stored[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	// Merge missing into current and persist.
	merged := append(current, missing...)
	if err := h.characterClassFeatures.SetAll(ctx, char.ID, merged); err != nil {
		h.logger.Warn("persisting class features", zap.Int64("id", char.ID), zap.Error(err))
		return nil
	}

	_ = conn.WriteLine(telnet.Colorf(telnet.BrightGreen,
		"%d class feature(s) added: %s.", len(missing), strings.Join(missing, ", ")))
	return nil
}

// ensureGender checks whether the character has a gender set and, if not,
// prompts the player to select one and persists the result.
// Called before gameBridge for both new and existing characters.
//
// In headless mode (conn.Headless) this function is a no-op: automated clients
// cannot respond to interactive prompts.
//
// Precondition: char must have a valid ID.
// Postcondition: char.Gender is non-empty on return; returns non-nil error only on fatal failure.
func (h *AuthHandler) ensureGender(ctx context.Context, conn *telnet.Conn, char *character.Character) error {
	if conn.Headless {
		return nil
	}
	if char.Gender != "" {
		return nil
	}
	_ = conn.WriteLine(telnet.Colorize(telnet.BrightYellow,
		"\r\nYour character needs a gender before entering the world."))
	gender, err := PromptGenderStep(conn)
	if err != nil {
		return fmt.Errorf("gender prompt: %w", err)
	}
	if gender == "" {
		// player cancelled — assign random so they can enter the game
		gender = character.RandomStandardGender()
	}
	if err := h.characters.SaveGender(ctx, char.ID, gender); err != nil {
		h.logger.Warn("saving gender", zap.Int64("id", char.ID), zap.Error(err))
	}
	char.Gender = gender
	return nil
}

// PromptGenderStep shows the gender selection menu and returns the chosen gender string.
// Returns ("", nil) if the player types "cancel".
// Options: 1=male, 2=female, 3=non-binary, 4=indeterminate, 5=custom, 0=random.
//
// Precondition: conn must be open.
// Postcondition: Returns a valid gender string or ("", nil) on cancel.
func PromptGenderStep(conn *telnet.Conn) (string, error) {
	_ = conn.WriteLine(telnet.Colorize(telnet.BrightYellow, "\r\nChoose your character's gender:"))
	_ = conn.WriteLine(fmt.Sprintf("  %s1%s. Male", telnet.Green, telnet.Reset))
	_ = conn.WriteLine(fmt.Sprintf("  %s2%s. Female", telnet.Green, telnet.Reset))
	_ = conn.WriteLine(fmt.Sprintf("  %s3%s. Non-binary", telnet.Green, telnet.Reset))
	_ = conn.WriteLine(fmt.Sprintf("  %s4%s. Indeterminate", telnet.Green, telnet.Reset))
	_ = conn.WriteLine(fmt.Sprintf("  %s5%s. Custom...", telnet.Green, telnet.Reset))
	_ = conn.WriteLine(fmt.Sprintf("  %s0%s. Random (default)", telnet.Green, telnet.Reset))
	_ = conn.WritePrompt(telnet.Colorize(telnet.BrightWhite, "Select [0-5, default=0]: "))
	line, err := conn.ReadLine()
	if err != nil {
		return "", fmt.Errorf("reading gender selection: %w", err)
	}
	line = strings.TrimSpace(line)
	if strings.ToLower(line) == "cancel" {
		return "", nil
	}
	switch line {
	case "1":
		return "male", nil
	case "2":
		return "female", nil
	case "3":
		return "non-binary", nil
	case "4":
		return "indeterminate", nil
	case "5":
		_ = conn.WritePrompt(telnet.Colorize(telnet.BrightWhite, "Enter custom gender (max 32 chars): "))
		custom, err := conn.ReadLine()
		if err != nil {
			return "", fmt.Errorf("reading custom gender: %w", err)
		}
		custom = strings.TrimSpace(custom)
		if strings.ToLower(custom) == "cancel" || custom == "" {
			return "", nil
		}
		if len(custom) > 32 {
			custom = custom[:32]
		}
		return "custom:" + custom, nil
	default:
		return character.RandomStandardGender(), nil
	}
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
	gender string,
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
	newChar.Gender = gender

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

	// Skill selection: only when skills and skill storage are configured.
	if h.characterSkills != nil && len(h.allSkills) > 0 {
		var chosenSkills []string
		if job.SkillGrants != nil && job.SkillGrants.Choices != nil && job.SkillGrants.Choices.Count > 0 {
			chosenSkills, err = h.skillChoiceLoop(ctx, conn, job.SkillGrants.Choices.Pool, job.SkillGrants.Choices.Count)
			if err != nil {
				h.logger.Error("skill choice loop", zap.String("name", created.Name), zap.Error(err))
				_ = conn.WriteLine(telnet.Colorf(telnet.Red, "Error during skill selection: %v", err))
				return created, nil // character was created; skills will be backfilled on login
			}
			if chosenSkills == nil {
				// player cancelled skill selection — return character; skills backfilled on login
				return created, nil
			}
		}
		allSkillIDs := make([]string, len(h.allSkills))
		for i, sk := range h.allSkills {
			allSkillIDs[i] = sk.ID
		}
		skillMap := character.BuildSkillsFromJob(job, allSkillIDs, chosenSkills)
		if err := h.characterSkills.SetAll(ctx, created.ID, skillMap); err != nil {
			h.logger.Error("persisting character skills", zap.String("name", created.Name), zap.Error(err))
			_ = conn.WriteLine(telnet.Colorf(telnet.Yellow,
				"Warning: skills could not be saved and will be assigned on login."))
		}
	}

	_ = conn.WriteLine(telnet.Colorf(telnet.BrightGreen,
		"Character %s created! [%s]", created.Name, elapsed))
	return created, nil
}

// RenderArchetypeMenu returns the formatted archetype selection menu string.
// Exported for testing.
//
// Precondition: archetypes must be non-nil (may be empty).
// Postcondition: Returns a formatted string listing all archetypes with R option.
func RenderArchetypeMenu(archetypes []*ruleset.Archetype) string {
	var sb strings.Builder
	for i, a := range archetypes {
		sb.WriteString(fmt.Sprintf("  %s%d%s. %s%s%s (HP/lvl: %d, Key: %s)\r\n     %s\r\n",
			telnet.Green, i+1, telnet.Reset,
			telnet.BrightWhite, a.Name, telnet.Reset,
			a.HitPointsPerLevel, a.KeyAbility,
			a.Description))
	}
	sb.WriteString(fmt.Sprintf("  %sR%s. Random (default)\r\n", telnet.Green, telnet.Reset))
	return sb.String()
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
	return fmt.Sprintf("%s%s%s — Lvl %d %s from %s [%s]",
		telnet.BrightWhite, c.Name, telnet.Reset,
		c.Level, c.Class, regionDisplay, c.Team)
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
	sb.WriteString(fmt.Sprintf("  Gender: %s\r\n", c.Gender))
	sb.WriteString(fmt.Sprintf("  HP:     %d/%d\r\n", c.CurrentHP, c.MaxHP))
	sb.WriteString(fmt.Sprintf("  BRT:%2d  QCK:%2d  GRT:%2d  RSN:%2d  SAV:%2d  FLR:%2d\r\n",
		c.Abilities.Brutality, c.Abilities.Quickness, c.Abilities.Grit,
		c.Abilities.Reasoning, c.Abilities.Savvy, c.Abilities.Flair))
	return sb.String()
}
