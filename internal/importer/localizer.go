package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/cory-johannsen/mud/internal/game/technology"
	"gopkg.in/yaml.v3"
)

// Localizer rewrites the Name and Description of a TechnologyDef in Gunchete lore style.
//
// Precondition: ctx must be non-nil; def must be non-nil.
// Postcondition: on success, def.Name and def.Description may be modified;
// all other fields are unchanged. Returns non-nil error only for API-level failures.
type Localizer interface {
	Localize(ctx context.Context, def *technology.TechnologyDef) error
}

// NoopLocalizer satisfies the Localizer interface without modifying def.
type NoopLocalizer struct{}

func (NoopLocalizer) Localize(_ context.Context, _ *technology.TechnologyDef) error { return nil }

// ClaudeLocalizer calls the Claude API (claude-sonnet-4-6) to rewrite Name and
// Description in Gunchete cyberpunk lore style.
type ClaudeLocalizer struct {
	client      *anthropic.Client
	refDoc      string
	sampleTechs string
}

// NewClaudeLocalizer constructs a ClaudeLocalizer.
// apiKey is the Anthropic API key.
// repoRoot is the repository root directory (used to load the reference doc and sample techs).
//
// Precondition: apiKey must be non-empty; repoRoot must point to the repo root.
// Postcondition: returns a non-nil *ClaudeLocalizer or a non-nil error.
func NewClaudeLocalizer(apiKey, repoRoot string) (*ClaudeLocalizer, error) {
	refPath := filepath.Join(repoRoot, "docs", "requirements", "pf2e-import-reference.md")
	refBytes, err := os.ReadFile(refPath)
	if err != nil {
		return nil, fmt.Errorf("reading pf2e-import-reference.md: %w", err)
	}

	samples, err := loadTechSamples(filepath.Join(repoRoot, "content", "technologies"))
	if err != nil {
		return nil, fmt.Errorf("loading tech samples: %w", err)
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &ClaudeLocalizer{
		client:      &client,
		refDoc:      string(refBytes),
		sampleTechs: samples,
	}, nil
}

// Localize calls Claude to rewrite def.Name and def.Description in Gunchete lore style.
// On JSON parse failure of the response, keeps the original values and prints a warning.
// On API error, returns the error.
func (c *ClaudeLocalizer) Localize(ctx context.Context, def *technology.TechnologyDef) error {
	systemPrompt := fmt.Sprintf(`You are a lore writer for Gunchete, a cyberpunk tabletop RPG.
Your job is to translate fantasy spell names and descriptions into Gunchete's cyberpunk aesthetic.
In Gunchete there is no magic — only technology, chemistry, drugs, and cybernetics.

Reference document:
%s

Sample existing technologies (name: description):
%s

Rules:
- Preserve all mechanical text exactly: dice expressions (e.g. "1d6"), ranges, durations, save types.
- Only rewrite flavor and lore text.
- Keep descriptions concise (1-3 sentences).
- Never mention magic, spells, mana, or fantasy tropes.
- Respond with ONLY valid JSON in this exact format: {"name": "...", "description": "..."}`,
		c.refDoc, c.sampleTechs)

	userPrompt := fmt.Sprintf(`Translate the following technology into Gunchete lore style.

Current name: %s
Current description: %s

Respond with ONLY valid JSON: {"name": "...", "description": "..."}`,
		def.Name, def.Description)

	msg, err := c.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.ModelClaudeSonnet4_6,
		MaxTokens: 512,
		System: []anthropic.TextBlockParam{
			{Text: systemPrompt},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(userPrompt)),
		},
	})
	if err != nil {
		return fmt.Errorf("Claude API error: %w", err)
	}

	if len(msg.Content) == 0 {
		fmt.Fprintf(os.Stderr, "WARNING: Claude returned empty response for %q; keeping original\n", def.Name)
		return nil
	}

	raw := msg.Content[0].Text
	var result struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to parse Claude response for %q: %v; keeping original\n", def.Name, err)
		return nil
	}

	if result.Name != "" {
		def.Name = result.Name
	}
	if result.Description != "" {
		def.Description = result.Description
	}
	return nil
}

// loadTechSamples reads up to 10 technology YAML files from techDir and returns
// a formatted string of "name: description" pairs.
func loadTechSamples(techDir string) (string, error) {
	var samples []string
	err := filepath.WalkDir(techDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return err
		}
		if len(samples) >= 10 {
			return filepath.SkipAll
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}
		var def technology.TechnologyDef
		if err := yaml.Unmarshal(data, &def); err != nil {
			return nil
		}
		samples = append(samples, fmt.Sprintf("%s: %s", def.Name, def.Description))
		return nil
	})
	if err != nil {
		return "", err
	}
	return strings.Join(samples, "\n"), nil
}
