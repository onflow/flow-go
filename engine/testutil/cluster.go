package testutil

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// TransactionForCluster generates a transaction that will be assigned to the
// target cluster ID.
func TransactionForCluster(clusters *flow.ClusterList, target flow.IdentityList) *flow.TransactionBody {
	tx := unittest.TransactionBodyFixture()

	// Bound to avoid infinite loop in case the routing algorithm is broken
	tx.Nonce = 0
	for i := 0; i < 10000; i++ {
		tx.Nonce++
		routed := clusters.ByTxID(tx.ID())
		if routed.Fingerprint() == target.Fingerprint() {
			return &tx
		}
	}

	panic(fmt.Sprintf("unable to find transaction for target (%x) with %d clusters", target, clusters.Size()))
}
