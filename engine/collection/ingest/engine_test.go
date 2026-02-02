package ingest

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"golang.org/x/time/rate"

	"github.com/onflow/flow-go/access/validator"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/factory"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network"
	mocknetwork "github.com/onflow/flow-go/network/mock"
	realprotocol "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/mocks"
)

type Suite struct {
	suite.Suite

	N_COLLECTORS int
	N_CLUSTERS   uint

	conduit *mocknetwork.Conduit
	me      *module.Local
	conf    Config

	pools *epochs.TransactionPools

	identities flow.IdentityList
	clusters   flow.ClusterList

	state      *protocol.State
	snapshot   *protocol.Snapshot
	epochQuery *mocks.EpochQuery
	root       *flow.Block

	// backend for mocks
	blocks map[flow.Identifier]*flow.Block
	final  *flow.Block

	engine *Engine
}

func TestIngest(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	var err error

	suite.N_COLLECTORS = 4
	suite.N_CLUSTERS = 2

	log := zerolog.New(io.Discard)
	metrics := metrics.NewNoopCollector()

	net := new(mocknetwork.EngineRegistry)
	suite.conduit = new(mocknetwork.Conduit)
	net.On("Register", mock.Anything, mock.Anything).Return(suite.conduit, nil).Once()

	collectors := unittest.IdentityListFixture(suite.N_COLLECTORS, unittest.WithRole(flow.RoleCollection))
	me := collectors[0]
	others := unittest.IdentityListFixture(4, unittest.WithAllRolesExcept(flow.RoleCollection))
	suite.identities = append(collectors, others...)

	suite.me = new(module.Local)
	suite.me.On("NodeID").Return(me.NodeID)

	suite.pools = epochs.NewTransactionPools(func(_ uint64) mempool.Transactions {
		return herocache.NewTransactions(1000, log, metrics)
	})

	assignments := unittest.ClusterAssignment(suite.N_CLUSTERS, collectors.ToSkeleton())
	suite.clusters, err = factory.NewClusterList(assignments, collectors.ToSkeleton())
	suite.Require().NoError(err)

	suite.root = unittest.Block.Genesis(flow.Emulator)
	suite.final = suite.root
	suite.blocks = make(map[flow.Identifier]*flow.Block)
	suite.blocks[suite.root.ID()] = suite.root

	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.state.On("Final").Return(suite.snapshot)
	suite.snapshot.On("Head").Return(
		func() *flow.Header { return suite.final.ToHeader() },
		func() error { return nil },
	)
	suite.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) realprotocol.Snapshot {
			snap := new(protocol.Snapshot)
			block, ok := suite.blocks[blockID]
			if ok {
				snap.On("Head").Return(block.ToHeader(), nil)
			} else {
				snap.On("Head").Return(nil, storage.ErrNotFound)
			}
			snap.On("Epochs").Return(suite.epochQuery)
			return snap
		})

	// set up the current epoch by default, with counter=1
	epoch := new(protocol.CommittedEpoch)
	epoch.On("Counter").Return(uint64(1), nil)
	epoch.On("Clustering").Return(suite.clusters, nil)
	suite.epochQuery = mocks.NewEpochQuery(suite.T(), 1, epoch)

	suite.conf = DefaultConfig()
	chain := flow.Testnet.Chain()
	suite.engine, err = New(log, net, suite.state, metrics, metrics, metrics, suite.me, chain, suite.pools, suite.conf, NewAddressRateLimiter(rate.Limit(1), 1))
	suite.Require().NoError(err)
}

