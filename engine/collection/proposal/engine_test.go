package proposal_test

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/collection/proposal"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/network/mocknetwork"
	clusterstate "github.com/onflow/flow-go/state/cluster/mock"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	realstorage "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

type Suite struct {
	suite.Suite

	// protocol state
	proto struct {
		state    *protocol.MutableState
		snapshot *protocol.Snapshot
		query    *protocol.EpochQuery
		epoch    *protocol.Epoch
	}
	// cluster state
	cluster struct {
		state    *clusterstate.MutableState
		snapshot *clusterstate.Snapshot
		params   *clusterstate.Params
	}

	me           *module.Local
	net          *module.Network
	conduit      *mocknetwork.Conduit
	transactions *storage.Transactions
	headers      *storage.Headers
	payloads     *storage.ClusterPayloads
	builder      *module.Builder
	finalizer    *module.Finalizer
	pending      *module.PendingClusterBlockBuffer
	sync         *module.BlockRequester
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
	suite.proto.state = new(protocol.MutableState)
	suite.proto.snapshot = new(protocol.Snapshot)
	suite.proto.query = new(protocol.EpochQuery)
	suite.proto.epoch = new(protocol.Epoch)
	suite.proto.state.On("Final").Return(suite.proto.snapshot)
	suite.proto.snapshot.On("Head").Return(&flow.Header{}, nil)
	suite.proto.snapshot.On("Identities", mock.Anything).Return(unittest.IdentityListFixture(1), nil)
	suite.proto.snapshot.On("Epochs").Return(suite.proto.query)
	suite.proto.query.On("Current").Return(suite.proto.epoch)

	// create a fake cluster
	clusters := flow.ClusterList{flow.IdentityList{me}}
	suite.proto.epoch.On("Clustering").Return(clusters, nil)
	clusterID := flow.ChainID("cluster-id")

	// mock out cluster state
	suite.cluster.state = new(clusterstate.MutableState)
	suite.cluster.snapshot = new(clusterstate.Snapshot)
	suite.cluster.params = new(clusterstate.Params)
	suite.cluster.state.On("Final").Return(suite.cluster.snapshot)
	suite.cluster.snapshot.On("Head").Return(&flow.Header{}, nil)
	suite.cluster.state.On("Params").Return(suite.cluster.params)
	suite.cluster.params.On("ChainID").Return(clusterID, nil)

	suite.me = new(module.Local)
	suite.me.On("NodeID").Return(me.NodeID)

	suite.net = new(module.Network)
	suite.conduit = new(mocknetwork.Conduit)
	suite.net.On("Register", engine.ChannelConsensusCluster(clusterID), mock.Anything).Return(suite.conduit, nil)
	suite.conduit.On("Close").Return(nil).Maybe()

	suite.transactions = new(storage.Transactions)
	suite.headers = new(storage.Headers)
	suite.payloads = new(storage.ClusterPayloads)
	suite.builder = new(module.Builder)
	suite.finalizer = new(module.Finalizer)
	suite.pending = new(module.PendingClusterBlockBuffer)
	suite.pending.On("Size").Return(uint(0))
	suite.pending.On("PruneByHeight", mock.Anything).Return()
	suite.sync = new(module.BlockRequester)
	suite.hotstuff = new(module.HotStuff)

	eng, err := proposal.New(
		log,
		suite.net,
		suite.me,
		metrics,
		metrics,
		metrics,
		suite.proto.state,
		suite.cluster.state,
		suite.transactions,
		suite.headers,
		suite.payloads,
		suite.pending,
	)
	require.NoError(suite.T(), err)
	suite.eng = eng.WithHotStuff(suite.hotstuff).WithSync(suite.sync)
}

func (suite *Suite) TestHandleProposal() {
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
	suite.pending.On("ByID", block.Header.ParentID).Return(nil, false)

	// we have already received and stored the parent
	suite.headers.On("ByBlockID", parent.ID()).Return(parent.Header, nil)
	suite.pending.On("ByID", block.Header.ParentID).Return(nil, false)
	suite.transactions.On("Store", mock.Anything).Return(nil)
	// should store payload and header
	suite.payloads.On("Store", mock.Anything, mock.Anything).Return(nil).Once()
	suite.headers.On("Store", mock.Anything).Return(nil).Once()
	// should extend state with new block
	suite.cluster.state.On("Extend", &block).Return(nil).Once()
	// should submit to consensus algo
	suite.hotstuff.On("SubmitProposal", proposal.Header, parent.Header.View).Once()
	// we don't have any cached children
	suite.pending.On("ByParentID", block.ID()).Return(nil, false)

	chainID, err := suite.cluster.params.ChainID()
	suite.Assert().Nil(err)
	err = suite.eng.Process(engine.ChannelConsensusCluster(chainID), originID, proposal)
	suite.Assert().Nil(err)

	// assert that the proposal was submitted to consensus algo
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

	// we do not have the parent yet
	suite.headers.On("ByBlockID", block.Header.ParentID).Return(nil, realstorage.ErrNotFound)
	suite.pending.On("ByID", block.Header.ParentID).Return(nil, false)
	// should request parent block
	suite.sync.On("RequestBlock", block.Header.ParentID)
	// should add the proposal to pending buffer
	suite.pending.On("Add", originID, proposal).Return(true).Once()

	chainID, err := suite.cluster.params.ChainID()
	suite.Assert().Nil(err)
	err = suite.eng.Process(engine.ChannelConsensusCluster(chainID), originID, proposal)
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

	chainID, err := suite.cluster.params.ChainID()
	suite.Assert().Nil(err)
	err = suite.eng.Process(engine.ChannelConsensusCluster(chainID), originID, proposal)
	suite.Assert().Nil(err)

	// proposal should not have been submitted to consensus algo
	suite.hotstuff.AssertNotCalled(suite.T(), "SubmitProposal")
	// parent block should be requested
	suite.conduit.AssertExpectations(suite.T())
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

	// we have already received and stored the parent
	headersDB[parent.ID()] = parent.Header
	suite.pending.On("ByID", parent.ID()).Return(nil, false)
	// should extend state with new block
	suite.cluster.state.On("Extend", &block).
		Run(func(_ mock.Arguments) {
			// once we add the block to the state, ensure it is retrievable from storage
			headersDB[block.ID()] = block.Header
		}).
		Return(nil).Once()
	suite.cluster.state.On("Extend", &child).Return(nil).Once()
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

	chainID, err := suite.cluster.params.ChainID()
	suite.Assert().Nil(err)
	err = suite.eng.Process(engine.ChannelConsensusCluster(chainID), originID, proposal)
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

	chainID, err := suite.cluster.params.ChainID()
	suite.Assert().Nil(err)
	err = suite.eng.Process(engine.ChannelConsensusCluster(chainID), originID, vote)
	suite.Assert().Nil(err)

	suite.hotstuff.AssertExpectations(suite.T())
}
