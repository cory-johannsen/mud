// Package version exposes the application version injected at build time.
package version

// Version is set via -ldflags "-X github.com/cory-johannsen/mud/internal/version.Version=vX.Y.Z"
// at build time. Falls back to "dev" when built without ldflags (local development).
var Version = "dev"