func (suite *Suite) TestInvalidTransaction() {

	suite.Run("missing field", func() {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.root.ID()
		tx.Script = nil

		err := suite.engine.ProcessTransaction(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &validator.IncompleteTransactionError{}))
	})

	suite.Run("gas limit exceeds the maximum allowed", func() {
		tx := unittest.TransactionBodyFixture()
		tx.Payer = unittest.RandomAddressFixture()
		tx.ReferenceBlockID = suite.root.ID()
		tx.GasLimit = flow.DefaultMaxTransactionGasLimit + 1

		err := suite.engine.ProcessTransaction(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &validator.InvalidGasLimitError{}))
	})

	suite.Run("invalid reference block ID", func() {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = unittest.IdentifierFixture()

		err := suite.engine.ProcessTransaction(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &engine.UnverifiableInputError{}))
	})

	suite.Run("un-parseable script", func() {
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.root.ID()
		tx.Script = []byte("definitely a real transaction")

		err := suite.engine.ProcessTransaction(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &validator.InvalidScriptError{}))
	})

	// In some cases the Cadence parser will panic rather than return an error.
	// If this happens, we should recover from the panic and return an InvalidScriptError.
	// See: https://github.com/onflow/cadence/issues/3428, https://github.com/dapperlabs/flow-go/issues/6964
	suite.Run("transaction script exceeds parse token limit (Cadence parser panic should be caught)", func() {
		const tokenLimit = 1 << 19
		script := "{};"
		for len(script) < tokenLimit {
			script += script
		}

		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.root.ID()
		tx.Script = []byte("transaction { execute {" + script + "}}")

		err := suite.engine.ProcessTransaction(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &validator.InvalidScriptError{}))
	})

	suite.Run("invalid signature format", func() {
		signer := flow.Testnet.Chain().ServiceAddress()
		keyIndex := uint32(0)

		sig1 := unittest.TransactionSignatureFixture()
		sig1.KeyIndex = keyIndex
		sig1.Address = signer
		sig1.SignerIndex = 0

		sig2 := unittest.TransactionSignatureFixture()
		sig2.KeyIndex = keyIndex
		sig2.Address = signer
		sig2.SignerIndex = 1

		suite.Run("invalid format of an envelope signature", func() {
			invalidSig := unittest.InvalidFormatSignature()
			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = suite.root.ID()
			tx.EnvelopeSignatures[0] = invalidSig

			err := suite.engine.ProcessTransaction(&tx)
			suite.Assert().Error(err)
			suite.Assert().True(errors.As(err, &validator.InvalidRawSignatureError{}))
		})

		suite.Run("invalid format of a payload signature", func() {
			invalidSig := unittest.InvalidFormatSignature()
			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = suite.root.ID()
			tx.PayloadSignatures = []flow.TransactionSignature{invalidSig}

			err := suite.engine.ProcessTransaction(&tx)
			suite.Assert().Error(err)
			suite.Assert().True(errors.As(err, &validator.InvalidRawSignatureError{}))
		})

		suite.Run("duplicated signature (envelope only)", func() {
			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = suite.root.ID()
			tx.EnvelopeSignatures = []flow.TransactionSignature{sig1, sig2}
			err := suite.engine.ProcessTransaction(&tx)
			suite.Assert().Error(err)
			suite.Assert().True(errors.As(err, &validator.DuplicatedSignatureError{}))
		})

		suite.Run("duplicated signature (payload only)", func() {
			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = suite.root.ID()
			tx.PayloadSignatures = []flow.TransactionSignature{sig1, sig2}

			err := suite.engine.ProcessTransaction(&tx)
			suite.Assert().Error(err)
			suite.Assert().True(errors.As(err, &validator.DuplicatedSignatureError{}))
		})

		suite.Run("duplicated signature (cross case)", func() {
			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = suite.root.ID()
			tx.PayloadSignatures = []flow.TransactionSignature{sig1}
			tx.EnvelopeSignatures = []flow.TransactionSignature{sig2}

			err := suite.engine.ProcessTransaction(&tx)
			suite.Assert().Error(err)
			suite.Assert().True(errors.As(err, &validator.DuplicatedSignatureError{}))
		})

		suite.Run("missing payer signature", func() {
			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = suite.root.ID()
			tx.Payer = unittest.RandomAddressFixture()
			tx.ProposalKey.Address = signer
			tx.Authorizers = []flow.Address{signer}

			tx.EnvelopeSignatures = []flow.TransactionSignature{sig1}

			err := suite.engine.ProcessTransaction(&tx)

			suite.Assert().True(errors.As(err, &validator.MissingSignatureError{}))
			suite.Assert().Contains(err.Error(), "payer envelope signature is missing")
		})

		suite.Run("missing proposal signature", func() {
			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = suite.root.ID()
			tx.Payer = signer
			tx.ProposalKey.Address = unittest.RandomAddressFixture()
			tx.Authorizers = []flow.Address{signer}

			tx.EnvelopeSignatures = []flow.TransactionSignature{sig1}

			err := suite.engine.ProcessTransaction(&tx)

			suite.Assert().True(errors.As(err, &validator.MissingSignatureError{}))
			suite.Assert().Contains(err.Error(), "proposer signature on either payload or envelope is missing")
		})

		suite.Run("missing authorizer signature", func() {
			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = suite.root.ID()
			tx.Payer = signer
			tx.ProposalKey.Address = signer
			tx.Authorizers = []flow.Address{signer, unittest.RandomAddressFixture()}

			tx.EnvelopeSignatures = []flow.TransactionSignature{sig1}

			err := suite.engine.ProcessTransaction(&tx)

			suite.Assert().True(errors.As(err, &validator.MissingSignatureError{}))
			suite.Assert().Contains(err.Error(), "authorizer signature on either payload or envelope is missing")
		})

		suite.Run("unrelated signature (envelope only)", func() {
			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = suite.root.ID()
			tx.Payer = signer
			tx.ProposalKey.Address = signer
			tx.Authorizers = []flow.Address{signer}

			unrelatedSig := unittest.TransactionSignatureFixture()
			unrelatedSig.Address = unittest.RandomAddressFixture() // unrelated address
			tx.EnvelopeSignatures = []flow.TransactionSignature{sig1, unrelatedSig}

			err := suite.engine.ProcessTransaction(&tx)
			suite.Assert().Error(err)
			suite.Assert().True(errors.As(err, &validator.UnrelatedAccountSignatureError{}))
		})

		suite.Run("unrelated signature (payload only)", func() {
			tx := unittest.TransactionBodyFixture()
			tx.ReferenceBlockID = suite.root.ID()
			tx.Payer = signer
			tx.ProposalKey.Address = signer
			tx.Authorizers = []flow.Address{signer}

			unrelatedSig := unittest.TransactionSignatureFixture()
			unrelatedSig.Address = unittest.RandomAddressFixture() // unrelated address
			tx.PayloadSignatures = []flow.TransactionSignature{sig1, unrelatedSig}

			err := suite.engine.ProcessTransaction(&tx)
			suite.Assert().Error(err)
			suite.Assert().True(errors.As(err, &validator.UnrelatedAccountSignatureError{}))
		})
	})

	suite.Run("invalid signature", func() {
		// TODO cannot check signatures in MVP
		unittest.SkipUnless(suite.T(), unittest.TEST_TODO, "skipping unimplemented test")
	})

	suite.Run("invalid address", func() {
		invalid := unittest.InvalidAddressFixture()
		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.root.ID()
		tx.Payer = invalid

		err := suite.engine.ProcessTransaction(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &validator.InvalidAddressError{}))
	})

	suite.Run("expired reference block ID", func() {
		// "finalize" a sufficiently high block that root block is expired
		suite.final = unittest.BlockFixture(
			unittest.Block.WithHeight(suite.root.Height + flow.DefaultTransactionExpiry + 1),
		)

		tx := unittest.TransactionBodyFixture()
		tx.ReferenceBlockID = suite.root.ID()

		err := suite.engine.ProcessTransaction(&tx)
		suite.Assert().Error(err)
		suite.Assert().True(errors.As(err, &validator.ExpiredTransactionError{}))
	})

}

