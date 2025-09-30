package compliance

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	realModule "github.com/onflow/flow-go/module"
	real "github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	mocknetwork "github.com/onflow/flow-go/network/mock"
	"github.com/onflow/flow-go/state"
	protint "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
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

	// engine parameters
	participants flow.IdentityList
	myID         flow.Identifier
	head         *flow.Header

	// storage data
	headerDB   map[flow.Identifier]*flow.Header
	payloadDB  map[flow.Identifier]*flow.Payload
	pendingDB  map[flow.Identifier]flow.Slashable[*flow.Proposal]
	childrenDB map[flow.Identifier][]flow.Slashable[*flow.Proposal]

	// mocked dependencies
	me                        *module.Local
	metrics                   *metrics.NoopCollector
	tracer                    realModule.Tracer
	headers                   *storage.Headers
	payloads                  *storage.Payloads
	state                     *protocol.ParticipantState
	snapshot                  *protocol.Snapshot
	con                       *mocknetwork.Conduit
	net                       *mocknetwork.EngineRegistry
	pending                   *module.PendingBlockBuffer
	hotstuff                  *module.HotStuff
	sync                      *module.BlockRequester
	proposalViolationNotifier *hotstuff.ProposalViolationConsumer
	validator                 *hotstuff.Validator
	voteAggregator            *hotstuff.VoteAggregator
	timeoutAggregator         *hotstuff.TimeoutAggregator

	// engine under test
	core *Core
}

func (cs *CommonSuite) SetupTest() {
	// initialize the paramaters
	cs.participants = unittest.IdentityListFixture(3,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithInitialWeight(1000),
	)
	cs.myID = cs.participants[0].NodeID
	block := unittest.BlockFixture()
	cs.head = block.ToHeader()

	// initialize the storage data
	cs.headerDB = make(map[flow.Identifier]*flow.Header)
	cs.payloadDB = make(map[flow.Identifier]*flow.Payload)
	cs.pendingDB = make(map[flow.Identifier]flow.Slashable[*flow.Proposal])
	cs.childrenDB = make(map[flow.Identifier][]flow.Slashable[*flow.Proposal])

	// store the head header and payload
	cs.headerDB[block.Hash()] = block.ToHeader()
	cs.payloadDB[block.Hash()] = &block.Payload

	// set up local module mock
	cs.me = &module.Local{}
	cs.me.On("NodeID").Return(
		func() flow.Identifier {
			return cs.myID
		},
	)

	// set up header storage mock
	cs.headers = &storage.Headers{}
	cs.headers.On("Store", mock.Anything).Return(
		func(header *flow.Header) error {
			cs.headerDB[header.Hash()] = header
			return nil
		},
	)
	cs.headers.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Header {
			return cs.headerDB[blockID]
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

	// set up payload storage mock
	cs.payloads = &storage.Payloads{}
	cs.payloads.On("Store", mock.Anything, mock.Anything).Return(
		func(header *flow.Header, payload *flow.Payload) error {
			cs.payloadDB[header.Hash()] = payload
			return nil
		},
	)
	cs.payloads.On("ByBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.Payload {
			return cs.payloadDB[blockID]
		},
		func(blockID flow.Identifier) error {
			_, exists := cs.payloadDB[blockID]
			if !exists {
				return storerr.ErrNotFound
			}
			return nil
		},
	)

	// set up protocol state mock
	cs.state = &protocol.ParticipantState{}
	cs.state.On("Final").Return(
		func() protint.Snapshot {
			return cs.snapshot
		},
	)
	cs.state.On("AtBlockID", mock.Anything).Return(
		func(blockID flow.Identifier) protint.Snapshot {
			return cs.snapshot
		},
	)
	cs.state.On("Extend", mock.Anything, mock.Anything).Return(nil)

	// set up protocol snapshot mock
	cs.snapshot = &protocol.Snapshot{}
	cs.snapshot.On("Identities", mock.Anything).Return(
		func(filter flow.IdentityFilter[flow.Identity]) flow.IdentityList {
			return cs.participants.Filter(filter)
		},
		nil,
	)
	cs.snapshot.On("Head").Return(
		func() *flow.Header {
			return cs.head
		},
		nil,
	)

	// set up network conduit mock
	cs.con = &mocknetwork.Conduit{}
	cs.con.On("Publish", mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Publish", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Unicast", mock.Anything, mock.Anything).Return(nil)

	// set up network module mock
	cs.net = &mocknetwork.EngineRegistry{}
	cs.net.On("Register", mock.Anything, mock.Anything).Return(
		func(channel channels.Channel, engine netint.MessageProcessor) netint.Conduit {
			return cs.con
		},
		nil,
	)

	// set up pending module mock
	cs.pending = &module.PendingBlockBuffer{}
	cs.pending.On("Add", mock.Anything, mock.Anything).Return(true)
	cs.pending.On("ByID", mock.Anything).Return(
		func(blockID flow.Identifier) flow.Slashable[*flow.Proposal] {
			return cs.pendingDB[blockID]
		},
		func(blockID flow.Identifier) bool {
			_, ok := cs.pendingDB[blockID]
			return ok
		},
	)
	cs.pending.On("ByParentID", mock.Anything).Return(
		func(blockID flow.Identifier) []flow.Slashable[*flow.Proposal] {
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

	// set up hotstuff module mock
	cs.hotstuff = module.NewHotStuff(cs.T())

	cs.validator = hotstuff.NewValidator(cs.T())
	cs.voteAggregator = hotstuff.NewVoteAggregator(cs.T())
	cs.timeoutAggregator = hotstuff.NewTimeoutAggregator(cs.T())

	// set up synchronization module mock
	cs.sync = &module.BlockRequester{}
	cs.sync.On("RequestBlock", mock.Anything, mock.Anything).Return(nil)
	cs.sync.On("Done", mock.Anything).Return(unittest.ClosedChannel())

	// set up no-op metrics mock
	cs.metrics = metrics.NewNoopCollector()

	// set up no-op tracer
	cs.tracer = trace.NewNoopTracer()

	// set up notifier for reporting protocol violations
	cs.proposalViolationNotifier = hotstuff.NewProposalViolationConsumer(cs.T())

	// initialize the engine
	e, err := NewCore(
		unittest.Logger(),
		cs.metrics,
		cs.metrics,
		cs.metrics,
		cs.metrics,
		cs.proposalViolationNotifier,
		cs.tracer,
		cs.headers,
		cs.payloads,
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

	cs.core = e
}

func (cs *CoreSuite) TestOnBlockProposalValidParent() {

	// create a proposal that directly descends from the latest finalized header
	originID := cs.participants[1].NodeID
	block := unittest.BlockWithParentFixture(cs.head)
	proposal := unittest.ProposalFromBlock(block)

	// store the data for retrieval
	cs.headerDB[block.ParentID] = cs.head

	hotstuffProposal := model.SignedProposalFromBlock(proposal)
	cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
	cs.voteAggregator.On("AddBlock", hotstuffProposal).Once()
	cs.hotstuff.On("SubmitProposal", hotstuffProposal)

	// it should be processed without error
	err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
		OriginID: originID,
		Message:  proposal,
	})
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, proposal)
}

