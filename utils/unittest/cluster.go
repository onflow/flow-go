package unittest

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protocol"
)

// TransactionForCluster generates a transaction that will be assigned to the
// target cluster ID.
func TransactionForCluster(nClusters int, target flow.ClusterID) *flow.Transaction {
	tx := TransactionFixture()

	// Bound to avoid infinite loop in case the routing algorithm is broken
	for i := 0; i < 10000; i++ {
		tx.Nonce++
		id := protocol.Route(nClusters, tx.Fingerprint())
		if id == target {
			return &tx
		}
	}

	panic(fmt.Sprintf("unable to find transaction for target (%d) with %d clusters", target, nClusters))
}