// should return an error if the engine is shutdown and not processing transactions
func (suite *Suite) TestComponentShutdown() {
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()

	// start then shut down the engine
	parentCtx, cancel := context.WithCancel(context.Background())
	ctx := irrecoverable.NewMockSignalerContext(suite.T(), parentCtx)
	suite.engine.Start(ctx)
	unittest.AssertClosesBefore(suite.T(), suite.engine.Ready(), 10*time.Millisecond)
	cancel()
	unittest.AssertClosesBefore(suite.T(), suite.engine.ShutdownSignal(), 10*time.Millisecond)

	err := suite.engine.ProcessTransaction(&tx)
	suite.Assert().ErrorIs(err, component.ErrComponentShutdown)
}

// should store transactions for local cluster and propagate to other cluster members
func (suite *Suite) TestRoutingLocalCluster() {

	local, _, ok := suite.clusters.ByNodeID(suite.me.NodeID())
	suite.Require().True(ok)

	// get a transaction that will be routed to local cluster
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()
	tx = unittest.AlterTransactionForCluster(tx, suite.clusters, local, func(transaction *flow.TransactionBody) {})

	// should route to local cluster
	suite.conduit.
		On("Multicast", (*messages.TransactionBody)(&tx), suite.conf.PropagationRedundancy+1, local.NodeIDs()[0], local.NodeIDs()[1]).
		Return(nil)

	err := suite.engine.ProcessTransaction(&tx)
	suite.Assert().NoError(err)

	// should be added to local mempool for the current epoch
	currentEpoch, err := suite.epochQuery.Current()
	suite.Assert().NoError(err)
	suite.Assert().True(suite.pools.ForEpoch(currentEpoch.Counter()).Has(tx.ID()))
	suite.conduit.AssertExpectations(suite.T())
}

