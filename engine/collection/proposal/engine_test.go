package proposal_test

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	"github.com/dapperlabs/flow-go/module/metrics"
	module "github.com/dapperlabs/flow-go/module/mock"
	network "github.com/dapperlabs/flow-go/network/mock"
	clusterstate "github.com/dapperlabs/flow-go/state/cluster/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	realstorage "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	// protocol state
	proto struct {
		state    *protocol.State
		snapshot *protocol.Snapshot
		mutator  *protocol.Mutator
	}
	// cluster state
	cluster struct {
		state    *clusterstate.State
		snapshot *clusterstate.Snapshot
		mutator  *clusterstate.Mutator
	}

	me           *module.Local
	net          *module.Network
	con          *network.Conduit
	validator    *module.TransactionValidator
	pool         *mempool.Transactions
	transactions *storage.Transactions
	headers      *storage.Headers
	payloads     *storage.ClusterPayloads
	builder      *module.Builder
	finalizer    *module.Finalizer
	pending      *module.PendingClusterBlockBuffer
	sync         *module.Synchronization
	hotstuff     *module.HotStuff
	eng          *proposal.Engine
}

func TestProposalEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	log := zerolog.New(os.Stderr)
	metrics := metrics.NewNoopCollector()

	me := unittest.IdentityFixture(unittest.WithRole(flow.RoleCollection))

	// mock out protocol state
	suite.proto.state = new(protocol.State)
	suite.proto.snapshot = new(protocol.Snapshot)
	suite.proto.mutator = new(protocol.Mutator)
	suite.proto.state.On("Final").Return(suite.proto.snapshot)
	suite.proto.state.On("Mutate").Return(suite.proto.mutator)
	suite.proto.snapshot.On("Head").Return(&flow.Header{}, nil)
	suite.proto.snapshot.On("Identities", mock.Anything).Return(unittest.IdentityListFixture(1), nil)

	// mock out cluster state
	suite.cluster.state = new(clusterstate.State)
	suite.cluster.snapshot = new(clusterstate.Snapshot)
	suite.cluster.mutator = new(clusterstate.Mutator)
	suite.cluster.state.On("Final").Return(suite.cluster.snapshot)
	suite.cluster.state.On("Mutate").Return(suite.cluster.mutator)
	suite.cluster.snapshot.On("Head").Return(&flow.Header{}, nil)

	// create a fake cluster
	clusters := flow.NewClusterList(1)
	clusters.Add(0, me)
	suite.proto.snapshot.On("Clusters").Return(clusters, nil)

	suite.me = new(module.Local)
	suite.me.On("NodeID").Return(me.NodeID)

	suite.net = new(module.Network)
	suite.con = new(network.Conduit)
	suite.net.On("Register", mock.Anything, mock.Anything).Return(suite.con, nil)

	suite.validator = new(module.TransactionValidator)
	suite.pool = new(mempool.Transactions)
	suite.pool.On("Size").Return(uint(0))
	suite.transactions = new(storage.Transactions)
	suite.headers = new(storage.Headers)
	suite.payloads = new(storage.ClusterPayloads)
	suite.builder = new(module.Builder)
	suite.finalizer = new(module.Finalizer)
	suite.pending = new(module.PendingClusterBlockBuffer)
	suite.pending.On("Size").Return(uint(0))
	suite.pending.On("PruneByHeight", mock.Anything).Return()
	suite.sync = new(module.Synchronization)
	suite.hotstuff = new(module.HotStuff)

	eng, err := proposal.New(log, suite.net, suite.me, metrics, metrics, metrics, suite.proto.state, suite.cluster.state, suite.validator, suite.pool, suite.transactions, suite.headers, suite.payloads, suite.pending)
	require.NoError(suite.T(), err)
	suite.eng = eng.WithConsensus(suite.hotstuff).WithSynchronization(suite.sync)
}

