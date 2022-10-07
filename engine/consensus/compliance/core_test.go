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

	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	hotstuff "github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	consensus "github.com/onflow/flow-go/engine/consensus/mock"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	realModule "github.com/onflow/flow-go/module"
	real "github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/channels"
	"github.com/onflow/flow-go/network/mocknetwork"
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
	pendingDB  map[flow.Identifier]*flow.PendingBlock
	childrenDB map[flow.Identifier][]*flow.PendingBlock

	// mocked dependencies
	me                *module.Local
	metrics           *metrics.NoopCollector
	tracer            realModule.Tracer
	cleaner           *storage.Cleaner
	headers           *storage.Headers
	payloads          *storage.Payloads
	state             *protocol.MutableState
	snapshot          *protocol.Snapshot
	con               *mocknetwork.Conduit
	net               *mocknetwork.Network
	prov              *consensus.ProposalProvider
	pending           *module.PendingBlockBuffer
	hotstuff          *module.HotStuff
	sync              *module.BlockRequester
	validator         *hotstuff.Validator
	voteAggregator    *hotstuff.VoteAggregator
	timeoutAggregator *hotstuff.TimeoutAggregator

	// engine under test
	core *Core
}

func (cs *CommonSuite) SetupTest() {
	// seed the RNG
	rand.Seed(time.Now().UnixNano())

	// initialize the paramaters
	cs.participants = unittest.IdentityListFixture(3,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithWeight(1000),
	)
	cs.myID = cs.participants[0].NodeID
	block := unittest.BlockFixture()
	cs.head = block.Header

	// initialize the storage data
	cs.headerDB = make(map[flow.Identifier]*flow.Header)
	cs.payloadDB = make(map[flow.Identifier]*flow.Payload)
	cs.pendingDB = make(map[flow.Identifier]*flow.PendingBlock)
	cs.childrenDB = make(map[flow.Identifier][]*flow.PendingBlock)

	// store the head header and payload
	cs.headerDB[block.ID()] = block.Header
	cs.payloadDB[block.ID()] = block.Payload

	// set up local module mock
	cs.me = &module.Local{}
	cs.me.On("NodeID").Return(
		func() flow.Identifier {
			return cs.myID
		},
	)

	// set up storage cleaner
	cs.cleaner = &storage.Cleaner{}
	cs.cleaner.On("RunGC").Return()

	// set up header storage mock
	cs.headers = &storage.Headers{}
	cs.headers.On("Store", mock.Anything).Return(
		func(header *flow.Header) error {
			cs.headerDB[header.ID()] = header
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

	// set up payload storage mock
	cs.payloads = &storage.Payloads{}
	cs.payloads.On("Store", mock.Anything, mock.Anything).Return(
		func(header *flow.Header, payload *flow.Payload) error {
			cs.payloadDB[header.ID()] = payload
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
	cs.state = &protocol.MutableState{}
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
		func(filter flow.IdentityFilter) flow.IdentityList {
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
	cs.net = &mocknetwork.Network{}
	cs.net.On("Register", mock.Anything, mock.Anything).Return(
		func(channel channels.Channel, engine netint.MessageProcessor) netint.Conduit {
			return cs.con
		},
		nil,
	)

	// set up the provider engine
	cs.prov = &consensus.ProposalProvider{}
	cs.prov.On("ProvideProposal", mock.Anything).Return()

	// set up pending module mock
	cs.pending = &module.PendingBlockBuffer{}
	cs.pending.On("Add", mock.Anything, mock.Anything).Return(true)
	cs.pending.On("ByID", mock.Anything).Return(
		func(blockID flow.Identifier) *flow.PendingBlock {
			return cs.pendingDB[blockID]
		},
		func(blockID flow.Identifier) bool {
			_, ok := cs.pendingDB[blockID]
			return ok
		},
	)
	cs.pending.On("ByParentID", mock.Anything).Return(
		func(blockID flow.Identifier) []*flow.PendingBlock {
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

	// initialize the engine
	e, err := NewCore(
		unittest.Logger(),
		cs.metrics,
		cs.tracer,
		cs.metrics,
		cs.metrics,
		cs.cleaner,
		cs.headers,
		cs.payloads,
		cs.state,
		cs.pending,
		cs.sync,
		cs.validator,
		cs.voteAggregator,
		cs.timeoutAggregator,
	)
	require.NoError(cs.T(), err, "engine initialization should pass")

	cs.core = e
	// assign engine with consensus & synchronization
	cs.core.hotstuff = cs.hotstuff
}

func (cs *CoreSuite) TestOnBlockProposalValidParent() {

	// create a proposal that directly descends from the latest finalized header
	originID := cs.participants[1].NodeID
	block := unittest.BlockWithParentFixture(cs.head)
	proposal := unittest.ProposalFromBlock(block)

	// store the data for retrieval
	cs.headerDB[block.Header.ParentID] = cs.head

	cs.validator.On("ValidateProposal", model.ProposalFromFlow(block.Header, cs.head.View)).Return(nil)
	cs.hotstuff.On("SubmitProposal", block.Header, cs.head.View)

	// it should be processed without error
	err := cs.core.OnBlockProposal(originID, proposal)
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, block)
}

func (cs *CoreSuite) TestOnBlockProposalValidAncestor() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(ancestor.Header)
	block := unittest.BlockWithParentFixture(parent.Header)
	proposal := unittest.ProposalFromBlock(block)

	// store the data for retrieval
	cs.headerDB[parent.ID()] = parent.Header
	cs.headerDB[ancestor.ID()] = ancestor.Header

	cs.validator.On("ValidateProposal", model.ProposalFromFlow(block.Header, parent.Header.View)).Return(nil)
	cs.hotstuff.On("SubmitProposal", block.Header, parent.Header.View)

	// it should be processed without error
	err := cs.core.OnBlockProposal(originID, proposal)
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, block)
}

func (cs *CoreSuite) TestOnBlockProposalSkipProposalThreshold() {

	// create a proposal which is far enough ahead to be dropped
	originID := cs.participants[1].NodeID
	block := unittest.BlockFixture()
	block.Header.Height = cs.head.Height + compliance.DefaultConfig().SkipNewProposalsThreshold + 1
	proposal := unittest.ProposalFromBlock(&block)

	err := cs.core.OnBlockProposal(originID, proposal)
	require.NoError(cs.T(), err)

	// block should be dropped - not added to state or cache
	cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything)
	cs.validator.AssertNotCalled(cs.T(), "ValidateProposal", mock.Anything)
	cs.pending.AssertNotCalled(cs.T(), "Add", originID, mock.Anything)
}

// TestOnBlockProposal_InvalidProposal tests that a proposal which fails HotStuff validation.
//   - should not go through compliance checks
//   - should not be added to the state
//   - we should not attempt to process its children
//   - we should notify VoteAggregator, for known errors
func (cs *CoreSuite) TestOnBlockProposal_InvalidProposal() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(ancestor.Header)
	block := unittest.BlockWithParentFixture(parent.Header)
	proposal := unittest.ProposalFromBlock(block)
	hotstuffProposal := model.ProposalFromFlow(block.Header, parent.Header.View)

	// store the data for retrieval
	cs.headerDB[parent.ID()] = parent.Header
	cs.headerDB[ancestor.ID()] = ancestor.Header

	cs.Run("invalid block error", func() {
		// the block fails HotStuff validation
		*cs.validator = *hotstuff.NewValidator(cs.T())
		cs.validator.On("ValidateProposal", model.ProposalFromFlow(block.Header, parent.Header.View)).Return(model.InvalidBlockError{})
		// we should notify VoteAggregator about the invalid block
		cs.voteAggregator.On("InvalidBlock", hotstuffProposal).Return(nil)

		// the expected error should be handled within the Core
		err := cs.core.OnBlockProposal(originID, proposal)
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should not extend the state with the header
		cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything, block)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
		cs.voteAggregator.AssertExpectations(cs.T())
	})

	cs.Run("view for unknown epoch error", func() {
		// the block fails HotStuff validation
		*cs.validator = *hotstuff.NewValidator(cs.T())
		cs.validator.On("ValidateProposal", model.ProposalFromFlow(block.Header, parent.Header.View)).Return(model.ErrViewForUnknownEpoch)

		// the expected error should be handled within the Core
		err := cs.core.OnBlockProposal(originID, proposal)
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should not extend the state with the header
		cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything, block)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})

	cs.Run("unexpected error", func() {
		// the block fails HotStuff validation
		unexpectedErr := errors.New("generic unexpected error")
		*cs.validator = *hotstuff.NewValidator(cs.T())
		cs.validator.On("ValidateProposal", model.ProposalFromFlow(block.Header, parent.Header.View)).Return(unexpectedErr)

		// the error should be propagated
		err := cs.core.OnBlockProposal(originID, proposal)
		require.ErrorIs(cs.T(), err, unexpectedErr)

		// we should not extend the state with the header
		cs.state.AssertNotCalled(cs.T(), "Extend", mock.Anything, block)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})
}

