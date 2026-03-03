package ruleset

import "sort"

// JobRegistry provides fast lookup of team affiliation by job/class ID.
type JobRegistry struct {
	jobs map[string]*Job
}

// NewJobRegistry returns an empty JobRegistry.
//
// Postcondition: Returns a non-nil *JobRegistry ready to accept registrations.
func NewJobRegistry() *JobRegistry {
	return &JobRegistry{jobs: make(map[string]*Job)}
}

// Register adds a Job to the registry.
//
// Precondition: job must be non-nil with a non-empty ID.
// Postcondition: job is retrievable via TeamFor using job.ID;
// if called multiple times with the same ID, the last call wins.
func (r *JobRegistry) Register(job *Job) {
	if job == nil {
		panic("JobRegistry.Register: precondition violated: job must be non-nil")
	}
	if job.ID == "" {
		panic("JobRegistry.Register: precondition violated: job ID must be non-empty")
	}
	r.jobs[job.ID] = job
}

// TeamFor returns the team string for the given class ID.
//
// Precondition: classID may be any string.
// Postcondition: Returns "" if classID is unknown or if the job has no team affiliation.
func (r *JobRegistry) TeamFor(classID string) string {
	if j, ok := r.jobs[classID]; ok {
		return j.Team
	}
	return ""
}

// Job returns the Job for the given class ID, if registered.
//
// Precondition: classID may be any string.
// Postcondition: Returns the registered Job and true, or nil and false if not found.
func (r *JobRegistry) Job(classID string) (*Job, bool) {
	j, ok := r.jobs[classID]
	return j, ok
}

// ArchetypesForTeam returns the distinct archetype IDs that have at least one job
// available for the given team. Jobs with an empty Team field are available to all teams.
//
// Precondition: team may be any string.
// Postcondition: Returns a deduplicated, sorted slice; empty slice (not nil) if none match.
func (r *JobRegistry) ArchetypesForTeam(team string) []string {
	seen := make(map[string]struct{})
	for _, j := range r.jobs {
		if j.Team == team || j.Team == "" {
			seen[j.Archetype] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for a := range seen {
		result = append(result, a)
	}
	sort.Strings(result)
	return result
}

// JobsForTeamAndArchetype returns all jobs that match the given team and archetype.
// A job with an empty Team field is available to any team.
//
// Precondition: team and archetype may be any string.
// Postcondition: Returns a non-nil slice (may be empty).
func (r *JobRegistry) JobsForTeamAndArchetype(team, archetype string) []*Job {
	result := make([]*Job, 0)
	for _, j := range r.jobs {
		if j.Archetype == archetype && (j.Team == team || j.Team == "") {
			result = append(result, j)
		}
	}
	return result
}
