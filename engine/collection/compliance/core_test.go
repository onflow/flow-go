package compliance

import (
	"errors"
	"math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/cluster"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	realbuffer "github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	clusterint "github.com/onflow/flow-go/state/cluster"
	clusterstate "github.com/onflow/flow-go/state/cluster/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestComplianceCore(t *testing.T) {
	suite.Run(t, new(ComplianceCoreSuite))
}

type ComplianceCoreSuite struct {
	suite.Suite

	head *cluster.Block
	// storage data
	headerDB map[flow.Identifier]*cluster.Block

	pendingDB  map[flow.Identifier]*cluster.PendingBlock
	childrenDB map[flow.Identifier][]*cluster.PendingBlock

	// mocked dependencies
	state          *clusterstate.MutableState
	snapshot       *clusterstate.Snapshot
	metrics        *metrics.NoopCollector
	headers        *storage.Headers
	pending        *module.PendingClusterBlockBuffer
	hotstuff       *module.HotStuff
	sync           *module.BlockRequester
	voteAggregator *hotstuff.VoteAggregator

	// engine under test
	core *Core
}

func (cs *ComplianceCoreSuite) SetupTest() {
	// seed the RNG
	rand.Seed(time.Now().UnixNano())

	block := unittest.ClusterBlockFixture()
	cs.head = &block

	// initialize the storage data
	cs.headerDB = make(map[flow.Identifier]*cluster.Block)
	cs.pendingDB = make(map[flow.Identifier]*cluster.PendingBlock)
	cs.childrenDB = make(map[flow.Identifier][]*cluster.PendingBlock)

	// store the head header and payload
	cs.headerDB[block.ID()] = cs.head

	// set up header storage mock
	cs.headers = &storage.Headers{}
	cs.headers.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			if header := cs.headerDB[blockID]; header != nil {
				return cs.headerDB[blockID].Header
			}
			return nil
		},
		func(blockID flow.Identifier) error {
			_, exists := cs.headerDB[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	// set up protocol state mock
	cs.state = &clusterstate.MutableState{}
	cs.state.On("Final").Return(
		func() clusterint.Snapshot {
			return cs.snapshot
		},
	)
	cs.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) clusterint.Snapshot {
			return cs.snapshot
		},
	)
	cs.state.On("Extend", mock.Anything).Return(nil)

	// set up protocol snapshot mock
	cs.snapshot = &clusterstate.Snapshot{}
	cs.snapshot.On("Head").Return(
		func() *flow.Header {
			return cs.head.Header
		},
		nil,
	)

	// set up pending module mock
	cs.pending = &module.PendingClusterBlockBuffer{}
	cs.pending.On("Add", mock.Anything, mock.Anything).Return(true)
	cs.pending.On("ByID", mock.Anything).Return(
		func(blockID flow.Identifier) *cluster.PendingBlock {
			return cs.pendingDB[blockID]
		},
		func(blockID flow.Identifier) bool {
			_, ok := cs.pendingDB[blockID]
			return ok
		},
	)
	cs.pending.On("ByParentID", mock.Anything).Return(
		func(blockID flow.Identifier) []*cluster.PendingBlock {
			return cs.childrenDB[blockID]
		},
		func(blockID flow.Identifier) bool {
			_, ok := cs.childrenDB[blockID]
			return ok
		},
	)
	cs.pending.On("DropForParent", mock.Anything).Return()
	cs.pending.On("Size").Return(uint(0))
	cs.pending.On("PruneByView", mock.Anything).Return()

	closed := func() <-chan struct{} {
		channel := make(chan struct{})
		close(channel)
		return channel
	}()

	// set up hotstuff module mock
	cs.hotstuff = &module.HotStuff{}

	cs.voteAggregator = &hotstuff.VoteAggregator{}

	// set up synchronization module mock
	cs.sync = &module.BlockRequester{}
	cs.sync.On("RequestBlock", mock.Anything).Return(nil)
	cs.sync.On("Done", mock.Anything).Return(closed)

	// set up no-op metrics mock
	cs.metrics = metrics.NewNoopCollector()

	// initialize the engine
	core, err := NewCore(
		unittest.Logger(),
		cs.metrics,
		cs.metrics,
		cs.metrics,
		cs.headers,
		cs.state,
		cs.pending,
		cs.voteAggregator,
	)
	require.NoError(cs.T(), err, "engine initialization should pass")

	cs.core = core
	// assign engine with consensus & synchronization
	cs.core.hotstuff = cs.hotstuff
	cs.core.sync = cs.sync
}

