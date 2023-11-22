package compliance

import (
	"errors"
	"testing"

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
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/state"
	clusterint "github.com/onflow/flow-go/state/cluster"
	clusterstate "github.com/onflow/flow-go/state/cluster/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestComplianceCore(t *testing.T) {
	suite.Run(t, new(CoreSuite))
}

// CoreSuite tests the compliance core logic.
type CoreSuite struct {
	CommonSuite
}

// CommonSuite is shared between compliance core and engine testing.
type CommonSuite struct {
	suite.Suite

	head *cluster.Block
	// storage data
	headerDB map[flow.Identifier]*cluster.Block

	pendingDB  map[flow.Identifier]flow.Slashable[*cluster.Block]
	childrenDB map[flow.Identifier][]flow.Slashable[*cluster.Block]

	// mocked dependencies
	state                     *clusterstate.MutableState
	snapshot                  *clusterstate.Snapshot
	metrics                   *metrics.NoopCollector
	proposalViolationNotifier *hotstuff.ProposalViolationConsumer
	headers                   *storage.Headers
	pending                   *module.PendingClusterBlockBuffer
	hotstuff                  *module.HotStuff
	sync                      *module.BlockRequester
	validator                 *hotstuff.Validator
	voteAggregator            *hotstuff.VoteAggregator
	timeoutAggregator         *hotstuff.TimeoutAggregator

	// engine under test
	core *Core
}

func (cs *CommonSuite) SetupTest() {
	block := unittest.ClusterBlockFixture()
	cs.head = &block

	// initialize the storage data
	cs.headerDB = make(map[flow.Identifier]*cluster.Block)
	cs.pendingDB = make(map[flow.Identifier]flow.Slashable[*cluster.Block])
	cs.childrenDB = make(map[flow.Identifier][]flow.Slashable[*cluster.Block])

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
	cs.headers.On("Exists", mock.Anything).Return(
		func(blockID flow.Identifier) bool {
			_, exists := cs.headerDB[blockID]
			return exists
		}, func(blockID flow.Identifier) error {
			return nil
		})

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
		func(blockID flow.Identifier) flow.Slashable[*cluster.Block] {
			return cs.pendingDB[blockID]
		},
		func(blockID flow.Identifier) bool {
			_, ok := cs.pendingDB[blockID]
			return ok
		},
	)
	cs.pending.On("ByParentID", mock.Anything).Return(
		func(blockID flow.Identifier) []flow.Slashable[*cluster.Block] {
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
	cs.hotstuff = module.NewHotStuff(cs.T())

	cs.validator = hotstuff.NewValidator(cs.T())
	cs.voteAggregator = hotstuff.NewVoteAggregator(cs.T())
	cs.timeoutAggregator = hotstuff.NewTimeoutAggregator(cs.T())

	// set up synchronization module mock
	cs.sync = &module.BlockRequester{}
	cs.sync.On("RequestBlock", mock.Anything, mock.AnythingOfType("uint64")).Return(nil)
	cs.sync.On("Done", mock.Anything).Return(closed)

	// set up no-op metrics mock
	cs.metrics = metrics.NewNoopCollector()

	// set up notifier for reporting protocol violations
	cs.proposalViolationNotifier = hotstuff.NewProposalViolationConsumer(cs.T())

	// initialize the engine
	core, err := NewCore(
		unittest.Logger(),
		cs.metrics,
		cs.metrics,
		cs.metrics,
		cs.metrics,
		cs.proposalViolationNotifier,
		cs.headers,
		cs.state,
		cs.pending,
		cs.sync,
		cs.validator,
		cs.hotstuff,
		cs.voteAggregator,
		cs.timeoutAggregator,
		compliance.DefaultConfig(),
	)
	require.NoError(cs.T(), err, "engine initialization should pass")

	cs.core = core
}

func (cs *CoreSuite) TestOnBlockProposalValidParent() {

	// create a proposal that directly descends from the latest finalized header
	originID := unittest.IdentifierFixture()
	block := unittest.ClusterBlockWithParent(cs.head)

	proposal := messages.NewClusterBlockProposal(&block)

	// store the data for retrieval
	cs.headerDB[block.Header.ParentID] = cs.head

	hotstuffProposal := model.ProposalFromFlow(block.Header)
	cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
	cs.voteAggregator.On("AddBlock", hotstuffProposal).Once()
	cs.hotstuff.On("SubmitProposal", hotstuffProposal)

	// it should be processed without error
	err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
		OriginID: originID,
		Message:  proposal,
	})
	require.NoError(cs.T(), err, "valid block proposal should pass")
}