// TestOnBlockProposal_InvalidExtension tests processing a proposal which passes HotStuff validation,
// but fails compliance checks.
//   - should not be added to the state
//   - we should not attempt to process its children
//   - we should notify VoteAggregator, for known errors
func (cs *CoreSuite) TestOnBlockProposal_InvalidExtension() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(ancestor.Header)
	block := unittest.BlockWithParentFixture(parent.Header)
	proposal := unittest.ProposalFromBlock(block)
	hotstuffProposal := model.ProposalFromFlow(block.Header, parent.Header.View)

	// store the data for retrieval
	cs.headerDB[parent.ID()] = parent.Header
	cs.headerDB[ancestor.ID()] = ancestor.Header

	// the block passes HotStuff validation
	cs.validator.On("ValidateProposal", hotstuffProposal).Return(nil)

	cs.Run("invalid block", func() {
		// make sure we fail to extend the state
		*cs.state = protocol.MutableState{}
		cs.state.On("Final").Return(func() protint.Snapshot { return cs.snapshot })
		cs.state.On("Extend", mock.Anything, mock.Anything).Return(state.NewInvalidExtensionError(""))
		// we should notify VoteAggregator about the invalid block
		cs.voteAggregator.On("InvalidBlock", hotstuffProposal).Return(nil)

		// the expected error should be handled within the Core
		err := cs.core.OnBlockProposal(originID, proposal)
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should extend the state with the header
		cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, block)
		// we should not pass the block to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything, mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
		cs.voteAggregator.AssertExpectations(cs.T())
	})

	cs.Run("outdated block", func() {
		// make sure we fail to extend the state
		*cs.state = protocol.MutableState{}
		cs.state.On("Final").Return(func() protint.Snapshot { return cs.snapshot })
		cs.state.On("Extend", mock.Anything, mock.Anything).Return(state.NewOutdatedExtensionError(""))

		// the expected error should be handled within the Core
		err := cs.core.OnBlockProposal(originID, proposal)
		require.NoError(cs.T(), err, "proposal with invalid extension should fail")

		// we should extend the state with the header
		cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, block)
		// we should not pass the block to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything, mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})

	cs.Run("unexpected error", func() {
		// make sure we fail to extend the state
		*cs.state = protocol.MutableState{}
		cs.state.On("Final").Return(func() protint.Snapshot { return cs.snapshot })
		unexpectedErr := errors.New("unexpected generic error")
		cs.state.On("Extend", mock.Anything, mock.Anything).Return(unexpectedErr)

		// it should be processed without error
		err := cs.core.OnBlockProposal(originID, proposal)
		require.ErrorIs(cs.T(), err, unexpectedErr)

		// we should extend the state with the header
		cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, block)
		// we should not pass the block to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything, mock.Anything)
		// we should not attempt to process the children
		cs.pending.AssertNotCalled(cs.T(), "ByParentID", mock.Anything)
	})
}

