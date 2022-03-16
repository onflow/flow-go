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
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
	realModule "github.com/onflow/flow-go/module"
	real "github.com/onflow/flow-go/module/buffer"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/trace"
	netint "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/mocknetwork"
	protint "github.com/onflow/flow-go/state/protocol"
	protocol "github.com/onflow/flow-go/state/protocol/mock"
	storerr "github.com/onflow/flow-go/storage"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestComplianceCore(t *testing.T) {
	suite.Run(t, new(ComplianceCoreSuite))
}

type ComplianceCoreSuite struct {
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
	me             *module.Local
	metrics        *metrics.NoopCollector
	tracer         realModule.Tracer
	cleaner        *storage.Cleaner
	headers        *storage.Headers
	payloads       *storage.Payloads
	state          *protocol.MutableState
	snapshot       *protocol.Snapshot
	con            *mocknetwork.Conduit
	net            *mocknetwork.Network
	prov           *mocknetwork.Engine
	pending        *module.PendingBlockBuffer
	hotstuff       *module.HotStuff
	sync           *module.BlockRequester
	voteAggregator *hotstuff.VoteAggregator

	// engine under test
	core *Core
}

func (cs *ComplianceCoreSuite) SetupTest() {
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
		func(channel netint.Channel, engine netint.MessageProcessor) netint.Conduit {
			return cs.con
		},
		nil,
	)

	// set up the provider engine
	cs.prov = &mocknetwork.Engine{}
	cs.prov.On("SubmitLocal", mock.Anything).Return()

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
		cs.voteAggregator,
	)
	require.NoError(cs.T(), err, "engine initialization should pass")

	cs.core = e
	// assign engine with consensus & synchronization
	cs.core.hotstuff = cs.hotstuff
}

func (cs *ComplianceCoreSuite) TestOnBlockProposalValidParent() {

	// create a proposal that directly descends from the latest finalized header
	originID := cs.participants[1].NodeID
	block := unittest.BlockWithParentFixture(cs.head)
	proposal := unittest.ProposalFromBlock(block)

	// store the data for retrieval
	cs.headerDB[block.Header.ParentID] = cs.head

	cs.hotstuff.On("SubmitProposal", block.Header, cs.head.View).Return()

	// it should be processed without error
	err := cs.core.OnBlockProposal(originID, proposal)
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, block)

	// we should submit the proposal to hotstuff
	cs.hotstuff.AssertExpectations(cs.T())
}

func (cs *ComplianceCoreSuite) TestOnBlockProposalValidAncestor() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(ancestor.Header)
	block := unittest.BlockWithParentFixture(parent.Header)
	proposal := unittest.ProposalFromBlock(block)

	// store the data for retrieval
	cs.headerDB[parent.ID()] = parent.Header
	cs.headerDB[ancestor.ID()] = ancestor.Header

	cs.hotstuff.On("SubmitProposal", block.Header, parent.Header.View).Return()

	// it should be processed without error
	err := cs.core.OnBlockProposal(originID, proposal)
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, block)

	// we should submit the proposal to hotstuff
	cs.hotstuff.AssertExpectations(cs.T())
}

func (cs *ComplianceCoreSuite) TestOnBlockProposalInvalidExtension() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(ancestor.Header)
	block := unittest.BlockWithParentFixture(parent.Header)
	proposal := unittest.ProposalFromBlock(block)

	// store the data for retrieval
	cs.headerDB[parent.ID()] = parent.Header
	cs.headerDB[ancestor.ID()] = ancestor.Header

	// make sure we fail to extend the state
	*cs.state = protocol.MutableState{}
	cs.state.On("Final").Return(
		func() protint.Snapshot {
			return cs.snapshot
		},
	)
	cs.state.On("Extend", mock.Anything, mock.Anything).Return(errors.New("dummy error"))

	// it should be processed without error
	err := cs.core.OnBlockProposal(originID, proposal)
	require.Error(cs.T(), err, "proposal with invalid extension should fail")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Extend", mock.Anything, block)

	// we should not submit the proposal to hotstuff
	cs.hotstuff.AssertExpectations(cs.T())
}

func (cs *ComplianceCoreSuite) TestProcessBlockAndDescendants() {

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

	cs.hotstuff.On("SubmitProposal", parent.Header, cs.head.View).Return().Once()
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

func (cs *ComplianceCoreSuite) TestProposalBufferingOrder() {

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

	// process all of the descendants
	for _, proposal := range proposals {

		// check that we request the ancestor block each time
		cs.sync.On("RequestBlock", mock.Anything).Once().Run(
			func(args mock.Arguments) {
				ancestorID := args.Get(0).(flow.Identifier)
				assert.Equal(cs.T(), missing.Header.ID(), ancestorID, "should always request root block")
			},
		)

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
			cs.headerDB[header.ID()] = header
		},
	)

	// process the root proposal
	err := cs.core.OnBlockProposal(originID, missing)
	require.NoError(cs.T(), err, "root proposal should pass")

	// make sure we submitted all four proposals
	cs.hotstuff.AssertExpectations(cs.T())
}
