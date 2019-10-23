// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/model/flow"
)

// Committee represents an interface to the node identity table.
type Committee interface {
	Me() flow.Node
	Get(nodeID string) (flow.Node, error)
	Select(filters ...NodeFilter) (flow.NodeList, error)
}

// NodeFilter is a function that returns true if we want to include a node.
type NodeFilter func(flow.Node) bool
