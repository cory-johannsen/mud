package mentalstate

import "github.com/google/wire"

// Providers is the wire provider set for mental state dependencies.
var Providers = wire.NewSet(NewManager)
