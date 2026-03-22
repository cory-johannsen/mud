package ruleset

import (
	"fmt"

	"github.com/google/wire"
	"go.uber.org/zap"
)

// JobsDir is the path to job YAML definitions.
type JobsDir string

// SkillsFile is the path to the skills YAML file.
type SkillsFile string

// FeatsFile is the path to the feats YAML file.
type FeatsFile string

// ClassFeaturesFile is the path to the class features YAML file.
type ClassFeaturesFile string

// ArchetypesDir is the path to archetype YAML definitions.
type ArchetypesDir string

// RegionsDir is the path to region YAML definitions.
type RegionsDir string

// TeamsDir is the path to team YAML definitions.
type TeamsDir string

// NewJobRegistryFromDir loads jobs and returns a JobRegistry.
func NewJobRegistryFromDir(dir JobsDir, logger *zap.Logger) (*JobRegistry, error) {
	jobs, err := LoadJobs(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading jobs: %w", err)
	}
	reg := NewJobRegistry()
	for _, j := range jobs {
		reg.Register(j)
	}
	logger.Info("loaded job definitions", zap.Int("count", len(jobs)))
	return reg, nil
}

// LoadAllSkills loads skills from file.
func LoadAllSkills(file SkillsFile, logger *zap.Logger) ([]*Skill, error) {
	skills, err := LoadSkills(string(file))
	if err != nil {
		return nil, fmt.Errorf("loading skills: %w", err)
	}
	logger.Info("loaded skill definitions", zap.Int("count", len(skills)))
	return skills, nil
}

// LoadAllFeats loads feats from file.
func LoadAllFeats(file FeatsFile, logger *zap.Logger) ([]*Feat, error) {
	feats, err := LoadFeats(string(file))
	if err != nil {
		return nil, fmt.Errorf("loading feats: %w", err)
	}
	logger.Info("loaded feat definitions", zap.Int("count", len(feats)))
	return feats, nil
}

// NewFeatRegistryFromFeats creates a FeatRegistry from loaded feats.
func NewFeatRegistryFromFeats(feats []*Feat) *FeatRegistry {
	return NewFeatRegistry(feats)
}

// LoadAllClassFeatures loads class features from file.
func LoadAllClassFeatures(file ClassFeaturesFile, logger *zap.Logger) ([]*ClassFeature, error) {
	features, err := LoadClassFeatures(string(file))
	if err != nil {
		return nil, fmt.Errorf("loading class features: %w", err)
	}
	logger.Info("loaded class features", zap.Int("count", len(features)))
	return features, nil
}

// NewClassFeatureRegistryFromFeatures creates a ClassFeatureRegistry.
func NewClassFeatureRegistryFromFeatures(features []*ClassFeature) *ClassFeatureRegistry {
	return NewClassFeatureRegistry(features)
}

// LoadArchetypeMap loads archetypes and returns a map keyed by ID.
func LoadArchetypeMap(dir ArchetypesDir, logger *zap.Logger) (map[string]*Archetype, error) {
	list, err := LoadArchetypes(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading archetypes: %w", err)
	}
	m := make(map[string]*Archetype, len(list))
	for _, a := range list {
		m[a.ID] = a
	}
	logger.Info("loaded archetype definitions", zap.Int("count", len(list)))
	return m, nil
}

// LoadRegionMap loads regions and returns a map keyed by ID.
func LoadRegionMap(dir RegionsDir, logger *zap.Logger) (map[string]*Region, error) {
	list, err := LoadRegions(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading regions: %w", err)
	}
	m := make(map[string]*Region, len(list))
	for _, r := range list {
		m[r.ID] = r
	}
	logger.Info("loaded region definitions", zap.Int("count", len(list)))
	return m, nil
}

// LoadAllTeams loads teams from dir (used by devserver/frontend only).
func LoadAllTeams(dir TeamsDir, logger *zap.Logger) ([]*Team, error) {
	teams, err := LoadTeams(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading teams: %w", err)
	}
	logger.Info("loaded team definitions", zap.Int("count", len(teams)))
	return teams, nil
}

// LoadAllJobsSlice loads jobs as a slice (used by devserver/frontend).
func LoadAllJobsSlice(dir JobsDir, logger *zap.Logger) ([]*Job, error) {
	jobs, err := LoadJobs(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading jobs: %w", err)
	}
	logger.Info("loaded job definitions", zap.Int("count", len(jobs)))
	return jobs, nil
}

// LoadAllRegionsSlice loads regions as a slice (used by devserver/frontend).
func LoadAllRegionsSlice(dir RegionsDir, logger *zap.Logger) ([]*Region, error) {
	regions, err := LoadRegions(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading regions: %w", err)
	}
	logger.Info("loaded region definitions", zap.Int("count", len(regions)))
	return regions, nil
}

// LoadAllArchetypesSlice loads archetypes as a slice (used by devserver/frontend).
func LoadAllArchetypesSlice(dir ArchetypesDir, logger *zap.Logger) ([]*Archetype, error) {
	list, err := LoadArchetypes(string(dir))
	if err != nil {
		return nil, fmt.Errorf("loading archetypes: %w", err)
	}
	logger.Info("loaded archetype definitions", zap.Int("count", len(list)))
	return list, nil
}

// GameProviders is the full ruleset provider set for the gameserver.
var GameProviders = wire.NewSet(
	NewJobRegistryFromDir,
	LoadAllSkills,
	LoadAllFeats,
	NewFeatRegistryFromFeats,
	LoadAllClassFeatures,
	NewClassFeatureRegistryFromFeatures,
	LoadArchetypeMap,
	LoadRegionMap,
)

// RulesetContentProviders is the minimal ruleset provider set for devserver/frontend.
var RulesetContentProviders = wire.NewSet(
	LoadAllJobsSlice,
	LoadAllRegionsSlice,
	LoadAllArchetypesSlice,
	LoadAllSkills,
	LoadAllFeats,
	LoadAllClassFeatures,
	LoadAllTeams,
)
