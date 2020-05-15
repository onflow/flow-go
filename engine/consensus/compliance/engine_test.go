package compliance

import (
	"errors"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	real "github.com/dapperlabs/flow-go/module/buffer"
	"github.com/dapperlabs/flow-go/module/metrics"
	module "github.com/dapperlabs/flow-go/module/mock"
	netint "github.com/dapperlabs/flow-go/network"
	network "github.com/dapperlabs/flow-go/network/mock"
	protint "github.com/dapperlabs/flow-go/state/protocol"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	storerr "github.com/dapperlabs/flow-go/storage"
	storage "github.com/dapperlabs/flow-go/storage/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestComplianceEngine(t *testing.T) {
	suite.Run(t, new(ComplianceSuite))
}

type ComplianceSuite struct {
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
	me       *module.Local
	metrics  *metrics.NoopCollector
	cleaner  *storage.Cleaner
	headers  *storage.Headers
	payloads *storage.Payloads
	state    *protocol.State
	snapshot *protocol.Snapshot
	mutator  *protocol.Mutator
	con      *network.Conduit
	net      *module.Network
	prov     *network.Engine
	pending  *module.PendingBlockBuffer
	hotstuff *module.HotStuff
	sync     *module.Synchronization

	// engine under test
	e *Engine
}

