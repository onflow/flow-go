// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order

import (
	"strings"

	"github.com/dapperlabs/flow-go/model/flow"
)

// ByID orders the identities by ID.
func ByID(id1 flow.Identity, id2 flow.Identity) bool {
	return strings.Compare(id1.NodeID, id2.NodeID) < 0
}