func (cs *CoreSuite) TestOnBlockProposalValidAncestor() {

	// create a proposal that has two ancestors in the cache
	originID := unittest.IdentifierFixture()
	ancestor := unittest.ClusterBlockWithParent(cs.head)
	parent := unittest.ClusterBlockWithParent(&ancestor)
	block := unittest.ClusterBlockWithParent(&parent)
	proposal := messages.NewClusterBlockProposal(&block)

	// store the data for retrieval
	cs.headerDB[parent.ID()] = &parent
	cs.headerDB[ancestor.ID()] = &ancestor

	hotstuffProposal := model.ProposalFromFlow(block.Header)
	cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
	cs.voteAggregator.On("AddBlock", hotstuffProposal).Once()
	cs.hotstuff.On("SubmitProposal", hotstuffProposal).Once()

	// it should be processed without error
	err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
		OriginID: originID,
		Message:  proposal,
	})
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Extend", &block)
}

func (cs *CoreSuite) TestOnBlockProposalSkipProposalThreshold() {

	// create a proposal which is far enough ahead to be dropped
	originID := unittest.IdentifierFixture()
	block := unittest.ClusterBlockFixture()
	block.Header.Height = cs.head.Header.Height + compliance.DefaultConfig().SkipNewProposalsThreshold + 1
	proposal := unittest.ClusterProposalFromBlock(&block)

	err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
		OriginID: originID,
		Message:  proposal,
	})
	require.NoError(cs.T(), err)

	// block should be dropped - not added to state or cache
	cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything)
	cs.pending.AssertNotCalled(cs.T(), "Add", originID, mock.Anything)
}

// TestOnBlockProposal_FailsHotStuffValidation tests that a proposal which fails HotStuff validation.
//   - should not go through protocol state validation
//   - should not be added to the state
//   - we should not attempt to process its children
//   - we should notify VoteAggregator, for known errors
func (cs *CoreSuite) TestOnBlockProposal_FailsHotStuffValidation() {

	// create a proposal that has two ancestors in the cache
	originID := unittest.IdentifierFixture()
	ancestor := unittest.ClusterBlockWithParent(cs.head)
	parent := unittest.ClusterBlockWithParent(&ancestor)
	block := unittest.ClusterBlockWithParent(&parent)
	proposal := messages.NewClusterBlockProposal(&block)
	hotstuffProposal := model.ProposalFromFlow(block.Header)

	// store the data for retrieval
	cs.headerDB[parent.ID()] = &parent
	cs.headerDB[ancestor.ID()] = &ancestor

	cs.Run("invalid block error", func() {
		// the block fails HotStuff validation
		*cs.validator = *hotstuff.NewValidator(cs.T())
		sentinelError := model.NewInvalidProposalErrorf(hotstuffProposal, "")
		cs.validator.On("ValidateProposal", hotstuffProposal).Return(sentinelError)
		cs.proposalViolationNotifier.On("OnInvalidBlockDetected", flow.Slashable[model.InvalidProposalError]{
			OriginID: originID,
			Message:  sentinelError.(model.InvalidProposalError),
		}).Return().Once()
		// we should notify VoteAggregator about the invalid block
		cs.voteAggregator.On("InvalidBlock", hotstuffProposal).Return(nil)

		// the expected error should be handled within the Core
		err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should not extend the state with the header
		cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})

	cs.Run("view for unknown epoch error", func() {
		// the block fails HotStuff validation
		*cs.validator = *hotstuff.NewValidator(cs.T())
		cs.validator.On("ValidateProposal", hotstuffProposal).Return(model.ErrViewForUnknownEpoch)

		// this error is not expected should raise an exception
		err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.Error(cs.T(), err, "proposal with invalid extension should fail")
		require.NotErrorIs(cs.T(), err, model.ErrViewForUnknownEpoch)

		// we should not extend the state with the header
		cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})

	cs.Run("unexpected error", func() {
		// the block fails HotStuff validation
		unexpectedErr := errors.New("generic unexpected error")
		*cs.validator = *hotstuff.NewValidator(cs.T())
		cs.validator.On("ValidateProposal", hotstuffProposal).Return(unexpectedErr)

		// the error should be propagated
		err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.ErrorIs(cs.T(), err, unexpectedErr)

		// we should not extend the state with the header
		cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})
}

