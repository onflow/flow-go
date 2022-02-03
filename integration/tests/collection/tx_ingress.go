package collection

import (
	"context"
	"testing"
	"time"

	sdk "github.com/onflow/flow-go-sdk"
	sdkclient "github.com/onflow/flow-go-sdk/client"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func logStartFinish(fn func(*testing.T)) func(*testing.T) {
	return func(t *testing.T) {
		t.Logf("%v ================> START RUN TESTING %v", time.Now().UTC(), t.Name())
		fn(t)
		t.Logf("%v ================> FINISH RUN TESTING %v", time.Now().UTC(), t.Name())
	}
}

type IngressSuite struct {
	CollectorSuite
}

// Test sending various invalid transactions to a single-cluster configuration.
// The transactions should be rejected by the collection node and not included
// in any collection.
func (suite *IngressSuite) TestTransactionIngress_InvalidTransaction() {
	t := suite.T()
	t.Logf("%v ================> START TESTING %v", time.Now().UTC(), t.Name())

	suite.SetupTest("col_txingress_invalid", 3, 1)

	// pick a collector to test against
	col1 := suite.Collector(0, 0)

	client, err := sdkclient.New(col1.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure()) //nolint:staticcheck
	require.Nil(t, err)

	t.Run("missing reference block id", logStartFinish(func(t *testing.T) {
		malformed := suite.NextTransaction(func(tx *sdk.Transaction) {
			tx.SetReferenceBlockID(sdk.EmptyID)
		})

		ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
		defer cancel()
		err := client.SendTransaction(ctx, *malformed)
		suite.Assert().Error(err)
	}))

	t.Run("missing script", logStartFinish(func(t *testing.T) {
		malformed := suite.NextTransaction(func(tx *sdk.Transaction) {
			tx.SetScript(nil)
		})

		expected := access.IncompleteTransactionError{
			MissingFields: []string{flow.TransactionFieldScript.String()},
		}

		ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
		defer cancel()
		err := client.SendTransaction(ctx, *malformed)
		unittest.AssertErrSubstringMatch(t, expected, err)
	}))
	t.Run("expired transaction", logStartFinish(func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		unittest.SkipUnless(t, unittest.TEST_WIP, "blocked by https://github.com/dapperlabs/flow-go/issues/3005")
	}))
	t.Run("non-existent reference block ID", logStartFinish(func(t *testing.T) {
		// TODO blocked by https://github.com/dapperlabs/flow-go/issues/3005
		unittest.SkipUnless(t, unittest.TEST_WIP, "blocked by https://github.com/dapperlabs/flow-go/issues/3005")
	}))
	t.Run("unparseable script", logStartFinish(func(t *testing.T) {
		// TODO script parsing not implemented
		unittest.SkipUnless(t, unittest.TEST_WIP, "script parsing not implemented")
	}))
	t.Run("invalid signature", logStartFinish(func(t *testing.T) {
		// TODO signature validation not implemented
		unittest.SkipUnless(t, unittest.TEST_WIP, "signature validation not implemented")
	}))
	t.Run("invalid sequence number", logStartFinish(func(t *testing.T) {
		// TODO nonce validation not implemented
		unittest.SkipUnless(t, unittest.TEST_WIP, "nonce validation not implemented")
	}))
	t.Run("insufficient payer balance", logStartFinish(func(t *testing.T) {
		// TODO balance checking not implemented
		unittest.SkipUnless(t, unittest.TEST_WIP, "balance checking not implemented")
	}))
}

// Test sending a single valid transaction to a single cluster.
// The transaction should be included in a collection.
func (suite *IngressSuite) TestTxIngress_SingleCluster() {
	t := suite.T()
	t.Logf("%v ================> START TESTING %v", time.Now().UTC(), t.Name())

	suite.SetupTest("col_txingress_singlecluster", 3, 1)

	// pick a collector to test against
	col1 := suite.Collector(0, 0)

	client, err := sdkclient.New(col1.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure()) //nolint:staticcheck
	require.Nil(t, err)

	tx := suite.NextTransaction()
	require.Nil(t, err)

	t.Log("sending transaction: ", tx.ID())

	ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
	err = client.SendTransaction(ctx, *tx)
	cancel()
	assert.Nil(t, err)
	t.Log("sent transaction: ", tx.ID())

	// wait for the transaction to be included in a collection
	suite.AwaitTransactionsIncluded(convert.IDFromSDK(tx.ID()))

	t.Log("stopping containers")
	suite.net.StopContainers()

	state := suite.ClusterStateFor(col1.Config.NodeID)

	// the transaction should be included in exactly one collection
	checker := unittest.NewClusterStateChecker(state)
	checker.
		ExpectContainsTx(convert.IDFromSDK(tx.ID())).
		ExpectTxCount(1).
		Assert(t)

	t.Logf("finish testing single cluster")
}

