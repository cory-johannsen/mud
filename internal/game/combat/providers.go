package combat

import "github.com/google/wire"

// Providers is the wire provider set for combat dependencies.
var Providers = wire.NewSet(NewEngine)
