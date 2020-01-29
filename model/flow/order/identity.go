// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order

import (
	"bytes"

	"github.com/dapperlabs/flow-go/model/flow"
)

func ByNodeIDAsc(id1 *flow.Identity, id2 *flow.Identity) bool {
	return bytes.Compare(id1.NodeID[:], id2.NodeID[:]) < 0
}
