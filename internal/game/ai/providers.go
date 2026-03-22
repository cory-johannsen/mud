package ai

import "github.com/google/wire"

// NewEmptyRegistry creates an empty AI registry.
func NewEmptyRegistry() *Registry {
	return NewRegistry()
}

// Providers is the wire provider set for AI dependencies.
var Providers = wire.NewSet(NewEmptyRegistry)
