package collection

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/integration/testnet"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestRecovery(t *testing.T) {
	suite.Run(t, new(RecoverySuite))
}

type RecoverySuite struct {
	CollectorSuite
}

// Start consensus, pause a node, allow consensus to continue, then restart
// the node. It should be able to catch up.
func (suite *RecoverySuite) TestProposal_Recovery() {
	t := suite.T()

	// TODO this doesn't quite work with network disconnect/connect for some
	// reason, skipping for now
	unittest.SkipUnless(t, unittest.TEST_TODO, "incomplete implementation - disconnection of one node by disconnecting from Docker network is unreliable")

	t.Logf("%v ================> START TESTING %v", time.Now().UTC(), t.Name())

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
