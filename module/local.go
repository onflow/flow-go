// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package module

import (
	"github.com/dapperlabs/flow-go/model"
)

// Local encapsulates the stable local node information.
type Local interface {

	// NodeID returns the node ID of the local node.
	NodeID() model.Identifier

	// Address returns the (listen) address of the local node.
	Address() string
}
