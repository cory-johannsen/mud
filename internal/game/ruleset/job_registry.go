package ruleset

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
