package collection

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	client "github.com/onflow/flow-go-sdk/access/grpc"

	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/model/flow"
)

func TestMultiCluster(t *testing.T) {
	suite.Run(t, new(MultiClusterSuite))
}

type MultiClusterSuite struct {
	CollectorSuite
}

// Run consensus on a multi-cluster setup. Ensure that transactions
// are always included in the appropriate cluster.
func (suite *MultiClusterSuite) TestProposal_MultiCluster() {
	t := suite.T()
	t.Logf("%v ================> START TESTING %v", time.Now().UTC(), t.Name())
	var (
		nNodes        = 9
		nClusters     = 3
		clusterSize   = nNodes / nClusters
		nTransactions = 10
	)

	suite.SetupTest("col_proposal_multicluster", uint(nNodes), uint(nClusters))

	clusters := suite.Clusters()

	// create a client for each node, organized by cluster
	clientsByCluster := [3][]*client.Client{{}, {}, {}}
	for i := 0; i < nClusters; i++ {
		forCluster := make([]*client.Client, 0, clusterSize)

		for j := 0; j < clusterSize; j++ {
			node := suite.Collector(uint(i), uint(j))
			client, err := node.SDKClient()
			suite.Require().NoError(err)
			forCluster = append(forCluster, client)
		}

		clientsByCluster[i] = append(clientsByCluster[i], forCluster...)
	}

	suite.Run("correct cluster - no dupes", func() {
		t := suite.T()
		t.Logf("%v ================> START RUN TESTING %v", time.Now().UTC(), t.Name())
		suite.T().Log("ROUND 1")

		// keep track of which cluster is responsible for which transaction ID
		txIDsByCluster := [3][]flow.Identifier{{}, {}, {}}

		for clusterIdx := 0; clusterIdx < nClusters; clusterIdx++ {
			forCluster := make([]flow.Identifier, 0, nTransactions)

			for txIdx := 0; txIdx < nTransactions; txIdx++ {
				cluster, ok := clusters.ByIndex(uint(clusterIdx))
				require.True(suite.T(), ok)
				tx := suite.TxForCluster(cluster)
				forCluster = append(forCluster, convert.IDFromSDK(tx.ID()))

				// pick a client from this cluster
				target := clientsByCluster[clusterIdx][txIdx%clusterSize]
				// queue up a function to send the transaction
				go func() {
					ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
					err := target.SendTransaction(ctx, *tx)
					cancel()
					suite.Require().NoError(err)
				}()
			}

			txIDsByCluster[clusterIdx] = append(txIDsByCluster[clusterIdx], forCluster...)
		}

		// wait for all the transactions to be included
		suite.AwaitTransactionsIncluded(
			append(append(txIDsByCluster[0], txIDsByCluster[1]...), txIDsByCluster[2]...)...,
		)
		t.Logf("%v ================> FINISH RUN TESTING %v", time.Now().UTC(), t.Name())
	})

	suite.Run("correct cluster - with dupes", func() {
		t := suite.T()
		t.Logf("%v ================> START RUN TESTING %v", time.Now().UTC(), t.Name())
		suite.T().Log("ROUND 2")

		// keep track of which cluster is responsible for which transaction ID
		txIDsByCluster := [3][]flow.Identifier{{}, {}, {}}

		for clusterIdx := 0; clusterIdx < nClusters; clusterIdx++ {
			forCluster := make([]flow.Identifier, 0, nTransactions)

			for txIdx := 0; txIdx < nTransactions; txIdx++ {
				cluster, ok := clusters.ByIndex(uint(clusterIdx))
				require.True(suite.T(), ok)
				tx := suite.TxForCluster(cluster)
				forCluster = append(forCluster, convert.IDFromSDK(tx.ID()))

				// pick two clients from this cluster (to introduce dupes)
				target1 := clientsByCluster[clusterIdx][rand.Intn(clusterSize)]
				target2 := clientsByCluster[clusterIdx][rand.Intn(clusterSize)]

				// queue up a function to send the transaction (twice)
				go func() {
					ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
					err := target1.SendTransaction(ctx, *tx)
					suite.Require().NoError(err)
					err = target2.SendTransaction(ctx, *tx)
					suite.Require().NoError(err)
					cancel()
				}()
			}

			txIDsByCluster[clusterIdx] = append(txIDsByCluster[clusterIdx], forCluster...)
		}

		// wait for all the transactions to be included
		suite.AwaitTransactionsIncluded(
			append(append(txIDsByCluster[0], txIDsByCluster[1]...), txIDsByCluster[2]...)...,
		)
		t.Logf("%v ================> FINISH RUN TESTING %v", time.Now().UTC(), t.Name())
	})

	suite.Run("wrong cluster - no dupes", func() {
		t := suite.T()
		t.Logf("%v ================> START RUN TESTING %v", time.Now().UTC(), t.Name())
		suite.T().Log("ROUND 3")

		// keep track of which cluster is responsible for which transaction ID
		txIDsByCluster := [3][]flow.Identifier{{}, {}, {}}

		for clusterIdx := 0; clusterIdx < nClusters; clusterIdx++ {
			forCluster := make([]flow.Identifier, 0, nTransactions)

			for txIdx := 0; txIdx < nTransactions; txIdx++ {
				cluster, ok := clusters.ByIndex(uint(clusterIdx))
				require.True(suite.T(), ok)
				tx := suite.TxForCluster(cluster)
				forCluster = append(forCluster, convert.IDFromSDK(tx.ID()))

				// pick a client from a different cluster
				clusterIndex := rand.Intn(nClusters)
				if clusterIndex == clusterIdx {
					clusterIndex = (clusterIndex + 1) % nClusters
				}
				target := clientsByCluster[clusterIndex][txIdx%clusterSize]

				// queue up a function to send the transaction
				go func() {
					ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
					err := target.SendTransaction(ctx, *tx)
					cancel()
					suite.Require().NoError(err)
				}()
			}

			txIDsByCluster[clusterIdx] = append(txIDsByCluster[clusterIdx], forCluster...)
		}

		// wait for all the transactions to be included
		suite.AwaitTransactionsIncluded(
			append(append(txIDsByCluster[0], txIDsByCluster[1]...), txIDsByCluster[2]...)...,
		)
		t.Logf("%v ================> FINISH RUN TESTING %v", time.Now().UTC(), t.Name())
	})

	suite.Run("wrong cluster - with dupes", func() {
		t := suite.T()
		t.Logf("%v ================> START RUN TESTING %v", time.Now().UTC(), t.Name())
		suite.T().Log("ROUND 4")

		// keep track of which cluster is responsible for which transaction ID
		txIDsByCluster := [3][]flow.Identifier{{}, {}, {}}

		for clusterIdx := 0; clusterIdx < nClusters; clusterIdx++ {
			forCluster := make([]flow.Identifier, 0, nTransactions)

			for txIdx := 0; txIdx < nTransactions; txIdx++ {
				cluster, ok := clusters.ByIndex(uint(clusterIdx))
				require.True(suite.T(), ok)
				tx := suite.TxForCluster(cluster)
				forCluster = append(forCluster, convert.IDFromSDK(tx.ID()))

				for senderIdx := 0; senderIdx < nClusters; senderIdx++ {
					// don't send the responsible cluster
					if senderIdx == clusterIdx {
						continue
					}

					target := clientsByCluster[senderIdx][rand.Intn(clusterSize)]
					// queue up a function to send the transaction
					go func() {
						ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
						err := target.SendTransaction(ctx, *tx)
						cancel()
						suite.Require().NoError(err)
					}()
				}
			}
			txIDsByCluster[clusterIdx] = append(txIDsByCluster[clusterIdx], forCluster...)
		}

		// wait for all the new transactions to be included
		suite.AwaitTransactionsIncluded(
			append(append(txIDsByCluster[0], txIDsByCluster[1]...), txIDsByCluster[2]...)...,
		)
		t.Logf("%v ================> FINISH RUN TESTING %v", time.Now().UTC(), t.Name())
	})
}