func (cs *CoreSuite) TestOnBlockProposalValidAncestor() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(ancestor.ToHeader())
	block := unittest.BlockWithParentFixture(parent.ToHeader())
	proposal := unittest.ProposalFromBlock(block)

	// store the data for retrieval
	cs.headerDB[parent.Hash()] = parent.ToHeader()
	cs.headerDB[ancestor.Hash()] = ancestor.ToHeader()

	hotstuffProposal := model.SignedProposalFromBlock(proposal)
	cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
	cs.voteAggregator.On("AddBlock", hotstuffProposal).Once()
	cs.hotstuff.On("SubmitProposal", hotstuffProposal)

	// it should be processed without error
	err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
		OriginID: originID,
		Message:  proposal,
	})
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, proposal)
}

func (cs *CoreSuite) TestOnBlockProposalSkipProposalThreshold() {

	// create a proposal which is far enough ahead to be dropped
	originID := cs.participants[1].NodeID
	view := cs.head.View + compliance.DefaultConfig().SkipNewProposalsThreshold + 1
	block := unittest.BlockFixture(
		unittest.Block.WithView(view),
		unittest.Block.WithParentView(view-1),
	)
	proposal := unittest.ProposalFromBlock(block)

	err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
		OriginID: originID,
		Message:  proposal,
	})
	require.NoError(cs.T(), err)

	// block should be dropped - not added to state or cache
	cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything, mock.Anything)
	cs.validator.AssertNotCalled(cs.T(), "ValidateProposal", mock.Anything)
	cs.pending.AssertNotCalled(cs.T(), "Add", originID, mock.Anything)
}

