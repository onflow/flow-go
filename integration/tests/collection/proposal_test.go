package collection

import (
	"context"
	"math/rand"

	"github.com/onflow/flow-go-sdk/client"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/integration/convert"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Run consensus on a multi-cluster setup. Ensure that transactions
// are always included in the appropriate cluster.
func (suite *CollectorSuite) TestProposal_MultiCluster() {

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
			client, err := client.New(node.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure())
			suite.Require().Nil(err)
			forCluster = append(forCluster, client)
		}

		clientsByCluster[i] = append(clientsByCluster[i], forCluster...)
	}

	suite.Run("correct cluster - no dupes", func() {
		suite.T().Log("ROUND 1")

		// keep track of which cluster is responsible for which transaction ID
		txIDsByCluster := [3][]flow.Identifier{{}, {}, {}}

		for clusterIdx := 0; clusterIdx < nClusters; clusterIdx++ {
			forCluster := make([]flow.Identifier, 0, nTransactions)

			for txIdx := 0; txIdx < nTransactions; txIdx++ {
				tx := suite.TxForCluster(clusters.ByIndex(uint(clusterIdx)))
				forCluster = append(forCluster, convert.IDFromSDK(tx.ID()))

				// pick a client from this cluster
				target := clientsByCluster[clusterIdx][txIdx%clusterSize]
				// queue up a function to send the transaction
				go func() {
					ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
					err := target.SendTransaction(ctx, *tx)
					cancel()
					suite.Require().Nil(err)
				}()
			}

			txIDsByCluster[clusterIdx] = append(txIDsByCluster[clusterIdx], forCluster...)
		}

		// wait for all the transactions to be included
		suite.AwaitTransactionsIncluded(
			append(append(txIDsByCluster[0], txIDsByCluster[1]...), txIDsByCluster[2]...)...,
		)
	})

	suite.Run("correct cluster - with dupes", func() {
		suite.T().Log("ROUND 2")

		// keep track of which cluster is responsible for which transaction ID
		txIDsByCluster := [3][]flow.Identifier{{}, {}, {}}

		for clusterIdx := 0; clusterIdx < nClusters; clusterIdx++ {
			forCluster := make([]flow.Identifier, 0, nTransactions)

			for txIdx := 0; txIdx < nTransactions; txIdx++ {
				tx := suite.TxForCluster(clusters.ByIndex(uint(clusterIdx)))
				forCluster = append(forCluster, convert.IDFromSDK(tx.ID()))

				// pick two clients from this cluster (to introduce dupes)
				target1 := clientsByCluster[clusterIdx][rand.Intn(clusterSize)]
				target2 := clientsByCluster[clusterIdx][rand.Intn(clusterSize)]

				// queue up a function to send the transaction (twice)
				go func() {
					ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
					err := target1.SendTransaction(ctx, *tx)
					suite.Require().Nil(err)
					err = target2.SendTransaction(ctx, *tx)
					suite.Require().Nil(err)
					cancel()
				}()
			}

			txIDsByCluster[clusterIdx] = append(txIDsByCluster[clusterIdx], forCluster...)
		}

		// wait for all the transactions to be included
		suite.AwaitTransactionsIncluded(
			append(append(txIDsByCluster[0], txIDsByCluster[1]...), txIDsByCluster[2]...)...,
		)
	})

	suite.Run("wrong cluster - no dupes", func() {
		suite.T().Log("ROUND 3")

		// keep track of which cluster is responsible for which transaction ID
		txIDsByCluster := [3][]flow.Identifier{{}, {}, {}}

		for clusterIdx := 0; clusterIdx < nClusters; clusterIdx++ {
			forCluster := make([]flow.Identifier, 0, nTransactions)

			for txIdx := 0; txIdx < nTransactions; txIdx++ {
				tx := suite.TxForCluster(clusters.ByIndex(uint(clusterIdx)))
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
					suite.Require().Nil(err)
				}()
			}

			txIDsByCluster[clusterIdx] = append(txIDsByCluster[clusterIdx], forCluster...)
		}

		// wait for all the transactions to be included
		suite.AwaitTransactionsIncluded(
			append(append(txIDsByCluster[0], txIDsByCluster[1]...), txIDsByCluster[2]...)...,
		)
	})

	suite.Run("wrong cluster - with dupes", func() {
		suite.T().Log("ROUND 4")

		// keep track of which cluster is responsible for which transaction ID
		txIDsByCluster := [3][]flow.Identifier{{}, {}, {}}

		for clusterIdx := 0; clusterIdx < nClusters; clusterIdx++ {
			forCluster := make([]flow.Identifier, 0, nTransactions)

			for txIdx := 0; txIdx < nTransactions; txIdx++ {
				tx := suite.TxForCluster(clusters.ByIndex(uint(clusterIdx)))
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
						suite.Require().Nil(err)
						suite.T().Log("sent tx: ", convert.IDFromSDK(tx.ID()))
					}()
				}
			}
			txIDsByCluster[clusterIdx] = append(txIDsByCluster[clusterIdx], forCluster...)
		}

		// wait for all the new transactions to be included
		suite.AwaitTransactionsIncluded(
			append(append(txIDsByCluster[0], txIDsByCluster[1]...), txIDsByCluster[2]...)...,
		)
	})
}

// Start consensus, pause a node, allow consensus to continue, then restart
// the node. It should be able to catch up.
func (suite *CollectorSuite) TestProposal_Recovery() {

	// skipping - this will fail until HotStuff is integrated
	suite.T().SkipNow()

	var (
		nNodes        = 6
		nTransactions = 30
		err           error
	)

	suite.SetupTest("col_proposal_recovery", uint(nNodes), 1)

	// create a client for each of the collectors
	clients := make([]*client.Client, nNodes)
	for i := 0; i < nNodes; i++ {
		clients[i], err = client.New(
			suite.Collector(0, uint(i)).Addr(testnet.ColNodeAPIPort),
			grpc.WithInsecure(),
		)
		suite.Require().Nil(err)
	}

	// send a bunch of transactions
	txIDs := make([]flow.Identifier, 0, nTransactions)
	for i := 0; i < nTransactions; i++ {
		tx := suite.NextTransaction()

		// round-robin transactions between nodes
		target := clients[i%len(clients)]

		ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
		err = target.SendTransaction(ctx, *tx)
		suite.Require().Nil(err)
		cancel()

		txIDs = append(txIDs, convert.IDFromSDK(tx.ID()))
	}

	// wait for the transactions to be incorporated
	suite.AwaitTransactionsIncluded(txIDs...)

	// stop one of the nodes
	col1 := suite.Collector(0, 0)
	err = col1.Pause()
	suite.Require().Nil(err)

	// send some more transactions
	txIDs = make([]flow.Identifier, 0, nTransactions)
	for i := 0; i < nTransactions; i++ {
		tx := suite.NextTransaction()

		// round-robin transactions between collectors (except the paused one)
		target := clients[1:][i%(len(clients)-1)]

		ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
		err = target.SendTransaction(ctx, *tx)
		suite.Require().Nil(err)
		cancel()

		txIDs = append(txIDs, convert.IDFromSDK(tx.ID()))
	}

	// wait for the transactions to be included (5/6 nodes can make progress)
	suite.AwaitTransactionsIncluded(txIDs...)

	// restart the paused collector
	err = col1.Start()
	suite.Require().Nil(err)

	// TODO check that it has recovered
	// should be able to check this be looking for its signature on a
	// sufficiently high block proposal
}
