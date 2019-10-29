// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Committee represents an interface to the node identity table.
type Committee interface {
	Me() flow.Identity
	Get(nodeID string) (flow.Identity, error)
	Select(filters ...IdentityFilter) (flow.IdentityList, error)
}

// IdentityFilter is a function that returns true if we want to include a node.
type IdentityFilter func(flow.Identity) bool
