// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package order

import (
	"bytes"

	"github.com/onflow/flow-go/model/flow"
)

func ByNodeIDAsc(identity1 *flow.Identity, identity2 *flow.Identity) bool {
	return bytes.Compare(identity1.NodeID[:], identity2.NodeID[:]) < 0
}
