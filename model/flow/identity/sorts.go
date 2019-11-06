// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package identity

import (
	"strings"

	"github.com/dapperlabs/flow-go/model/flow"
)

// Reverse will reverse any sort.
func Reverse(less flow.IdentitySort) flow.IdentitySort {
	return func(id1 flow.Identity, id2 flow.Identity) bool {
		return less(id2, id1)
	}
}

// NodeIDAsc orders the identities by ID.
func NodeIDAsc(id1 flow.Identity, id2 flow.Identity) bool {
	return strings.Compare(id1.NodeID, id2.NodeID) < 0
}