// Test sending a single valid transaction to the responsible cluster in a
// multi-cluster configuration
//
// The transaction should not be routed and should be included in exactly one
// collection in only the responsible cluster.
func (suite *IngressSuite) TestTxIngressMultiCluster_CorrectCluster() {
	t := suite.T()
	t.Logf("%v ================> START TESTING %v", time.Now().UTC(), t.Name())

	// NOTE: we use 3-node clusters so that proposal messages are sent 1-K
	// as 1-1 messages are not picked up by the ghost node.
	const (
		nNodes    uint = 6
		nClusters uint = 2
	)

	suite.SetupTest("col_txingress_multicluster_correctcluster", nNodes, nClusters)

	clusters := suite.Clusters()

	// pick a cluster to target
	targetCluster, ok := clusters.ByIndex(0)
	require.True(suite.T(), ok)
	targetNode := suite.Collector(0, 0)

	// get a client pointing to the cluster member
	client, err := sdkclient.New(targetNode.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure()) //nolint:staticcheck
	require.Nil(t, err)

	tx := suite.TxForCluster(targetCluster)

	// submit the transaction
	ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
	err = client.SendTransaction(ctx, *tx)
	cancel()
	require.NoError(t, err)

	// wait for the transaction to be included in a collection
	suite.AwaitTransactionsIncluded(convert.IDFromSDK(tx.ID()))
	suite.net.StopContainers()

	// ensure the transaction IS included in target cluster collection
	for _, id := range targetCluster.NodeIDs() {
		state := suite.ClusterStateFor(id)

		// the transaction should be included exactly once
		checker := unittest.NewClusterStateChecker(state)
		checker.
			ExpectContainsTx(convert.IDFromSDK(tx.ID())).
			ExpectTxCount(1).
			Assert(t)
	}

	// ensure the transaction IS NOT included in other cluster collections
	for _, cluster := range clusters {
		// skip the target cluster
		if cluster.Fingerprint() == targetCluster.Fingerprint() {
			continue
		}

		for _, id := range cluster.NodeIDs() {
			state := suite.ClusterStateFor(id)

			// the transaction should not be included
			checker := unittest.NewClusterStateChecker(state)
			checker.
				ExpectOmitsTx(convert.IDFromSDK(tx.ID())).
				ExpectTxCount(0).
				Assert(t)
		}
	}
}

// Test sending a single valid transaction to a non-responsible cluster in a
// multi-cluster configuration
//
// The transaction should be routed to the responsible cluster and should be
// included in a collection in only the responsible cluster's state.
func (suite *IngressSuite) TestTxIngressMultiCluster_OtherCluster() {
	t := suite.T()
	t.Logf("%v ================> START TESTING %v", time.Now().UTC(), t.Name())

	// NOTE: we use 3-node clusters so that proposal messages are sent 1-K
	// as 1-1 messages are not picked up by the ghost node.
	const (
		nNodes    uint = 6
		nClusters uint = 2
	)

	suite.SetupTest("col_txingress_multicluster_othercluster", nNodes, nClusters)

	clusters := suite.Clusters()

	// pick a cluster to target
	// this cluster is responsible for the transaction
	targetCluster, ok := clusters.ByIndex(0)
	require.True(suite.T(), ok)

	// pick 1 node from the other cluster to send the transaction to
	otherNode := suite.Collector(1, 0)

	// create clients pointing to each other node
	client, err := sdkclient.New(otherNode.Addr(testnet.ColNodeAPIPort), grpc.WithInsecure()) //nolint:staticcheck
	require.Nil(t, err)

	// create a transaction that will be routed to the target cluster
	tx := suite.TxForCluster(targetCluster)

	// submit the transaction to the other NON-TARGET cluster and retry
	// several times to give the mesh network a chance to form
	go func() {
		for i := 0; i < 10; i++ {
			select {
			case <-suite.ctx.Done():
				// exit when suite is finished
				return
			case <-time.After(100 * time.Millisecond << time.Duration(i)):
				// retry on an exponential backoff
			}

			ctx, cancel := context.WithTimeout(suite.ctx, defaultTimeout)
			_ = client.SendTransaction(ctx, *tx)
			cancel()
		}
	}()

	// wait for the transaction to be included in a collection
	suite.AwaitTransactionsIncluded(convert.IDFromSDK(tx.ID()))
	suite.net.StopContainers()

	// ensure the transaction IS included in target cluster collection
	for _, id := range targetCluster.NodeIDs() {
		state := suite.ClusterStateFor(id)

		// the transaction should be included exactly once
		checker := unittest.NewClusterStateChecker(state)
		checker.
			ExpectContainsTx(convert.IDFromSDK(tx.ID())).
			ExpectTxCount(1).
			Assert(t)
	}

	// ensure the transaction IS NOT included in other cluster collections
	for _, cluster := range clusters {
		// skip the target cluster
		if cluster.Fingerprint() == targetCluster.Fingerprint() {
			continue
		}

		for _, id := range cluster.NodeIDs() {
			state := suite.ClusterStateFor(id)

			// the transaction should not be included
			checker := unittest.NewClusterStateChecker(state)
			checker.
				ExpectOmitsTx(convert.IDFromSDK(tx.ID())).
				ExpectTxCount(0).
				Assert(t)
		}
	}
}
