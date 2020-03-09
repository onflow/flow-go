package proposal_test

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/engine/collection/proposal"
	"github.com/dapperlabs/flow-go/model/cluster"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	mempool "github.com/dapperlabs/flow-go/module/mempool/mock"
	module "github.com/dapperlabs/flow-go/module/mock"
	"github.com/dapperlabs/flow-go/module/trace"
	network "github.com/dapperlabs/flow-go/network/mock"
	protocol "github.com/dapperlabs/flow-go/protocol/mock"
	realstorage "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	state        *protocol.State
	snapshot     *protocol.Snapshot
	mutator      *protocol.Mutator
	me           *module.Local
	net          *module.Network
	con          *network.Conduit
	provider     *network.Engine
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
	tracer, err := trace.NewTracer(log)
	require.NoError(suite.T(), err)

	suite.state = new(protocol.State)
	suite.snapshot = new(protocol.Snapshot)
	suite.mutator = new(protocol.Mutator)
	suite.state.On("Final").Return(suite.snapshot)
	suite.state.On("Mutate").Return(suite.mutator)
	suite.snapshot.On("Head").Return(&flow.Header{}, nil)
	suite.snapshot.On("Identities", mock.Anything).Return(unittest.IdentityListFixture(1), nil)
	suite.me = new(module.Local)
	suite.me.On("NodeID").Return(flow.Identifier{})

	suite.net = new(module.Network)
	suite.con = new(network.Conduit)
	suite.net.On("Register", mock.Anything, mock.Anything).Return(suite.con, nil)

	suite.provider = new(network.Engine)
	suite.pool = new(mempool.Transactions)
	suite.transactions = new(storage.Transactions)
	suite.headers = new(storage.Headers)
	suite.payloads = new(storage.ClusterPayloads)
	suite.builder = new(module.Builder)
	suite.finalizer = new(module.Finalizer)
	suite.cache = new(module.PendingClusterBlockBuffer)
	suite.coldstuff = new(module.ColdStuff)

	eng, err := proposal.New(log, suite.net, suite.me, suite.state, tracer, suite.provider, suite.pool, suite.transactions, suite.headers, suite.payloads, suite.cache)
	require.NoError(suite.T(), err)
	suite.eng = eng.WithConsensus(suite.coldstuff)
}

func (suite *Suite) TestHandleProposal() {
	originID := unittest.IdentifierFixture()
	parent := unittest.ClusterBlockFixture()
	block := unittest.ClusterBlockWithParent(&parent)

	proposal := &messages.ClusterBlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}

	tx := unittest.TransactionBodyFixture()

	// we have already received and stored the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(&parent.Header, nil)
	// we have all transactions
	suite.pool.On("Has", mock.Anything).Return(true)
	// should store transactions
	suite.pool.On("ByID", mock.Anything).Return(&tx, nil)
	suite.transactions.On("Store", mock.Anything).Return(nil)
	// should store payload and header
	suite.payloads.On("Store", mock.Anything, mock.Anything).Return(nil).Once()
	suite.headers.On("Store", mock.Anything).Return(nil).Once()
	// should extend state with new block
	suite.mutator.On("Extend", block.ID()).Return(nil).Once()
	// should submit to consensus algo
	suite.coldstuff.On("SubmitProposal", proposal.Header, parent.View).Once()
	// we don't have any cached children
	suite.cache.On("ByParentID", block.ID()).Return(nil, false)

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Nil(err)

	// assert that the proposal was submitted to consensus algo
	suite.coldstuff.AssertExpectations(suite.T())
}

func (suite *Suite) TestHandleProposalWithUnknownTransactions() {
	originID := unittest.IdentifierFixture()
	parent := unittest.ClusterBlockFixture()
	block := unittest.ClusterBlockWithParent(&parent)

	proposal := &messages.ClusterBlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}

	// we have already received and stored the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(&parent.Header, nil)
	// we are missing some transactions
	suite.pool.On("Has", mock.Anything).Return(false)
	// TODO assert that the missing transaction(s) is requested

	err := suite.eng.Process(originID, proposal)
	suite.Assert().Error(err)

	// should not store block
	suite.headers.AssertNotCalled(suite.T(), "Store", mock.Anything)
	suite.payloads.AssertNotCalled(suite.T(), "Store", mock.Anything, mock.Anything)
	// proposal should not have been submitted to consensus algo
	suite.coldstuff.AssertNotCalled(suite.T(), "SubmitProposal", mock.Anything, mock.Anything)
}

func (suite *Suite) TestHandlePendingProposal() {
	originID := unittest.IdentifierFixture()
	block := unittest.ClusterBlockFixture()

	proposal := &messages.ClusterBlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}

	// we do not have the parent yet
	suite.headers.On("ByBlockID", block.ParentID).Return(nil, realstorage.ErrNotFound)
	// should request parent block
	suite.con.On("Submit", mock.Anything, mock.Anything).Return(nil).Once()
	suite.cache.On("Add", mock.Anything).Return(true).Once()

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
		Header:  &block.Header,
		Payload: &block.Payload,
	}
	tx := unittest.TransactionBodyFixture()

	// we have already received and stored the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(&parent.Header, nil)
	suite.headers.On("ByBlockID", block.ID()).Return(&block.Header, nil)
	// we have all transactions
	suite.pool.On("Has", mock.Anything).Return(true)
	// should store transactions
	suite.pool.On("ByID", mock.Anything).Return(&tx, nil)
	suite.transactions.On("Store", mock.Anything).Return(nil)
	// should store payload and header
	suite.payloads.On("Store", mock.Anything, mock.Anything).Return(nil).Twice()
	suite.headers.On("Store", mock.Anything).Return(nil).Twice()
	// should extend state with new block
	suite.mutator.On("Extend", block.ID()).Return(nil).Once()
	suite.mutator.On("Extend", child.ID()).Return(nil).Once()
	// should submit to consensus algo
	suite.coldstuff.On("SubmitProposal", mock.Anything, mock.Anything).Twice()
	// should return the pending child
	suite.cache.On("ByParentID", block.ID()).Return([]*cluster.PendingBlock{{
		OriginID: unittest.IdentifierFixture(),
		Header:   &child.Header,
		Payload:  &child.Payload,
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
		BlockID:   unittest.IdentifierFixture(),
		View:      0,
		Signature: nil,
	}
	var randomBeaconSig crypto.Signature

	suite.coldstuff.On("SubmitVote", originID, vote.BlockID, vote.View, vote.Signature, randomBeaconSig).Once()

	err := suite.eng.Process(originID, vote)
	suite.Assert().Nil(err)

	suite.coldstuff.AssertExpectations(suite.T())
}
