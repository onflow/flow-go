// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Synchronization is responsible for synchronizing our node if we are too far
// behind the network; this can be done both proactively (internally) and
// reactively (by requesting certain blocks).
type Synchronization interface {
	ReadyDoneAware
	RequestBlock(blockID flow.Identifier)
}