func (cs *ComplianceCoreSuite) TestOnBlockProposalValidParent() {

	// create a proposal that directly descends from the latest finalized header
	originID := unittest.IdentifierFixture()
	block := unittest.ClusterBlockWithParent(cs.head)

	proposal := &messages.ClusterBlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	// store the data for retrieval
	cs.headerDB[block.Header.ParentID] = cs.head

	cs.hotstuff.On("SubmitProposal", proposal.Header, cs.head.Header.View).Return()

	// it should be processed without error
	err := cs.core.OnBlockProposal(originID, proposal)
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should submit the proposal to hotstuff
	cs.hotstuff.AssertExpectations(cs.T())
}

func (cs *ComplianceCoreSuite) TestOnBlockProposalValidAncestor() {

	// create a proposal that has two ancestors in the cache
	originID := unittest.IdentifierFixture()
	ancestor := unittest.ClusterBlockWithParent(cs.head)
	parent := unittest.ClusterBlockWithParent(&ancestor)
	block := unittest.ClusterBlockWithParent(&parent)
	proposal := &messages.ClusterBlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	// store the data for retrieval
	cs.headerDB[parent.ID()] = &parent
	cs.headerDB[ancestor.ID()] = &ancestor

	cs.hotstuff.On("SubmitProposal", block.Header, parent.Header.View).Return()

	// it should be processed without error
	err := cs.core.OnBlockProposal(originID, proposal)
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Extend", &block)

	// we should submit the proposal to hotstuff
	cs.hotstuff.AssertExpectations(cs.T())
}

func (cs *ComplianceCoreSuite) TestOnBlockProposalInvalidExtension() {

	// create a proposal that has two ancestors in the cache
	originID := unittest.IdentifierFixture()
	ancestor := unittest.ClusterBlockWithParent(cs.head)
	parent := unittest.ClusterBlockWithParent(&ancestor)
	block := unittest.ClusterBlockWithParent(&parent)
	proposal := &messages.ClusterBlockProposal{
		Header:  block.Header,
		Payload: block.Payload,
	}

	// store the data for retrieval
	cs.headerDB[parent.ID()] = &parent
	cs.headerDB[ancestor.ID()] = &ancestor

	// make sure we fail to extend the state
	*cs.state = clusterstate.MutableState{}
	cs.state.On("Final").Return(
		func() clusterint.Snapshot {
			return cs.snapshot
		},
	)
	cs.state.On("Extend", mock.Anything).Return(errors.New("dummy error"))

	// it should be processed without error
	err := cs.core.OnBlockProposal(originID, proposal)
	require.Error(cs.T(), err, "proposal with invalid extension should fail")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Extend", &block)

	// we should not submit the proposal to hotstuff
	cs.hotstuff.AssertExpectations(cs.T())
}

func (cs *ComplianceCoreSuite) TestProcessBlockAndDescendants() {

	// create three children blocks
	parent := unittest.ClusterBlockWithParent(cs.head)
	proposal := &messages.ClusterBlockProposal{
		Header:  parent.Header,
		Payload: parent.Payload,
	}
	block1 := unittest.ClusterBlockWithParent(&parent)
	block2 := unittest.ClusterBlockWithParent(&parent)
	block3 := unittest.ClusterBlockWithParent(&parent)

	pendingFromBlock := func(block *cluster.Block) *cluster.PendingBlock {
		return &cluster.PendingBlock{
			OriginID: block.Header.ProposerID,
			Header:   block.Header,
			Payload:  block.Payload,
		}
	}

	// create the pending blocks
	pending1 := pendingFromBlock(&block1)
	pending2 := pendingFromBlock(&block2)
	pending3 := pendingFromBlock(&block3)

	// store the parent on disk
	parentID := parent.ID()
	cs.headerDB[parentID] = &parent

	// store the pending children in the cache
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending1)
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending2)
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending3)

	cs.hotstuff.On("SubmitProposal", parent.Header, cs.head.Header.View).Return().Once()
	cs.hotstuff.On("SubmitProposal", block1.Header, parent.Header.View).Return().Once()
	cs.hotstuff.On("SubmitProposal", block2.Header, parent.Header.View).Return().Once()
	cs.hotstuff.On("SubmitProposal", block3.Header, parent.Header.View).Return().Once()

	// execute the connected children handling
	err := cs.core.processBlockAndDescendants(proposal)
	require.NoError(cs.T(), err, "should pass handling children")

	// check that we submitted each child to hotstuff
	cs.hotstuff.AssertExpectations(cs.T())

	// make sure we drop the cache after trying to process
	cs.pending.AssertCalled(cs.T(), "DropForParent", parent.Header.ID())
}

