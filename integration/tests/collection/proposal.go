package collection

import (
	"context"
	"math/rand"
	"time"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
)

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
			client, err := client.New(node.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure()) //nolint:staticcheck
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

type RecoverySuite struct {
	CollectorSuite
}

// Start consensus, pause a node, allow consensus to continue, then restart
// the node. It should be able to catch up.
func (suite *RecoverySuite) TestProposal_Recovery() {
	t := suite.T()
	t.Logf("%v ================> START TESTING %v", time.Now().UTC(), t.Name())

	// TODO this doesn't quite work with network disconnect/connect for some
	// reason, skipping for now
	suite.T().SkipNow()

	var (
		nNodes        = 5
		nTransactions = 30
		err           error
	)

	suite.SetupTest("col_proposal_recovery", uint(nNodes), 1)

	// create a client for each of the collectors
	clients := make([]*client.Client, nNodes)
	for i := 0; i < nNodes; i++ {
		clients[i], err = client.New(
			suite.Collector(0, uint(i)).Addr(testnet.ColNodeAPIPort),
			grpc.WithInsecure(), //nolint:staticcheck
		)
		suite.Require().NoError(err)
	}

	// send a bunch of transactions
	txIDs := make([]flow.Identifier, 0, nTransactions)
	for i := 0; i < nTransactions; i++ {

		tx := suite.NextTransaction()
		// round-robin transactions between nodes
		target := clients[i%len(clients)]

		go func() {
			ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
			err = target.SendTransaction(ctx, *tx)
			suite.Require().NoError(err)
			cancel()
		}()

		txIDs = append(txIDs, convert.IDFromSDK(tx.ID()))
	}

	// wait for the transactions to be incorporated
	suite.AwaitTransactionsIncluded(txIDs...)

	// stop one of the nodes
	suite.T().Log("stopping COL1")
	col1 := suite.Collector(0, 0)
	err = col1.Disconnect()
	suite.Require().NoError(err)

	// send some more transactions
	txIDs = make([]flow.Identifier, 0, nTransactions)
	for i := 0; i < nTransactions; i++ {
		tx := suite.NextTransaction()

		// round-robin transactions between collectors (except the paused one)
		target := clients[1:][i%(len(clients)-1)]

		ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
		err = target.SendTransaction(ctx, *tx)
		suite.Require().NoError(err)
		cancel()

		txIDs = append(txIDs, convert.IDFromSDK(tx.ID()))
	}

	// wait for the transactions to be included (4/5 nodes can make progress)
	suite.AwaitTransactionsIncluded(txIDs...)

	// stop another node
	suite.T().Log("stopping COL2")
	col2 := suite.Collector(0, 1)
	err = col2.Disconnect()
	suite.Require().NoError(err)

	// send some more transactions
	txIDs = make([]flow.Identifier, 0, nTransactions)
	for i := 0; i < nTransactions; i++ {
		tx := suite.NextTransaction()

		// round-robin transactions between collectors (except the paused ones)
		target := clients[2:][i%(len(clients)-2)]

		ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
		err = target.SendTransaction(ctx, *tx)
		suite.Require().NoError(err)
		cancel()

		txIDs = append(txIDs, convert.IDFromSDK(tx.ID()))
	}

	// ensure no progress was made (3/5 nodes cannot make progress)
	proposals := suite.AwaitProposals(10)
	height := proposals[0].Header.Height
	for _, prop := range proposals {
		suite.Assert().LessOrEqual(prop.Header.Height, height+2)
	}

	// restart the paused collectors
	suite.T().Log("restarting COL1")
	err = col1.Connect()
	suite.Require().NoError(err)

	suite.T().Log("restarting COL2")
	err = col2.Connect()
	suite.Require().NoError(err)

	// now we can make progress again, but the paused collectors need to catch
	// up to the current chain state first
	suite.AwaitTransactionsIncluded(txIDs...)
	t.Logf("%v ================> FINISH TESTING %v", time.Now().UTC(), t.Name())
}
