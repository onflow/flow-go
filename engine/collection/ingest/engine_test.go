package ingest

import (
	"errors"
	"io/ioutil"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	"github.com/dapperlabs/flow-go/module/metrics"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	realprotocol "github.com/dapperlabs/flow-go/state/protocol"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/storage"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	N_COLLECTORS int
	N_CLUSTERS   uint

	con  *network.Conduit
	me   *module.Local
	conf Config

	pool *mempool.Transactions

	identities flow.IdentityList
	clusters   flow.ClusterList
	state      *protocol.State
	snapshot   *protocol.Snapshot
	epochQuery *protocol.EpochQuery
	epoch      *protocol.Epoch
	root       *flow.Block

	// backend for mocks
	transactions map[flow.Identifier]*flow.TransactionBody
	blocks       map[flow.Identifier]*flow.Block
	final        *flow.Block

	engine *Engine
}

func TestIngest(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	var err error

	suite.N_COLLECTORS = 4
	suite.N_CLUSTERS = 2

	log := zerolog.New(ioutil.Discard)
	metrics := metrics.NewNoopCollector()

	net := new(module.Network)
	suite.con = new(network.Conduit)
	net.On("Register", mock.Anything, mock.Anything).Return(suite.con, nil).Once()

	collectors := unittest.IdentityListFixture(suite.N_COLLECTORS, unittest.WithRole(flow.RoleCollection))
	me := collectors[0]
	others := unittest.IdentityListFixture(4, unittest.WithAllRolesExcept(flow.RoleCollection))
	suite.identities = append(collectors, others...)

	suite.me = new(module.Local)
	suite.me.On("NodeID").Return(me.NodeID)

	suite.pool = new(mempool.Transactions)
	suite.transactions = make(map[flow.Identifier]*flow.TransactionBody)
	suite.pool.On("Add", mock.Anything).Run(
		func(args mock.Arguments) {
			tx := args[0].(*flow.TransactionBody)
			suite.transactions[tx.ID()] = tx
		}).Return(true)
	suite.pool.On("Has", mock.Anything).Return(
		func(txID flow.Identifier) bool {
			_, exists := suite.transactions[txID]
			return exists
		})

	assignments := unittest.ClusterAssignment(suite.N_CLUSTERS, collectors)
	suite.clusters, err = flow.NewClusterList(assignments, collectors)
	suite.Require().Nil(err)

	suite.root = unittest.GenesisFixture(suite.identities)
	suite.final = suite.root
	suite.blocks = make(map[flow.Identifier]*flow.Block)
	suite.blocks[suite.root.ID()] = suite.root

	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.state.On("Final").Return(suite.snapshot)
	suite.snapshot.On("Head").Return(
		func() *flow.Header { return suite.final.Header },
		func() error { return nil },
	)
	suite.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realprotocol.Snapshot {
			snap := new(protocol.Snapshot)
			block, ok := suite.blocks[blockID]
			if ok {
				snap.On("Head").Return(block.Header, nil)
			} else {
				snap.On("Head").Return(nil, storage.ErrNotFound)
			}
			return snap
		})

	suite.epochQuery = new(protocol.EpochQuery)
	suite.epoch = new(protocol.Epoch)
	suite.snapshot.On("Epochs").Return(suite.epochQuery)
	suite.epochQuery.On("Current").Return(suite.epoch)
	suite.epoch.On("Clustering").Return(suite.clusters, nil)

	suite.conf = DefaultConfig()
	suite.engine, err = New(log, net, suite.state, metrics, metrics, suite.me, suite.pool, suite.conf)
	suite.Require().Nil(err)
}

func (suite *Suite) TestInvalidTransaction() {

	suite.Run("missing field", func() {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.root.ID()
		tx.Script = nil

		err := suite.engine.ProcessLocal(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &IncompleteTransactionError{}))
	})

	suite.Run("gas limit exceeds the maximum allowed", func() {
		tx := unittest.TransactionBodyFixture()
		tx.GasLimit = flow.DefaultMaxGasLimit + 1

		err := suite.engine.ProcessLocal(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &GasLimitExceededError{}))
	})

	suite.Run("invalid reference block ID", func() {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = unittest.IdentifierFixture()

		err := suite.engine.ProcessLocal(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &ErrUnknownReferenceBlock))
	})

	suite.Run("un-parseable script", func() {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.root.ID()
		tx.Script = []byte("definitely a real transaction")

		err := suite.engine.ProcessLocal(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &InvalidScriptError{}))
	})

	suite.Run("invalid signature", func() {
		// TODO cannot check signatures in MVP
		suite.T().Skip()
	})

	suite.Run("expired reference block ID", func() {

		// "finalize" a sufficiently high block that root block is expired
		final := unittest.BlockFixture()
		final.Header.Height = suite.root.Header.Height + flow.DefaultTransactionExpiry + 1
		suite.final = &final

		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.root.ID()

		err := suite.engine.ProcessLocal(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &ExpiredTransactionError{}))
	})
}

