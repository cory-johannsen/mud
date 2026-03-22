package handlers

import "github.com/google/wire"

// Providers is the wire provider set for frontend handlers.
var Providers = wire.NewSet(NewAuthHandler)
