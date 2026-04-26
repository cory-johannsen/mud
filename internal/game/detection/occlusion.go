package detection

// OcclusionProvider is the seam reserved for Line-of-Sight / occlusion checks
// (#267). v1 of the detection ladder ships a no-op implementation that always
// returns "no occlusion"; #267 fills in real geometry.
//
// IsOccluded returns true if the line from observer→target is blocked by an
// opaque obstruction. Callers should treat occlusion as forcing the pair-
// state to at least Hidden when true.
type OcclusionProvider interface {
	IsOccluded(observerUID, targetUID string) bool
}

// NoOpOcclusion is the v1 default OcclusionProvider: every line is clear.
type NoOpOcclusion struct{}

// IsOccluded always returns false in the no-op provider.
func (NoOpOcclusion) IsOccluded(string, string) bool { return false }