// should not store transactions for a different cluster and should propagate
// to the responsible cluster
func (suite *Suite) TestRoutingRemoteCluster() {

	// find a remote cluster
	_, index, ok := suite.clusters.ByNodeID(suite.me.NodeID())
	suite.Require().True(ok)
	remote, ok := suite.clusters.ByIndex((index + 1) % suite.N_CLUSTERS)
	suite.Require().True(ok)

	// get a transaction that will be routed to remote cluster
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()
	tx = unittest.AlterTransactionForCluster(tx, suite.clusters, remote, func(transaction *flow.TransactionBody) {})

	// should route to remote cluster
	suite.conduit.
		On("Multicast", (*messages.TransactionBody)(&tx), suite.conf.PropagationRedundancy+1, remote[0].NodeID, remote[1].NodeID).
		Return(nil)

	err := suite.engine.ProcessTransaction(&tx)
	suite.Assert().NoError(err)

	// should not be added to local mempool
	currentEpoch, err := suite.epochQuery.Current()
	suite.Assert().NoError(err)
	suite.Assert().False(suite.pools.ForEpoch(currentEpoch.Counter()).Has(tx.ID()))
	suite.conduit.AssertExpectations(suite.T())
}

// should not store transactions for a different cluster and should not fail when propagating
// to an empty cluster
func (suite *Suite) TestRoutingToRemoteClusterWithNoNodes() {

	// find a remote cluster
	_, index, ok := suite.clusters.ByNodeID(suite.me.NodeID())
	suite.Require().True(ok)

	// set the next cluster to be empty
	emptyIdentityList := flow.IdentitySkeletonList{}
	nextClusterIndex := (index + 1) % suite.N_CLUSTERS
	suite.clusters[nextClusterIndex] = emptyIdentityList

	// get a transaction that will be routed to remote cluster
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()
	tx = unittest.AlterTransactionForCluster(tx, suite.clusters, emptyIdentityList, func(transaction *flow.TransactionBody) {})

	// should attempt route to remote cluster without providing any node ids
	suite.conduit.
		On("Multicast", (*messages.TransactionBody)(&tx), suite.conf.PropagationRedundancy+1).
		Return(network.EmptyTargetList)

	err := suite.engine.ProcessTransaction(&tx)
	suite.Assert().NoError(err)

	// should not be added to local mempool
	currentEpoch, err := suite.epochQuery.Current()
	suite.Assert().NoError(err)
	suite.Assert().False(suite.pools.ForEpoch(currentEpoch.Counter()).Has(tx.ID()))
	suite.conduit.AssertExpectations(suite.T())
}