func (cs *CoreSuite) TestProcessBlockAndDescendants() {

	// create three children blocks
	parent := unittest.BlockWithParentFixture(cs.head)
	proposal := unittest.ProposalFromBlock(parent)
	block1 := unittest.BlockWithParentFixture(parent.Header)
	block2 := unittest.BlockWithParentFixture(parent.Header)
	block3 := unittest.BlockWithParentFixture(parent.Header)

	// create the pending blocks
	pending1 := unittest.PendingFromBlock(block1)
	pending2 := unittest.PendingFromBlock(block2)
	pending3 := unittest.PendingFromBlock(block3)

	// store the parent on disk
	parentID := parent.ID()
	cs.headerDB[parentID] = parent.Header

	// store the pending children in the cache
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending1)
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending2)
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending3)

	cs.validator.On("ValidateProposal", model.ProposalFromFlow(parent.Header, cs.head.View)).Return(nil)
	cs.validator.On("ValidateProposal", model.ProposalFromFlow(block1.Header, parent.Header.View)).Return(nil)
	cs.validator.On("ValidateProposal", model.ProposalFromFlow(block2.Header, parent.Header.View)).Return(nil)
	cs.validator.On("ValidateProposal", model.ProposalFromFlow(block3.Header, parent.Header.View)).Return(nil)

	cs.hotstuff.On("SubmitProposal", parent.Header, cs.head.View).Once()
	cs.hotstuff.On("SubmitProposal", block1.Header, parent.Header.View).Once()
	cs.hotstuff.On("SubmitProposal", block2.Header, parent.Header.View).Once()
	cs.hotstuff.On("SubmitProposal", block3.Header, parent.Header.View).Once()

	// execute the connected children handling
	err := cs.core.processBlockAndDescendants(proposal, cs.head)
	require.NoError(cs.T(), err, "should pass handling children")

	// make sure we drop the cache after trying to process
	cs.pending.AssertCalled(cs.T(), "DropForParent", parent.Header.ID())
}

