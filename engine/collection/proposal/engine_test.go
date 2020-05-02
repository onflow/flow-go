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
	cache        *module.PendingClusterBlockBuffer
	eng          *proposal.Engine
	coldstuff    *module.ColdStuff
}

func TestProposalEngine(t *testing.T) {
	suite.Run(t, new(Suite))
}

func (suite *Suite) SetupTest() {
	log := zerolog.New(os.Stderr)
	metrics, err := metrics.NewCollector(log)
	require.NoError(suite.T(), err)

	me := unittest.IdentityFixture(func(idty *flow.Identity) { idty.Role = flow.RoleCollection })

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
	suite.transactions = new(storage.Transactions)
	suite.headers = new(storage.Headers)
	suite.payloads = new(storage.ClusterPayloads)
	suite.builder = new(module.Builder)
	suite.finalizer = new(module.Finalizer)
	suite.cache = new(module.PendingClusterBlockBuffer)
	suite.coldstuff = new(module.ColdStuff)

	eng, err := proposal.New(log, suite.net, suite.me, suite.proto.state, suite.cluster.state, metrics, suite.validator, suite.pool, suite.transactions, suite.headers, suite.payloads, suite.cache)
	require.NoError(suite.T(), err)
	suite.eng = eng.WithConsensus(suite.coldstuff)
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

	// we have already received and stored the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(parent.Header, nil)
	// we have all transactions
	suite.pool.On("Has", mock.Anything).Return(true)
	// should store transactions
	suite.pool.On("ByID", mock.Anything).Return(&tx, nil)
	suite.transactions.On("Store", mock.Anything).Return(nil)
	// should store payload and header
	suite.payloads.On("Store", mock.Anything, mock.Anything).Return(nil).Once()
	suite.headers.On("Store", mock.Anything).Return(nil).Once()
	// should extend state with new block
	suite.cluster.mutator.On("Extend", block.ID()).Return(nil).Once()
	// should submit to consensus algo
	suite.coldstuff.On("SubmitProposal", proposal.Header, parent.Header.View).Once()
	// we don't have any cached children
	suite.cache.On("ByParentID", block.ID()).Return(nil, false)

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// assert that the proposal was submitted to consensus algo
	suite.coldstuff.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandleProposalWithUnknownValidTransactions() {
	originID := unittest.IdentifierFixture()
	parent := unittest.ClusterBlockFixture()
	block := unittest.ClusterBlockWithParent(&parent)

	proposal := &messages.ClusterBlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	// we have already received and stored the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(parent.Header, nil)
	// we are missing all the transactions
	suite.pool.On("Has", mock.Anything).Return(false)
	// the missing transactions should be verified
	for _, tx := range block.Payload.Collection.Transactions {
		// all the transactions are valid
		suite.validator.On("ValidateTransaction", tx).Return(nil).Once()
	}

	// should store payload and header
	suite.payloads.On("Store", mock.Anything, mock.Anything).Return(nil).Once()
	suite.headers.On("Store", mock.Anything).Return(nil).Once()
	// should extend state with new block
	suite.cluster.mutator.On("Extend", block.ID()).Return(nil).Once()
	// should submit to consensus algo
	suite.coldstuff.On("SubmitProposal", proposal.Header, parent.Header.View).Once()
	// we don't have any cached children
	suite.cache.On("ByParentID", block.ID()).Return(nil, false)

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// should store block
	suite.headers.AssertExpectations(suite.T())
	suite.payloads.AssertExpectations(suite.T())
	// transactions should have been validated
	suite.validator.AssertExpectations(suite.T())
	suite.coldstuff.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandlePendingProposal() {
	originID := unittest.IdentifierFixture()
	block := unittest.ClusterBlockFixture()

	proposal := &messages.ClusterBlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	// we do not have the parent yet
	suite.headers.On("ByBlockID", block.Header.ParentID).Return(nil, realstorage.ErrNotFound)
	// should request parent block
	suite.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
	suite.cache.On("Add", mock.Anything).Return(true).Once()
	suite.cache.On("ByID", block.Header.ParentID).Return(nil, false)

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// proposal should not have been submitted to consensus algo
	suite.coldstuff.AssertNotCalled(suite.T(), "SubmitProposal")
	// parent block should be requested
	suite.con.AssertExpectations(suite.T())
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

	// we have the parent, it is in pending cache
	pendingParent := &cluster.PendingBlock{
		OriginID: originID,
		Header:   parent.Header,
		Payload:  parent.Payload,
	}
	suite.headers.On("ByBlockID", block.Header.ParentID).Return(nil, realstorage.ErrNotFound)

	// should add block to the cache
	suite.cache.On("Add", mock.Anything).Return(true).Once()
	suite.cache.On("ByID", parent.ID()).Return(pendingParent, true).Once()
	suite.cache.On("ByID", grandparent.ID()).Return(nil, false).Once()
	// should send a request for the grandparent
	suite.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		// assert the right ID was requested manually as we don't know what nonce was used
		Run(func(args mock.Arguments) {
			req := args.Get(0).(*messages.ClusterBlockRequest)
			suite.Assert().Equal(req.BlockID, grandparent.ID())
		}).
		Return(nil).
		Once()

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// proposal should not have been submitted to consensus algo
	suite.coldstuff.AssertNotCalled(suite.T(), "SubmitProposal")
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
	tx := unittest.TransactionBodyFixture()

	// we have already received and stored the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(parent.Header, nil)
	suite.headers.On("ByBlockID", block.ID()).Return(block.Header, nil)
	// we have all transactions
	suite.pool.On("Has", mock.Anything).Return(true)
	// should store transactions
	suite.pool.On("ByID", mock.Anything).Return(&tx, nil)
	suite.transactions.On("Store", mock.Anything).Return(nil)
	// should store payload and header
	suite.payloads.On("Store", mock.Anything, mock.Anything).Return(nil).Twice()
	suite.headers.On("Store", mock.Anything).Return(nil).Twice()
	// should extend state with new block
	suite.cluster.mutator.On("Extend", block.ID()).Return(nil).Once()
	suite.cluster.mutator.On("Extend", child.ID()).Return(nil).Once()
	// should submit to consensus algo
	suite.coldstuff.On("SubmitProposal", mock.Anything, mock.Anything).Twice()
	// should return the pending child
	suite.cache.On("ByParentID", block.ID()).Return([]*cluster.PendingBlock{{
		OriginID: unittest.IdentifierFixture(),
		Header:   child.Header,
		Payload:  child.Payload,
	}}, true)
	suite.cache.On("DropForParent", block.ID()).Once()
	suite.cache.On("ByParentID", child.ID()).Return(nil, false)

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// assert that the proposal was submitted to consensus algo
	suite.coldstuff.AssertExpectations(suite.T())
}

func (suite *Suite) TestReceiveVote() {

	originID := unittest.IdentifierFixture()
	vote := &messages.ClusterBlockVote{
		BlockID: unittest.IdentifierFixture(),
		View:    0,
		SigData: nil,
	}

	suite.coldstuff.On("SubmitVote", originID, vote.BlockID, vote.View, vote.SigData).Once()

	err := suite.eng.Process(originID, vote)
	suite.Assert().Nil(err)

	suite.coldstuff.AssertExpectations(suite.T())
}