// should not propagate transactions received from another node (that node is
// responsible for propagation)
func (suite *Suite) TestRoutingLocalClusterFromOtherNode() {

	local, _, ok := suite.clusters.ByNodeID(suite.me.NodeID())
	suite.Require().True(ok)

	// another node will send us the transaction
	sender := local.Filter(filter.Not(filter.HasNodeID[flow.IdentitySkeleton](suite.me.NodeID())))[0]

	// get a transaction that will be routed to local cluster
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()
	tx = unittest.AlterTransactionForCluster(tx, suite.clusters, local, func(transaction *flow.TransactionBody) {})

	// should not route to any node
	suite.conduit.AssertNumberOfCalls(suite.T(), "Multicast", 0)

	err := suite.engine.onTransaction(sender.NodeID, &tx)
	suite.Assert().NoError(err)

	// should be added to local mempool for current epoch
	currentEpoch, err := suite.epochQuery.Current()
	suite.Assert().NoError(err)
	suite.Assert().True(suite.pools.ForEpoch(currentEpoch.Counter()).Has(tx.ID()))
	suite.conduit.AssertExpectations(suite.T())
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
	suite.conduit.AssertNumberOfCalls(suite.T(), "Multicast", 0)

	_ = suite.engine.ProcessTransaction(&tx)

	// should not be added to local mempool
	currentEpoch, err := suite.epochQuery.Current()
	suite.Assert().NoError(err)
	suite.Assert().False(suite.pools.ForEpoch(currentEpoch.Counter()).Has(tx.ID()))
	suite.conduit.AssertExpectations(suite.T())
}

// We should route to the appropriate cluster if our cluster assignment changes
// on an epoch boundary. In this test, the clusters in epoch 2 are the reverse
// of those in epoch 1, and we check that the transaction is routed based on
// the clustering in epoch 2.
func (suite *Suite) TestRouting_ClusterAssignmentChanged() {

	epoch2Clusters := flow.ClusterList{
		suite.clusters[1],
		suite.clusters[0],
	}
	epoch2 := new(protocol.CommittedEpoch)
	epoch2.On("Counter").Return(uint64(2), nil)
	epoch2.On("Clustering").Return(epoch2Clusters, nil)
	// update the mocks to behave as though we have transitioned to epoch 2
	suite.epochQuery.AddCommitted(epoch2)
	suite.epochQuery.Transition()

	// get the local cluster in epoch 2
	epoch2Local, _, ok := epoch2Clusters.ByNodeID(suite.me.NodeID())
	suite.Require().True(ok)

	// get a transaction that will be routed to local cluster
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()
	tx = unittest.AlterTransactionForCluster(tx, epoch2Clusters, epoch2Local, func(transaction *flow.TransactionBody) {})

	// should route to local cluster
	suite.conduit.On("Multicast", (*messages.TransactionBody)(&tx), suite.conf.PropagationRedundancy+1, epoch2Local.NodeIDs()[0], epoch2Local.NodeIDs()[1]).Return(nil).Once()

	err := suite.engine.ProcessTransaction(&tx)
	suite.Assert().NoError(err)

	// should add to local mempool for epoch 2 only
	suite.Assert().True(suite.pools.ForEpoch(2).Has(tx.ID()))
	suite.Assert().False(suite.pools.ForEpoch(1).Has(tx.ID()))
	suite.conduit.AssertExpectations(suite.T())
}

// We will discard all transactions when we aren't assigned to any cluster.
func (suite *Suite) TestRouting_ClusterAssignmentRemoved() {

	// remove ourselves from the cluster assignment for epoch 2
	withoutMe := suite.identities.
		Filter(filter.Not(filter.HasNodeID[flow.Identity](suite.me.NodeID()))).
		Filter(filter.HasRole[flow.Identity](flow.RoleCollection)).ToSkeleton()
	epoch2Assignment := unittest.ClusterAssignment(suite.N_CLUSTERS, withoutMe)
	epoch2Clusters, err := factory.NewClusterList(epoch2Assignment, withoutMe)
	suite.Require().NoError(err)

	epoch2 := new(protocol.CommittedEpoch)
	epoch2.On("Counter").Return(uint64(2), nil)
	epoch2.On("InitialIdentities").Return(withoutMe, nil)
	epoch2.On("Clustering").Return(epoch2Clusters, nil)
	// update the mocks to behave as though we have transitioned to epoch 2
	suite.epochQuery.AddCommitted(epoch2)
	suite.epochQuery.Transition()

	// any transaction is OK here, since we're not in any cluster
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()

	err = suite.engine.ProcessTransaction(&tx)
	suite.Assert().Error(err)

	// should not add to mempool
	suite.Assert().False(suite.pools.ForEpoch(2).Has(tx.ID()))
	suite.Assert().False(suite.pools.ForEpoch(1).Has(tx.ID()))
	// should not propagate
	suite.conduit.AssertNumberOfCalls(suite.T(), "Multicast", 0)
}