// TestOnBlockProposal_FailsProtocolStateValidation tests processing a proposal which passes HotStuff validation,
// but fails protocol state validation.
//   - should not be added to the state
//   - we should not attempt to process its children
//   - we should notify VoteAggregator, for known errors
func (cs *CoreSuite) TestOnBlockProposal_FailsProtocolStateValidation() {

	// create a proposal that has two ancestors in the cache
	originID := unittest.IdentifierFixture()
	ancestor := unittest.ClusterBlockWithParent(cs.head)
	parent := unittest.ClusterBlockWithParent(&ancestor)
	block := unittest.ClusterBlockWithParent(&parent)
	proposal := messages.NewClusterBlockProposal(&block)
	hotstuffProposal := model.ProposalFromFlow(block.Header)

	// store the data for retrieval
	cs.headerDB[parent.ID()] = &parent
	cs.headerDB[ancestor.ID()] = &ancestor

	// the block passes HotStuff validation
	cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil)

	cs.Run("invalid block", func() {
		// make sure we fail to extend the state
		*cs.state = clusterstate.MutableState{}
		cs.state.On("Final").Return(func() clusterint.Snapshot { return cs.snapshot })
		sentinelErr := state.NewInvalidExtensionError("")
		cs.state.On("Extend", mock.Anything).Return(sentinelErr)
		cs.proposalViolationNotifier.On("OnInvalidBlockDetected", mock.Anything).Run(func(args mock.Arguments) {
			err := args.Get(0).(flow.Slashable[model.InvalidProposalError])
			require.ErrorIs(cs.T(), err.Message, sentinelErr)
			require.Equal(cs.T(), err.Message.InvalidProposal, hotstuffProposal)
			require.Equal(cs.T(), err.OriginID, originID)
		}).Return().Once()
		// we should notify VoteAggregator about the invalid block
		cs.voteAggregator.On("InvalidBlock", hotstuffProposal).Return(nil)

		// the expected error should be handled within the Core
		err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should extend the state with the header
		cs.state.AssertCalled(cs.T(), "Extend", &block)
		// we should not pass the block to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})

	cs.Run("outdated block", func() {
		// make sure we fail to extend the state
		*cs.state = clusterstate.MutableState{}
		cs.state.On("Final").Return(func() clusterint.Snapshot { return cs.snapshot })
		cs.state.On("Extend", mock.Anything).Return(state.NewOutdatedExtensionError(""))

		// the expected error should be handled within the Core
		err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should extend the state with the header
		cs.state.AssertCalled(cs.T(), "Extend", &block)
		// we should not pass the block to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})

	cs.Run("unexpected error", func() {
		// make sure we fail to extend the state
		*cs.state = clusterstate.MutableState{}
		cs.state.On("Final").Return(func() clusterint.Snapshot { return cs.snapshot })
		unexpectedErr := errors.New("unexpected generic error")
		cs.state.On("Extend", mock.Anything).Return(unexpectedErr)

		// it should be processed without error
		err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.ErrorIs(cs.T(), err, unexpectedErr)

		// we should extend the state with the header
		cs.state.AssertCalled(cs.T(), "Extend", &block)
		// we should not pass the block to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything, mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})
}

func (cs *CoreSuite) TestProcessBlockAndDescendants() {

	// create three children blocks
	parent := unittest.ClusterBlockWithParent(cs.head)
	block1 := unittest.ClusterBlockWithParent(&parent)
	block2 := unittest.ClusterBlockWithParent(&parent)
	block3 := unittest.ClusterBlockWithParent(&parent)

	pendingFromBlock := func(block *cluster.Block) flow.Slashable[*cluster.Block] {
		return flow.Slashable[*cluster.Block]{
			OriginID: block.Header.ProposerID,
			Message:  block,
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

	for _, block := range []cluster.Block{parent, block1, block2, block3} {
		hotstuffProposal := model.ProposalFromFlow(block.Header)
		cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
		cs.voteAggregator.On("AddBlock", hotstuffProposal).Once()
		cs.hotstuff.On("SubmitProposal", hotstuffProposal).Once()
	}

	// execute the connected children handling
	err := cs.core.processBlockAndDescendants(flow.Slashable[*cluster.Block]{
		OriginID: unittest.IdentifierFixture(),
		Message:  &parent,
	})
	require.NoError(cs.T(), err, "should pass handling children")

	// check that we submitted each child to hotstuff
	cs.hotstuff.AssertExpectations(cs.T())

	// make sure we drop the cache after trying to process
	cs.pending.AssertCalled(cs.T(), "DropForParent", parent.Header.ID())
}

func (cs *CoreSuite) TestProposalBufferingOrder() {

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
		cs.sync.On("RequestBlock", mock.Anything, mock.AnythingOfType("uint64")).Once().Run(
			func(args mock.Arguments) {
				ancestorID := args.Get(0).(flow.Identifier)
				assert.Equal(cs.T(), missing.Header.ID(), ancestorID, "should always request root block")
			},
		)

		proposal := messages.NewClusterBlockProposal(block)

		// process and make sure no error occurs (as they are unverifiable)
		err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
			OriginID: originID,
			Message:  proposal,
		})
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
	cs.hotstuff.On("SubmitProposal", mock.Anything).Times(4).Run(
		func(args mock.Arguments) {
			header := args.Get(0).(*model.Proposal).Block
			assert.Equal(cs.T(), order[index], header.BlockID, "should submit correct header to hotstuff")
			index++
			cs.headerDB[header.BlockID] = proposalsLookup[header.BlockID]
		},
	)
	cs.voteAggregator.On("AddBlock", mock.Anything).Times(4)
	cs.validator.On("ValidateProposal", mock.Anything).Times(4).Return(nil)

	missingProposal := messages.NewClusterBlockProposal(missing)

	proposalsLookup[missing.ID()] = missing

	// process the root proposal
	err := cs.core.OnBlockProposal(flow.Slashable[*messages.ClusterBlockProposal]{
		OriginID: originID,
		Message:  missingProposal,
	})
	require.NoError(cs.T(), err, "root proposal should pass")

	// make sure we submitted all four proposals
	cs.hotstuff.AssertExpectations(cs.T())
}