func (suite *Suite) TestHandleProposal() {
	originID := unittest.IdentifierFixture()
	parent := unittest.ClusterBlockFixture()
	block := unittest.ClusterBlockWithParent(&parent)

	proposal := &messages.ClusterBlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	tx := unittest.TransactionBodyFixture()

	// this is a new block
	suite.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound)
	suite.pending.On("ByID", block.ID()).Return(nil, false)
	suite.pending.On("ByID", block.Header.ParentID).Return(nil, false)

	// we have already received and stored the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(parent.Header, nil)
	suite.pending.On("ByID", block.Header.ParentID).Return(nil, false)
	// we have all transactions
	suite.pool.On("Has", mock.Anything).Return(true)
	// should store transactions
	suite.pool.On("ByID", mock.Anything).Return(&tx, true)
	suite.transactions.On("Store", mock.Anything).Return(nil)
	// should store payload and header
	suite.payloads.On("Store", mock.Anything, mock.Anything).Return(nil).Once()
	suite.headers.On("Store", mock.Anything).Return(nil).Once()
	// should extend state with new block
	suite.cluster.mutator.On("Extend", &block).Return(nil).Once()
	// should submit to consensus algo
	suite.hotstuff.On("SubmitProposal", proposal.Header, parent.Header.View).Once()
	// we don't have any cached children
	suite.pending.On("ByParentID", block.ID()).Return(nil, false)

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// assert that the proposal was submitted to consensus algo
	suite.hotstuff.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandleProposalWithUnknownValidTransactions() {
	originID := unittest.IdentifierFixture()
	parent := unittest.ClusterBlockFixture()
	block := unittest.ClusterBlockWithParent(&parent)

	proposal := &messages.ClusterBlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	// this is a new block
	suite.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound)
	suite.pending.On("ByID", block.ID()).Return(nil, false)

	// we have already received and processed the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(parent.Header, nil)
	suite.pending.On("ByID", parent.ID()).Return(nil, false)
	// we are missing all the transactions
	suite.pool.On("Has", mock.Anything).Return(false)
	// the missing transactions should be verified
	for _, tx := range block.Payload.Collection.Transactions {
		// all the transactions are valid
		suite.validator.On("ValidateTransaction", tx).Return(nil).Once()
	}

	// should extend state with new block
	suite.cluster.mutator.On("Extend", &block).Return(nil).Once()
	// should submit to consensus algo
	suite.hotstuff.On("SubmitProposal", proposal.Header, parent.Header.View).Once()
	// we don't have any cached children
	suite.pending.On("ByParentID", block.ID()).Return(nil, false)

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// should store block
	suite.headers.AssertExpectations(suite.T())
	suite.payloads.AssertExpectations(suite.T())
	// transactions should have been validated
	suite.validator.AssertExpectations(suite.T())
	suite.hotstuff.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandlePendingProposal() {
	originID := unittest.IdentifierFixture()
	block := unittest.ClusterBlockFixture()

	proposal := &messages.ClusterBlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	// this is a new block
	suite.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound)
	suite.pending.On("ByID", block.ID()).Return(nil, false)

	// we have all transactions
	suite.pool.On("Has", mock.Anything).Return(true)
	// we do not have the parent yet
	suite.headers.On("ByBlockID", block.Header.ParentID).Return(nil, realstorage.ErrNotFound)
	suite.pending.On("ByID", block.Header.ParentID).Return(nil, false)
	// should request parent block
	suite.sync.On("RequestBlock", block.Header.ParentID)
	// should add the proposal to pending buffer
	suite.pending.On("Add", originID, proposal).Return(true).Once()

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// proposal should not have been submitted to consensus algo
	suite.hotstuff.AssertNotCalled(suite.T(), "SubmitProposal")
	// parent block should be requested
	suite.sync.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandlePendingProposalWithPendingParent() {
	originID := unittest.IdentifierFixture()

	grandparent := unittest.ClusterBlockFixture()           // we are missing this
	parent := unittest.ClusterBlockWithParent(&grandparent) // we have this in the cache
	block := unittest.ClusterBlockWithParent(&parent)       // we receive this as a proposal

	suite.T().Logf("block: %x\nparent: %x\ng-parent: %x", block.ID(), parent.ID(), grandparent.ID())

	proposal := &messages.ClusterBlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	// this is a new block
	suite.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound)
	suite.pending.On("ByID", block.ID()).Return(nil, false)
	// we have all transactions
	suite.pool.On("Has", mock.Anything).Return(true)

	// we have the parent, it is in pending cache
	pendingParent := &cluster.PendingBlock{
		OriginID: originID,
		Header:   parent.Header,
		Payload:  parent.Payload,
	}
	suite.headers.On("ByBlockID", block.Header.ParentID).Return(nil, realstorage.ErrNotFound)

	// should add block to the cache
	suite.pending.On("Add", originID, proposal).Return(true).Once()
	suite.pending.On("ByID", parent.ID()).Return(pendingParent, true).Once()
	suite.pending.On("ByID", grandparent.ID()).Return(nil, false).Once()
	// should send a request for the grandparent
	suite.sync.On("RequestBlock", grandparent.ID())

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// proposal should not have been submitted to consensus algo
	suite.hotstuff.AssertNotCalled(suite.T(), "SubmitProposal")
	// parent block should be requested
	suite.con.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandleProposalWithPendingChildren() {
	originID := unittest.IdentifierFixture()
	parent := unittest.ClusterBlockFixture()
	block := unittest.ClusterBlockWithParent(&parent)
	child := unittest.ClusterBlockWithParent(&block)

	proposal := &messages.ClusterBlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	headersDB := make(map[flow.Identifier]*flow.Header)
	suite.headers.On("ByBlockID", mock.Anything).Return(
		func(id flow.Identifier) *flow.Header {
			return headersDB[id]
		},
		func(id flow.Identifier) error {
			_, exists := headersDB[id]
			if !exists {
				return realstorage.ErrNotFound
			}
			return nil
		},
	)

	// this is a new block
	suite.headers.On("ByBlockID", block.ID()).Return(nil, realstorage.ErrNotFound)
	suite.pending.On("ByID", block.ID()).Return(nil, false)
	// we have all transactions
	suite.pool.On("Has", mock.Anything).Return(true)

	// we have already received and stored the parent
	headersDB[parent.ID()] = parent.Header
	suite.pending.On("ByID", parent.ID()).Return(nil, false)
	// should extend state with new block
	suite.cluster.mutator.On("Extend", &block).
		Run(func(_ mock.Arguments) {
			// once we add the block to the state, ensure it is retrievable from storage
			headersDB[block.ID()] = block.Header
		}).
		Return(nil).Once()
	suite.cluster.mutator.On("Extend", &child).Return(nil).Once()
	// should submit to consensus algo
	suite.hotstuff.On("SubmitProposal", mock.Anything, mock.Anything).Twice()
	// should return the pending child
	suite.pending.On("ByParentID", block.ID()).Return([]*cluster.PendingBlock{{
		OriginID: unittest.IdentifierFixture(),
		Header:   child.Header,
		Payload:  child.Payload,
	}}, true)
	suite.pending.On("DropForParent", block.ID()).Once()
	suite.pending.On("ByParentID", child.ID()).Return(nil, false)

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// assert that the proposal was submitted to consensus algo
	suite.hotstuff.AssertExpectations(suite.T())
}

func (suite *Suite) TestReceiveVote() {

	originID := unittest.IdentifierFixture()
	vote := &messages.ClusterBlockVote{
		BlockID: unittest.IdentifierFixture(),
		View:    0,
		SigData: nil,
	}

	suite.hotstuff.On("SubmitVote", originID, vote.BlockID, vote.View, vote.SigData).Once()

	err := suite.eng.Process(originID, vote)
	suite.Assert().Nil(err)

	suite.hotstuff.AssertExpectations(suite.T())
}
