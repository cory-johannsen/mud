package telnet

import "github.com/google/wire"

// Providers is the wire provider set for the telnet acceptor.
var Providers = wire.NewSet(NewAcceptor)
