package unittest

import (
	"crypto/rand"
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
)

// TransactionForCluster generates a transaction that will be assigned to the
// target cluster ID.
func TransactionForCluster(clusters *flow.ClusterList, target flow.IdentityList) flow.TransactionBody {
	tx := TransactionBodyFixture()
	return AlterTransactionForCluster(tx, clusters, target, func(*flow.TransactionBody) {})
}

// AlterTransactionForCluster modifies a transaction nonce until it is assigned
// to the target cluster.
//
// The `after` function is run after each modification to allow for any content
// dependent changes to the transaction (eg. signing it).
func AlterTransactionForCluster(tx flow.TransactionBody, clusters *flow.ClusterList, target flow.IdentityList, after func(tx *flow.TransactionBody)) flow.TransactionBody {

	// Bound to avoid infinite loop in case the routing algorithm is broken
	tx.Nonce = 0
	for i := 0; i < 10000; i++ {
		_, err := rand.Read(tx.ReferenceBlockID[:])
		if err != nil {
			panic(err)
		}

		after(&tx)
		routed := clusters.ByTxID(tx.ID())
		if routed.Fingerprint() == target.Fingerprint() {
			return tx
		}
	}

	panic(fmt.Sprintf("unable to find transaction for target (%x) with %d clusters", target, clusters.Size()))
}
