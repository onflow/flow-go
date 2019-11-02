// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Committee represents an interface to the node identity table.
type Committee interface {
	Me() flow.Identity
	Get(nodeID string) (flow.Identity, error)
	Select() flow.IdentityList
	Leader(height uint64) flow.Identity
	Quorum() uint
}