func (cs *ComplianceSuite) SetupTest() {
	// seed the RNG
	rand.Seed(time.Now().UnixNano())

	// initialize the paramaters
	cs.participants = unittest.IdentityListFixture(3,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithStake(1000),
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
	cs.state = &protocol.State{}
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
	cs.state.On("Mutate", mock.Anything).Return(
		func() protint.Mutator {
			return cs.mutator
		},
	)

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

	// set up protocol mutator mock
	cs.mutator = &protocol.Mutator{}
	cs.mutator.On("Extend", mock.Anything).Return(nil)

	// set up network conduit mock
	cs.con = &network.Conduit{}
	cs.con.On("Submit", mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Submit", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	cs.con.On("Submit", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// set up network module mock
	cs.net = &module.Network{}
	cs.net.On("Register", mock.Anything, mock.Anything).Return(
		func(code uint8, engine netint.Engine) netint.Conduit {
			return cs.con
		},
		nil,
	)

	// set up the provider engine
	cs.prov = &network.Engine{}
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
	cs.pending.On("PruneByHeight", mock.Anything).Return()

	// set up hotstuff module mock
	cs.hotstuff = &module.HotStuff{}
	cs.hotstuff.On("SubmitProposal", mock.Anything, mock.Anything).Return()
	cs.hotstuff.On("SubmitVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	// set up synchronization module mock
	cs.sync = &module.Synchronization{}
	cs.sync.On("RequestBlock", mock.Anything).Return(nil)

	// set up no-op metrics mock
	cs.metrics = metrics.NewNoopCollector()

	// initialize the engine
	log := zerolog.New(os.Stderr)
	e, err := New(log, cs.metrics, cs.metrics, cs.metrics, cs.net, cs.me, cs.cleaner, cs.headers, cs.payloads, cs.state, cs.prov, cs.pending)
	require.NoError(cs.T(), err, "engine initialization should pass")

	// assign engine with consensus & synchronization
	cs.e = e.WithConsensus(cs.hotstuff).WithSynchronization(cs.sync)
}

func (cs *ComplianceSuite) TestSendVote() {
	// create parameters to send a vote
	blockID := unittest.IdentifierFixture()
	view := rand.Uint64()
	sig := unittest.SignatureFixture()
	recipientID := unittest.IdentifierFixture()

	// submit the vote
	err := cs.e.SendVote(blockID, view, sig, recipientID)
	require.NoError(cs.T(), err, "should pass send vote")

	// check it was called with right params
	vote := messages.BlockVote{
		BlockID: blockID,
		View:    view,
		SigData: sig,
	}
	cs.con.AssertCalled(cs.T(), "Submit", &vote, recipientID)
}

func (cs *ComplianceSuite) TestBroadcastProposal() {

	// add execution node to participants to make sure we exclude them from broadcast
	cs.participants = append(cs.participants, unittest.IdentityFixture(unittest.WithRole(flow.RoleExecution)))

	// generate a parent with height and chain ID set
	parent := unittest.BlockHeaderFixture()
	parent.ChainID = "test"
	parent.Height = 10
	cs.headerDB[parent.ID()] = &parent

	// create a block with the parent and store the payload with correct ID
	block := unittest.BlockWithParentFixture(&parent)
	block.Header.ProposerID = cs.myID
	cs.payloadDB[block.ID()] = block.Payload

	// keep a duplicate of the correct header to check against leader
	header := block.Header

	// unset chain and height to make sure they are correctly reconstructed
	block.Header.ChainID = ""
	block.Header.Height = 0

	// submit to broadcast proposal
	err := cs.e.BroadcastProposal(block.Header)
	require.NoError(cs.T(), err, "header broadcast should pass")

	// make sure chain ID and height were reconstructed and
	// we broadcast to correct nodes
	header.ChainID = "test"
	header.Height = 11
	msg := &messages.BlockProposal{
		Header:  header,
		Payload: block.Payload,
	}
	cs.con.AssertCalled(cs.T(), "Submit", msg, cs.participants[1].NodeID, cs.participants[2].NodeID)

	// should fail with wrong proposer
	header.ProposerID = unittest.IdentifierFixture()
	err = cs.e.BroadcastProposal(header)
	require.Error(cs.T(), err, "should fail with wrong proposer")
	header.ProposerID = cs.myID

	// should fail with changed (missing) parent
	header.ParentID[0]++
	err = cs.e.BroadcastProposal(header)
	require.Error(cs.T(), err, "should fail with missing parent")
	header.ParentID[0]--

	// should fail with wrong block ID (payload unavailable)
	header.View++
	err = cs.e.BroadcastProposal(header)
	require.Error(cs.T(), err, "should fail with missing payload")
	header.View--
}

func (cs *ComplianceSuite) TestOnBlockProposalValidParent() {

	// create a proposal that directly descends from the latest finalized header
	originID := cs.participants[1].NodeID
	block := unittest.BlockWithParentFixture(cs.head)
	proposal := unittest.ProposalFromBlock(&block)

	// store the data for retrieval
	cs.headerDB[block.Header.ParentID] = cs.head

	// it should be processed without error
	err := cs.e.onBlockProposal(originID, proposal)
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Mutate")
	cs.mutator.AssertCalled(cs.T(), "Extend", &block)

	// we should submit the proposal to hotstuff
	cs.hotstuff.AssertCalled(cs.T(), "SubmitProposal", block.Header, cs.head.View)
}

func (cs *ComplianceSuite) TestOnBlockProposalValidAncestor() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(ancestor.Header)
	block := unittest.BlockWithParentFixture(parent.Header)
	proposal := unittest.ProposalFromBlock(&block)

	// store the data for retrieval
	cs.headerDB[parent.ID()] = parent.Header
	cs.headerDB[ancestor.ID()] = ancestor.Header

	// it should be processed without error
	err := cs.e.onBlockProposal(originID, proposal)
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Mutate")
	cs.mutator.AssertCalled(cs.T(), "Extend", &block)

	// we should submit the proposal to hotstuff
	cs.hotstuff.AssertCalled(cs.T(), "SubmitProposal", block.Header, parent.Header.View)
}

func (cs *ComplianceSuite) TestOnBlockProposalInvalidExtension() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(ancestor.Header)
	block := unittest.BlockWithParentFixture(parent.Header)
	proposal := unittest.ProposalFromBlock(&block)

	// store the data for retrieval
	cs.headerDB[parent.ID()] = parent.Header
	cs.headerDB[ancestor.ID()] = ancestor.Header

	// make sure we fail to extend the state
	*cs.mutator = protocol.Mutator{}
	cs.mutator.On("Extend", mock.Anything).Return(errors.New("dummy error"))

	// it should be processed without error
	err := cs.e.onBlockProposal(originID, proposal)
	require.Error(cs.T(), err, "proposal with invalid extension should fail")

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Mutate")
	cs.mutator.AssertCalled(cs.T(), "Extend", &block)

	// we should not submit the proposal to hotstuff
	cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything, mock.Anything)
}

