package module

import (
	"github.com/onflow/flow-go/network"
)

// Engine is the interface all engines should implement in order to have a
// manageable lifecycle and recieve messages from the networking layer.
type Engine interface {
	ReadyDoneAware
	network.Engine
}