func (cs *CoreSuite) TestOnSubmitVote() {
	// create a vote
	originID := unittest.IdentifierFixture()
	vote := messages.BlockVote{
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

// TestOnSubmitTimeout tests that submitting messages.TimeoutObject adds model.TimeoutObject into
// TimeoutAggregator.
func (cs *CoreSuite) TestOnSubmitTimeout() {
	// create a vote
	originID := unittest.IdentifierFixture()
	timeout := messages.TimeoutObject{
		View:       rand.Uint64(),
		NewestQC:   helper.MakeQC(),
		LastViewTC: helper.MakeTC(),
		SigData:    unittest.SignatureFixture(),
	}

	cs.timeoutAggregator.On("AddTimeout", &model.TimeoutObject{
		View:       timeout.View,
		NewestQC:   timeout.NewestQC,
		LastViewTC: timeout.LastViewTC,
		SignerID:   originID,
		SigData:    timeout.SigData,
	}).Return()

	// execute the timeout submission
	err := cs.core.OnTimeoutObject(originID, &timeout)
	require.NoError(cs.T(), err, "timeout object should pass")
}

func (cs *CoreSuite) TestProposalBufferingOrder() {

	// create a proposal that we will not submit until the end
	originID := cs.participants[1].NodeID
	block := unittest.BlockWithParentFixture(cs.head)
	missing := unittest.ProposalFromBlock(block)

	// create a chain of descendants
	var proposals []*messages.BlockProposal
	parent := missing
	for i := 0; i < 3; i++ {
		descendant := unittest.BlockWithParentFixture(parent.Header)
		proposal := unittest.ProposalFromBlock(descendant)
		proposals = append(proposals, proposal)
		parent = proposal
	}

	// replace the engine buffer with the real one
	cs.core.pending = real.NewPendingBlocks()

	// check that we request the ancestor block each time
	cs.sync.On("RequestBlock", parent.Header.ID(), parent.Header.Height).Times(len(proposals))

	// process all the descendants
	for _, proposal := range proposals {
		// process and make sure no error occurs (as they are unverifiable)
		err := cs.core.OnBlockProposal(originID, proposal)
		require.NoError(cs.T(), err, "proposal buffering should pass")

		// make sure no block is forwarded to hotstuff
		cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", proposal.Header, missing.Header.View)
	}

	// check that we submit each proposal in a valid order
	//  - we must process the missing parent first
	//  - we can process the children next, in any order
	cs.validator.On("ValidateProposal", mock.Anything).Return(nil).Times(4)

	calls := 0                                   // track # of calls to SubmitProposal
	unprocessed := map[flow.Identifier]struct{}{ // track un-processed proposals
		missing.Header.ID():      {},
		proposals[0].Header.ID(): {},
		proposals[1].Header.ID(): {},
		proposals[2].Header.ID(): {},
	}
	cs.hotstuff.On("SubmitProposal", mock.Anything, mock.Anything).Times(4).Run(
		func(args mock.Arguments) {
			header := args.Get(0).(*flow.Header)
			if calls == 0 {
				// first header processed must be the common parent
				assert.Equal(cs.T(), missing.Header.ID(), header.ID())
			}
			// mark the proposal as processed
			delete(unprocessed, header.ID())
			cs.headerDB[header.ID()] = header
			calls++
		},
	)

	// process the root proposal
	err := cs.core.OnBlockProposal(originID, missing)
	require.NoError(cs.T(), err, "root proposal should pass")

	// all proposals should be processed
	assert.Len(cs.T(), unprocessed, 0)
}