// TestOnBlockProposal_FailsHotStuffValidation tests that a proposal which fails HotStuff validation.
//   - should not go through protocol state validation
//   - should not be added to the state
//   - we should not attempt to process its children
//   - we should notify VoteAggregator, for known errors
func (cs *CoreSuite) TestOnBlockProposal_FailsHotStuffValidation() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(ancestor.ToHeader())
	block := unittest.BlockWithParentFixture(parent.ToHeader())
	proposal := unittest.ProposalFromBlock(block)
	hotstuffProposal := model.SignedProposalFromBlock(proposal)

	// store the data for retrieval
	cs.headerDB[parent.Hash()] = parent.ToHeader()
	cs.headerDB[ancestor.Hash()] = ancestor.ToHeader()

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
		err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should not extend the state with the header
		cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything, proposal)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})

	cs.Run("view for unknown epoch error", func() {
		// the block fails HotStuff validation
		*cs.validator = *hotstuff.NewValidator(cs.T())
		cs.validator.On("ValidateProposal", hotstuffProposal).Return(model.ErrViewForUnknownEpoch)

		// the expected error should be handled within the Core
		err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should not extend the state with the header
		cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything, proposal)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})

	cs.Run("unexpected error", func() {
		// the block fails HotStuff validation
		unexpectedErr := errors.New("generic unexpected error")
		*cs.validator = *hotstuff.NewValidator(cs.T())
		cs.validator.On("ValidateProposal", hotstuffProposal).Return(unexpectedErr)

		// the error should be propagated
		err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.ErrorIs(cs.T(), err, unexpectedErr)

		// we should not extend the state with the header
		cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything, proposal)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})
}

// TestOnBlockProposal_FailsProtocolStateValidation tests processing a proposal which passes HotStuff validation,
// but fails protocol state validation
//   - should not be added to the state
//   - we should not attempt to process its children
//   - we should notify VoteAggregator, for known errors
func (cs *CoreSuite) TestOnBlockProposal_FailsProtocolStateValidation() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(ancestor.ToHeader())
	block := unittest.BlockWithParentFixture(parent.ToHeader())
	proposal := unittest.ProposalFromBlock(block)
	hotstuffProposal := model.SignedProposalFromBlock(proposal)

	// store the data for retrieval
	cs.headerDB[parent.Hash()] = parent.ToHeader()
	cs.headerDB[ancestor.Hash()] = ancestor.ToHeader()

	// the block passes HotStuff validation
	cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil)

	cs.Run("invalid block", func() {
		// make sure we fail to extend the state
		*cs.state = protocol.ParticipantState{}
		cs.state.On("Final").Return(func() protint.Snapshot { return cs.snapshot })
		sentinelErr := state.NewInvalidExtensionErrorf("")
		cs.state.On("Extend", mock.Anything, mock.Anything).Return(sentinelErr)
		cs.proposalViolationNotifier.On("OnInvalidBlockDetected", mock.Anything).Run(func(args mock.Arguments) {
			err := args.Get(0).(flow.Slashable[model.InvalidProposalError])
			require.ErrorIs(cs.T(), err.Message, sentinelErr)
			require.Equal(cs.T(), err.Message.InvalidProposal, hotstuffProposal)
			require.Equal(cs.T(), err.OriginID, originID)
		}).Return().Once()
		// we should notify VoteAggregator about the invalid block
		cs.voteAggregator.On("InvalidBlock", hotstuffProposal).Return(nil)

		// the expected error should be handled within the Core
		err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should extend the state with the header
		cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, proposal)
		// we should not pass the block to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})

	cs.Run("outdated block", func() {
		// make sure we fail to extend the state
		*cs.state = protocol.ParticipantState{}
		cs.state.On("Final").Return(func() protint.Snapshot { return cs.snapshot })
		cs.state.On("Extend", mock.Anything, mock.Anything).Return(state.NewOutdatedExtensionErrorf(""))

		// the expected error should be handled within the Core
		err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should extend the state with the header
		cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, proposal)
		// we should not pass the block to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})

	cs.Run("unexpected error", func() {
		// make sure we fail to extend the state
		*cs.state = protocol.ParticipantState{}
		cs.state.On("Final").Return(func() protint.Snapshot { return cs.snapshot })
		unexpectedErr := errors.New("unexpected generic error")
		cs.state.On("Extend", mock.Anything, mock.Anything).Return(unexpectedErr)

		// it should be processed without error
		err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.ErrorIs(cs.T(), err, unexpectedErr)

		// we should extend the state with the header
		cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, proposal)
		// we should not pass the block to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})
}

func (cs *CoreSuite) TestProcessBlockAndDescendants() {

	// create three children blocks
	parent := unittest.BlockWithParentFixture(cs.head)
	block1 := unittest.BlockWithParentFixture(parent.ToHeader())
	block2 := unittest.BlockWithParentFixture(parent.ToHeader())
	block3 := unittest.BlockWithParentFixture(parent.ToHeader())

	proposal0 := unittest.ProposalFromBlock(parent)
	proposal1 := unittest.ProposalFromBlock(block1)
	proposal2 := unittest.ProposalFromBlock(block2)
	proposal3 := unittest.ProposalFromBlock(block3)

	// create the pending proposals
	pending1 := unittest.AsSlashable(proposal1)
	pending2 := unittest.AsSlashable(proposal2)
	pending3 := unittest.AsSlashable(proposal3)

	// store the parent on disk
	parentID := parent.Hash()
	cs.headerDB[parentID] = parent.ToHeader()

	// store the pending children in the cache
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending1)
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending2)
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending3)

	for _, prop := range []*flow.Proposal{proposal0, proposal1, proposal2, proposal3} {
		hotstuffProposal := model.SignedProposalFromBlock(prop)
		cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil)
		cs.voteAggregator.On("AddBlock", hotstuffProposal).Once()
		cs.hotstuff.On("SubmitProposal", hotstuffProposal).Once()
	}

	// execute the connected children handling
	err := cs.core.processBlockAndDescendants(flow.Slashable[*flow.Proposal]{
		OriginID: unittest.IdentifierFixture(),
		Message:  proposal0,
	})
	require.NoError(cs.T(), err, "should pass handling children")

	// make sure we drop the cache after trying to process
	cs.pending.AssertCalled(cs.T(), "DropForParent", parent.Hash())
}