// should store transactions for local cluster and propagate to other cluster members
func (suite *Suite) TestRoutingLocalCluster() {

	local, _, ok := suite.clusters.ByNodeID(suite.me.NodeID())
	suite.Require().True(ok)

	// get a transaction that will be routed to local cluster
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()
	tx = unittest.AlterTransactionForCluster(tx, suite.clusters, local, func(transaction *flow.TransactionBody) {})

	// should route to other node in local cluster
	suite.con.
		On("Multicast", &tx, suite.conf.PropagationRedundancy+1, mock.Anything).
		Return(nil)

	err := suite.engine.ProcessLocal(&tx)
	suite.Assert().Nil(err)

	// should be added to local mempool
	stored, ok := suite.transactions[tx.ID()]
	suite.Assert().True(ok)
	suite.Assert().Equal(tx.ID(), stored.ID())
	suite.con.AssertExpectations(suite.T())
}

// should not store transactions for a different cluster and should propagate
// to the responsible cluster
func (suite *Suite) TestRoutingRemoteCluster() {

	// find a remote cluster
	_, index, ok := suite.clusters.ByNodeID(suite.me.NodeID())
	suite.Require().True(ok)
	remote, ok := suite.clusters.ByIndex((index + 1) % suite.N_CLUSTERS)
	suite.Require().True(ok)

	// get a transaction that will be routed to local cluster
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()
	tx = unittest.AlterTransactionForCluster(tx, suite.clusters, remote, func(transaction *flow.TransactionBody) {})

	// should route to both nodes in remote cluster
	suite.con.
		On("Multicast", &tx, suite.conf.PropagationRedundancy+1, mock.Anything).Run(
		func(args mock.Arguments) {
			selector := args.Get(2).(flow.IdentityFilter)
			suite.Assert().Equal(len(remote), len(remote.Filter(selector)))
		}).
		Return(nil)

	err := suite.engine.ProcessLocal(&tx)
	suite.Assert().Nil(err)

	// should not be added to local mempool
	_, ok = suite.transactions[tx.ID()]
	suite.Assert().False(ok)
	suite.con.AssertExpectations(suite.T())
}

// should not propagate transactions received from another node (that node is
// responsible for propagation)
func (suite *Suite) TestRoutingLocalClusterFromOtherNode() {

	local, _, ok := suite.clusters.ByNodeID(suite.me.NodeID())
	suite.Require().True(ok)

	// another node will send us the transaction
	sender := local.Filter(filter.Not(filter.HasNodeID(suite.me.NodeID())))[0]

	// get a transaction that will be routed to local cluster
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()
	tx = unittest.AlterTransactionForCluster(tx, suite.clusters, local, func(transaction *flow.TransactionBody) {})

	// should not route to any node
	suite.con.AssertNotCalled(suite.T(), "Multicast", &tx, suite.conf.PropagationRedundancy+1, mock.Anything)

	err := suite.engine.Process(sender.NodeID, &tx)
	suite.Assert().Nil(err)

	// should be added to local mempool
	stored, ok := suite.transactions[tx.ID()]
	suite.Assert().True(ok)
	suite.Assert().Equal(tx.ID(), stored.ID())
	suite.con.AssertExpectations(suite.T())
}

// should not route or store invalid transactions
func (suite *Suite) TestRoutingInvalidTransaction() {

	// find a remote cluster
	_, index, ok := suite.clusters.ByNodeID(suite.me.NodeID())
	suite.Require().True(ok)
	remote, ok := suite.clusters.ByIndex((index + 1) % suite.N_CLUSTERS)
	suite.Require().True(ok)

	// get transaction for target cluster, but make it invalid
	tx := unittest.TransactionBodyFixture()
	tx = unittest.AlterTransactionForCluster(tx, suite.clusters, remote,
		func(tx *flow.TransactionBody) {
			tx.GasLimit = 0
		})

	// should not route to any node
	suite.con.AssertNotCalled(suite.T(), "Multicast", tx, suite.conf.PropagationRedundancy+1, mock.Anything)

	_ = suite.engine.ProcessLocal(&tx)

	// should not be added to local mempool
	_, ok = suite.transactions[tx.ID()]
	suite.Assert().False(ok)
	suite.con.AssertExpectations(suite.T())
}
