package collection

import (
	"context"

	"github.com/onflow/flow-go-sdk/client"
	"google.golang.org/grpc"

	"github.com/dapperlabs/flow-go/integration/convert"
	"github.com/dapperlabs/flow-go/integration/testnet"
	"github.com/dapperlabs/flow-go/model/flow"
)

// Create some collections containing transactions. Then delete one of the
// node's database, requiring it to recover the missing blocks.
func (suite *CollectorSuite) TestProposal_Recovery() {

	var (
		nNodes        = 3
		nTransactions = 10
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

	// drop the paused node's database
	db, err := col1.DB()
	suite.Require().Nil(err)
	err = db.DropAll()
	suite.Require().Nil(err)

	// restart the paused collector
	err = col1.Start()
	suite.Require().Nil(err)

	// wait for all the transactions to be included
	// this requires the paused collector to recover existing blocks
	suite.AwaitTransactionsIncluded(txIDs...)
}