func (cs *CoreSuite) TestProposalBufferingOrder() {

	// create a proposal that we will not submit until the end
	originID := cs.participants[1].NodeID
	missingBlock := unittest.BlockWithParentFixture(cs.head)
	missingProposal := unittest.ProposalFromBlock(missingBlock)

	// create a chain of descendants
	var proposals []*flow.Proposal
	parent := missingProposal
	for i := 0; i < 3; i++ {
		descendant := unittest.BlockWithParentFixture(parent.Block.ToHeader())
		proposal := unittest.ProposalFromBlock(descendant)
		proposals = append(proposals, proposal)
		parent = proposal
	}

	// replace the engine buffer with the real one
	cs.core.pending = real.NewPendingBlocks()

	// check that we request the ancestor block each time
	cs.sync.On("RequestBlock", missingBlock.Hash(), missingBlock.Height).Times(len(proposals))

	// process all the descendants
	for _, proposal := range proposals {
		// process and make sure no error occurs (as they are unverifiable)
		err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
			OriginID: originID,
			Message:  proposal,
		})
		require.NoError(cs.T(), err, "proposal buffering should pass")

		// make sure no block is forwarded to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", model.SignedProposalFromBlock(proposal))
	}

	// check that we submit each proposal in a valid order
	//  - we must process the missingProposal parent first
	//  - we can process the children next, in any order
	cs.validator.On("ValidateProposal", mock.Anything).Return(nil).Times(4)

	calls := 0                                   // track # of calls to SubmitProposal
	unprocessed := map[flow.Identifier]struct{}{ // track un-processed proposals
		missingProposal.Block.Hash(): {},
		proposals[0].Block.Hash():    {},
		proposals[1].Block.Hash():    {},
		proposals[2].Block.Hash():    {},
	}
	cs.hotstuff.On("SubmitProposal", mock.Anything).Times(4).Run(
		func(args mock.Arguments) {
			proposal := args.Get(0).(*model.SignedProposal)
			header := proposal.Block
			if calls == 0 {
				// first header processed must be the common parent
				assert.Equal(cs.T(), missingProposal.Block.Hash(), header.BlockID)
			}
			// mark the proposal as processed
			delete(unprocessed, header.BlockID)
			cs.headerDB[header.BlockID] = helper.SignedProposalToFlow(proposal).Header
			calls++
		},
	)
	cs.voteAggregator.On("AddBlock", mock.Anything).Times(4)

	// process the root proposal
	err := cs.core.OnBlockProposal(flow.Slashable[*flow.Proposal]{
		OriginID: originID,
		Message:  missingProposal,
	})
	require.NoError(cs.T(), err, "root proposal should pass")

	// all proposals should be processed
	assert.Len(cs.T(), unprocessed, 0)
}