func (cs *ComplianceSuite) TestOnSubmitVote() {

	// create a vote
	originID := unittest.IdentifierFixture()
	vote := messages.BlockVote{
		BlockID: unittest.IdentifierFixture(),
		View:    rand.Uint64(),
		SigData: unittest.SignatureFixture(),
	}

	// execute the vote submission
	err := cs.e.onBlockVote(originID, &vote)
	require.NoError(cs.T(), err, "block vote should pass")

	// check the submit vote was called with correct parameters
	cs.hotstuff.AssertCalled(cs.T(), "SubmitVote", originID, vote.BlockID, vote.View, vote.SigData)
}

func (cs *ComplianceSuite) TestProcessPendingChildrenNone() {

	// generate random block ID
	header := unittest.BlockHeaderFixture()

	// execute handle connected children
	err := cs.e.processPendingChildren(&header)
	require.NoError(cs.T(), err, "should pass when no children")
	cs.snapshot.AssertNotCalled(cs.T(), "Final")
	cs.pending.AssertNotCalled(cs.T(), "DropForParent", mock.Anything)
}

func (cs *ComplianceSuite) TestProcessPendingChildren() {

	// create three children blocks
	parent := unittest.BlockFixture()
	parent.Header.Height = cs.head.Height + 1
	block1 := unittest.BlockWithParentFixture(parent.Header)
	block2 := unittest.BlockWithParentFixture(parent.Header)
	block3 := unittest.BlockWithParentFixture(parent.Header)

	// create the pending blocks
	pending1 := unittest.PendingFromBlock(&block1)
	pending2 := unittest.PendingFromBlock(&block2)
	pending3 := unittest.PendingFromBlock(&block3)

	// store the parent on disk
	parentID := parent.ID()
	cs.headerDB[parentID] = parent.Header

	// store the pending children in the cache
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending1)
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending2)
	cs.childrenDB[parentID] = append(cs.childrenDB[parentID], pending3)

	// execute the connected children handling
	err := cs.e.processPendingChildren(parent.Header)
	require.NoError(cs.T(), err, "should pass handling children")

	// check that we submitted each child to hotstuff
	cs.hotstuff.AssertCalled(cs.T(), "SubmitProposal", block1.Header, parent.Header.View)
	cs.hotstuff.AssertCalled(cs.T(), "SubmitProposal", block2.Header, parent.Header.View)
	cs.hotstuff.AssertCalled(cs.T(), "SubmitProposal", block3.Header, parent.Header.View)

	// make sure we drop the cache after trying to process
	cs.pending.AssertCalled(cs.T(), "DropForParent", parent.Header)
}

func (cs *ComplianceSuite) TestProposalBufferingOrder() {

	// create a proposal that we will not submit until the end
	originID := cs.participants[1].NodeID
	block := unittest.BlockWithParentFixture(cs.head)
	missing := unittest.ProposalFromBlock(&block)

	// create a chain of descendants
	var proposals []*messages.BlockProposal
	parent := missing
	for i := 0; i < 3; i++ {
		descendant := unittest.BlockWithParentFixture(parent.Header)
		proposal := unittest.ProposalFromBlock(&descendant)
		proposals = append(proposals, proposal)
		parent = proposal
	}

	// replace the engine buffer with the real one
	cs.e.pending = real.NewPendingBlocks()

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
		err := cs.e.onBlockProposal(originID, proposal)
		require.NoError(cs.T(), err, "proposal buffering should pass")

		// make sure no block is forwarded to hotstuff
		cs.hotstuff.AssertNumberOfCalls(cs.T(), "SubmitProposal", 0)
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
	err := cs.e.onBlockProposal(originID, missing)
	require.NoError(cs.T(), err, "root proposal should pass")

	// make sure we submitted all four proposals
	cs.hotstuff.AssertNumberOfCalls(cs.T(), "SubmitProposal", 4)
}