func (cs *ComplianceCoreSuite) TestOnSubmitVote() {
	// create a vote
	originID := unittest.IdentifierFixture()
	vote := messages.ClusterBlockVote{
		BlockID: unittest.IdentifierFixture(),
		View:    rand.Uint64(),
		SigData: unittest.SignatureFixture(),
	}

	cs.voteAggregator.On("AddVote", &model.Vote{
		View:     vote.View,
		BlockID:  vote.BlockID,
		SignerID: originID,
		SigData:  vote.SigData,
	}).Return()

	// execute the vote submission
	err := cs.core.OnBlockVote(originID, &vote)
	require.NoError(cs.T(), err, "block vote should pass")

	// check that submit vote was called with correct parameters
	cs.hotstuff.AssertExpectations(cs.T())
}

func (cs *ComplianceCoreSuite) TestProposalBufferingOrder() {

	// create a proposal that we will not submit until the end
	originID := unittest.IdentifierFixture()
	block := unittest.ClusterBlockWithParent(cs.head)
	missing := &block

	// create a chain of descendants
	var proposals []*cluster.Block
	proposalsLookup := make(map[flow.Identifier]*cluster.Block)
	parent := missing
	for i := 0; i < 3; i++ {
		proposal := unittest.ClusterBlockWithParent(parent)
		proposals = append(proposals, &proposal)
		proposalsLookup[proposal.ID()] = &proposal
		parent = &proposal
	}

	// replace the engine buffer with the real one
	cs.core.pending = realbuffer.NewPendingClusterBlocks()

	// process all of the descendants
	for _, block := range proposals {

		// check that we request the ancestor block each time
		cs.sync.On("RequestBlock", mock.Anything).Once().Run(
			func(args mock.Arguments) {
				ancestorID := args.Get(0).(flow.Identifier)
				assert.Equal(cs.T(), missing.Header.ID(), ancestorID, "should always request root block")
			},
		)

		proposal := &messages.ClusterBlockProposal{
			Header:  block.Header,
			Payload: block.Payload,
		}

		// process and make sure no error occurs (as they are unverifiable)
		err := cs.core.OnBlockProposal(originID, proposal)
		require.NoError(cs.T(), err, "proposal buffering should pass")

		// make sure no block is forwarded to hotstuff
		cs.hotstuff.AssertExpectations(cs.T())
	}

	// check that we submit ech proposal in order
	*cs.hotstuff = module.HotStuff{}
	index := 0
	order := []flow.Identifier{
		missing.Header.ID(),
		proposals[0].Header.ID(),
		proposals[1].Header.ID(),
		proposals[2].Header.ID(),
	}
	cs.hotstuff.On("SubmitProposal", mock.Anything, mock.Anything).Times(4).Run(
		func(args mock.Arguments) {
			header := args.Get(0).(*flow.Header)
			assert.Equal(cs.T(), order[index], header.ID(), "should submit correct header to hotstuff")
			index++
			cs.headerDB[header.ID()] = proposalsLookup[header.ID()]
		},
	)

	missingProposal := &messages.ClusterBlockProposal{
		Header:  missing.Header,
		Payload: missing.Payload,
	}

	proposalsLookup[missing.ID()] = missing

	// process the root proposal
	err := cs.core.OnBlockProposal(originID, missingProposal)
	require.NoError(cs.T(), err, "root proposal should pass")

	// make sure we submitted all four proposals
	cs.hotstuff.AssertExpectations(cs.T())
}