// The node is not a participant in epoch 2 and joins in epoch 3. We start the
// test in epoch 2.
//
// Test that the node discards transactions in epoch 2 and handles them
// in epoch 3.
func (suite *Suite) TestRouting_ClusterAssignmentAdded() {

	// EPOCH 2:

	// remove ourselves from the cluster assignment for epoch 2
	withoutMe := suite.identities.
		Filter(filter.Not(filter.HasNodeID[flow.Identity](suite.me.NodeID()))).
		Filter(filter.HasRole[flow.Identity](flow.RoleCollection)).ToSkeleton()
	epoch2Assignment := unittest.ClusterAssignment(suite.N_CLUSTERS, withoutMe)
	epoch2Clusters, err := factory.NewClusterList(epoch2Assignment, withoutMe)
	suite.Require().NoError(err)

	epoch2 := new(protocol.CommittedEpoch)
	epoch2.On("Counter").Return(uint64(2), nil)
	epoch2.On("InitialIdentities").Return(withoutMe, nil)
	epoch2.On("Clustering").Return(epoch2Clusters, nil)
	// update the mocks to behave as though we have transitioned to epoch 2
	suite.epochQuery.AddCommitted(epoch2)
	suite.epochQuery.Transition()

	// any transaction is OK here, since we're not in any cluster
	tx := unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()

	err = suite.engine.ProcessTransaction(&tx)
	suite.Assert().Error(err)

	// should not add to mempool
	suite.Assert().False(suite.pools.ForEpoch(2).Has(tx.ID()))
	suite.Assert().False(suite.pools.ForEpoch(1).Has(tx.ID()))
	// should not propagate
	suite.conduit.AssertNumberOfCalls(suite.T(), "Multicast", 0)

	// EPOCH 3:

	// include ourselves in cluster assignment
	withMe := suite.identities.Filter(filter.HasRole[flow.Identity](flow.RoleCollection)).ToSkeleton()
	epoch3Assignment := unittest.ClusterAssignment(suite.N_CLUSTERS, withMe)
	epoch3Clusters, err := factory.NewClusterList(epoch3Assignment, withMe)
	suite.Require().NoError(err)

	epoch3 := new(protocol.CommittedEpoch)
	epoch3.On("Counter").Return(uint64(3), nil)
	epoch3.On("Clustering").Return(epoch3Clusters, nil)
	// transition to epoch 3
	suite.epochQuery.AddCommitted(epoch3)
	suite.epochQuery.Transition()

	// get the local cluster in epoch 2
	epoch3Local, _, ok := epoch3Clusters.ByNodeID(suite.me.NodeID())
	suite.Require().True(ok)

	// get a transaction that will be routed to local cluster
	tx = unittest.TransactionBodyFixture()
	tx.ReferenceBlockID = suite.root.ID()
	tx = unittest.AlterTransactionForCluster(tx, epoch3Clusters, epoch3Local, func(transaction *flow.TransactionBody) {})

	// should route to local cluster
	suite.conduit.On("Multicast", (*messages.TransactionBody)(&tx), suite.conf.PropagationRedundancy+1, epoch3Local.NodeIDs()[0], epoch3Local.NodeIDs()[1]).Return(nil).Once()

	err = suite.engine.ProcessTransaction(&tx)
	suite.Assert().NoError(err)
}
