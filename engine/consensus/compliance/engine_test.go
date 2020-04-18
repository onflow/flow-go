package compliance

import (
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/messages"
	module "github.com/dapperlabs/flow-go/module/mock"
	netint "github.com/dapperlabs/flow-go/network"
	network "github.com/dapperlabs/flow-go/network/mock"
	protint "github.com/dapperlabs/flow-go/state/protocol"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
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
	childrenDB map[flow.Identifier][]flow.Identifier

	// mocked dependencies
	me       *module.Local
	state    *protocol.State
	snapshot *protocol.Snapshot
	mutator  *protocol.Mutator
	headers  *storage.Headers
	payloads *storage.Payloads
	con      *network.Conduit
	net      *module.Network
	prov     *network.Engine
	cache    *module.PendingBlockBuffer
	hotstuff *module.HotStuff
	sync     *module.Synchronization

	// engine under test
	e *Engine
}

func (cs *ComplianceSuite) SetupTest() {

	// initialize the paramaters
	cs.participants = unittest.IdentityListFixture(3,
		unittest.WithRole(flow.RoleConsensus),
		unittest.WithStake(1000),
	)
	cs.myID = cs.participants[0].NodeID
	block := unittest.BlockFixture()
	cs.head = &block.Header

	// initialize the storage data
	cs.headerDB = make(map[flow.Identifier]*flow.Header)
	cs.payloadDB = make(map[flow.Identifier]*flow.Payload)
	cs.pendingDB = make(map[flow.Identifier]*flow.PendingBlock)
	cs.childrenDB = make(map[flow.Identifier][]flow.Identifier)

	// store the head header and payload
	cs.headerDB[block.ID()] = &block.Header
	cs.payloadDB[block.ID()] = &block.Payload

	// set up local module mock
	cs.me = &module.Local{}
	cs.me.On("NodeID").Return(
		func() flow.Identifier {
			return cs.myID
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
				return fmt.Errorf("unknown header (%x)", blockID)
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
				return fmt.Errorf("unknown payload (%x)", blockID)
			}
			return nil
		},
	)

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

	// set up cache module mock
	cs.cache = &module.PendingBlockBuffer{}
	cs.cache.On("Add", mock.Anything).Return(false)
	cs.cache.On("ByParentID", mock.Anything).Return(nil, false)
	cs.cache.On("DropForParent", mock.Anything).Return()

	// set up hotstuff module mock
	cs.hotstuff = &module.HotStuff{}
	cs.hotstuff.On("SubmitProposal", mock.Anything, mock.Anything).Return()
	cs.hotstuff.On("SubmitVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()

	// set up synchronization module mock
	cs.sync = &module.Synchronization{}
	cs.sync.On("RequestBlock", mock.Anything).Return(nil)

	// initialize the engine
	log := zerolog.New(ioutil.Discard)
	e, err := New(log, cs.net, cs.me, cs.state, cs.headers, cs.payloads, cs.cache, cs.prov)
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
	cs.payloadDB[block.ID()] = &block.Payload

	// keep a duplicate of the correct header to check against leader
	header := block.Header

	// unset chain and height to make sure they are correctly reconstructed
	block.Header.ChainID = ""
	block.Header.Height = 0

	// submit to broadcast proposal
	err := cs.e.BroadcastProposal(&block.Header)
	require.NoError(cs.T(), err, "header broadcast should pass")

	// make sure chain ID and height were reconstructed and
	// we broadcast to correct nodes
	header.ChainID = "test"
	header.Height = 11
	msg := &messages.BlockProposal{
		Header:  &header,
		Payload: &block.Payload,
	}
	cs.con.AssertCalled(cs.T(), "Submit", msg, cs.participants[1].NodeID, cs.participants[2].NodeID)

	// should fail with wrong proposer
	header.ProposerID = unittest.IdentifierFixture()
	err = cs.e.BroadcastProposal(&header)
	require.Error(cs.T(), err, "should fail with wrong proposer")
	header.ProposerID = cs.myID

	// should fail with changed (missing) parent
	header.ParentID[0]++
	err = cs.e.BroadcastProposal(&header)
	require.Error(cs.T(), err, "should fail with missing parent")
	header.ParentID[0]--

	// should fail with wrong block ID (payload unavailable)
	header.View++
	err = cs.e.BroadcastProposal(&header)
	require.Error(cs.T(), err, "should fail with missing payload")
	header.View--
}

func (cs *ComplianceSuite) TestOnBlockProposalValidParent() {

	// create a proposal that directly descends from the latest finalized header
	originID := cs.participants[1].NodeID
	block := unittest.BlockWithParentFixture(cs.head)
	proposal := messages.BlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}

	// store the data for retrieval
	cs.headerDB[block.ParentID] = cs.head

	// it should be processed without error
	err := cs.e.onBlockProposal(originID, &proposal)
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should store the payload
	cs.payloads.AssertCalled(cs.T(), "Store", &block.Header, &block.Payload)

	// we should store the header
	cs.headers.AssertCalled(cs.T(), "Store", &block.Header)

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Mutate")
	cs.mutator.AssertCalled(cs.T(), "Extend", block.ID())

	// we should submit the proposal to hotstuff
	cs.hotstuff.AssertCalled(cs.T(), "SubmitProposal", &block.Header, cs.head.View)
}

func (cs *ComplianceSuite) TestOnBlockProposalValidAncestor() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(&ancestor.Header)
	block := unittest.BlockWithParentFixture(&parent.Header)
	proposal := messages.BlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}

	// store the data for retrieval
	cs.headerDB[parent.ID()] = &parent.Header
	cs.headerDB[parent.ID()] = &parent.Header
	cs.e.cache[parent.ID()] = &parent.Header
	cs.e.cache[ancestor.ID()] = &ancestor.Header

	// it should be processed without error
	err := cs.e.onBlockProposal(originID, &proposal)
	require.NoError(cs.T(), err, "valid block proposal should pass")

	// we should store the payload
	cs.payloads.AssertCalled(cs.T(), "Store", &block.Header, &block.Payload)

	// we should store the header
	cs.headers.AssertCalled(cs.T(), "Store", &block.Header)

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Mutate")
	cs.mutator.AssertCalled(cs.T(), "Extend", block.ID())

	// we should submit the proposal to hotstuff
	cs.hotstuff.AssertCalled(cs.T(), "SubmitProposal", &block.Header, parent.View)
}

func (cs *ComplianceSuite) TestOnBlockProposalInvalidHeight() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(&ancestor.Header)
	parent.Height = cs.head.Height // this should orphan the block
	block := unittest.BlockWithParentFixture(&parent.Header)
	block.Height = cs.head.Height
	proposal := messages.BlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}

	// store the data for retrieval
	cs.headerDB[parent.ID()] = &parent.Header
	cs.e.cache[parent.ID()] = &parent.Header
	cs.e.cache[ancestor.ID()] = &ancestor.Header

	// it should be processed without error
	err := cs.e.onBlockProposal(originID, &proposal)
	require.Error(cs.T(), err, "proposal with invalid ancestor should fail")

	// we should not store the payload
	cs.payloads.AssertNotCalled(cs.T(), "Store", mock.Anything, mock.Anything)

	// we should not store the header
	cs.headers.AssertNotCalled(cs.T(), "Store", mock.Anything)

	// we should not extend the state with the header
	cs.state.AssertNotCalled(cs.T(), "Mutate")
	cs.mutator.AssertNotCalled(cs.T(), "Extend", mock.Anything)

	// we should not submit the proposal to hotstuff
	cs.hotstuff.AssertNotCalled(cs.T(), "SubmitProposal", mock.Anything, mock.Anything)
}

func (cs *ComplianceSuite) TestOnBlockProposalInvalidExtension() {

	// create a proposal that has two ancestors in the cache
	originID := cs.participants[1].NodeID
	ancestor := unittest.BlockWithParentFixture(cs.head)
	parent := unittest.BlockWithParentFixture(&ancestor.Header)
	block := unittest.BlockWithParentFixture(&parent.Header)
	proposal := messages.BlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}

	// store the data for retrieval
	cs.headerDB[parent.ID()] = &parent.Header
	cs.e.cache[parent.ID()] = &parent.Header
	cs.e.cache[ancestor.ID()] = &ancestor.Header

	// make sure we fail to extend the state
	*cs.mutator = protocol.Mutator{}
	cs.mutator.On("Extend", mock.Anything).Return(errors.New("dummy error"))

	// it should be processed without error
	err := cs.e.onBlockProposal(originID, &proposal)
	require.Error(cs.T(), err, "proposal with invalid extension should fail")

	// we should store the payload
	cs.payloads.AssertCalled(cs.T(), "Store", &block.Header, &block.Payload)

	// we should store the header
	cs.headers.AssertCalled(cs.T(), "Store", &block.Header)

	// we should extend the state with the header
	cs.state.AssertCalled(cs.T(), "Mutate")
	cs.mutator.AssertCalled(cs.T(), "Extend", block.ID())

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

func (cs *ComplianceSuite) TestHandleMissingAncestor() {

	// create pending block
	originID := unittest.IdentifierFixture()
	ancestorID := unittest.IdentifierFixture()
	block := unittest.BlockFixture()
	proposal := messages.BlockProposal{
		Header:  &block.Header,
		Payload: &block.Payload,
	}

	// execute missing ancestors when (by default) we don't add to cache
	err := cs.e.handleMissingAncestor(originID, &proposal, ancestorID)
	require.NoError(cs.T(), err, "should pass with old proposl")
	cs.sync.AssertNotCalled(cs.T(), "RequestBlock", mock.Anything)

	// execute missing ancestor when we add to cache and should request
	*cs.cache = module.PendingBlockBuffer{}
	cs.cache.On("Add", mock.Anything).Return(true)
	err = cs.e.handleMissingAncestor(originID, &proposal, ancestorID)
	require.NoError(cs.T(), err, "should pass with new proposal")
	cs.sync.AssertCalled(cs.T(), "RequestBlock", ancestorID)
}

func (cs *ComplianceSuite) TestHandleConnectedChildrenNone() {

	// generate random block ID
	blockID := unittest.IdentifierFixture()

	// execute handle connected children
	err := cs.e.handleConnectedChildren(blockID)
	require.NoError(cs.T(), err, "should pass when no children")
	cs.snapshot.AssertNotCalled(cs.T(), "Final")
	cs.cache.AssertNotCalled(cs.T(), "DropForParent", mock.Anything)
}

func (cs *ComplianceSuite) TestHandleConnectedChildren() {

	// create three children blocks
	parent := unittest.BlockHeaderFixture()
	parent.Height = cs.head.Height + 1
	block1 := unittest.BlockWithParentFixture(&parent)
	block2 := unittest.BlockWithParentFixture(&parent)
	block3 := unittest.BlockWithParentFixture(&parent)

	// create the pending blocks
	pending1 := unittest.PendingBlockFixture(&block1)
	pending2 := unittest.PendingBlockFixture(&block2)
	pending3 := unittest.PendingBlockFixture(&block3)

	// make sure to return the children
	*cs.cache = module.PendingBlockBuffer{}
	cs.cache.On("ByParentID", parent.ID()).Return([]*flow.PendingBlock{pending1, pending2, pending3}, true)
	cs.cache.On("DropForParent", mock.Anything).Return()

	// make sure we quite first thing on handleMissingAncestor
	cs.cache.On("Add", mock.Anything).Return(false)

	// execute the connecte children handling
	err := cs.e.handleConnectedChildren(parent.ID())
	require.NoError(cs.T(), err, "should pass handling children")

	// check we tried to process all three children as proposals
	cs.snapshot.AssertNumberOfCalls(cs.T(), "Head", 3)

	// make sure we drop the cache after trying to process
	cs.cache.AssertCalled(cs.T(), "DropForParent", parent.ID())
}

func (cs *ComplianceSuite) TestProposalBufferingOrder() {

	// create a proposal that we will not submit until the end
	originID := cs.participants[1].NodeID
	rootBlock := unittest.BlockWithParentFixture(cs.head)
	rootProposal := messages.BlockProposal{
		Header:  &rootBlock.Header,
		Payload: &rootBlock.Payload,
	}

	// create a chain of descendants
	var descendants []*flow.Block
	var proposals []*messages.BlockProposal
	parent := &rootBlock
	for i := 0; i < 3; i++ {
		descendant := unittest.BlockWithParentFixture(&parent.Header)
		descendants = append(descendants, &descendant)
		proposal := messages.BlockProposal{
			Header:  &descendant.Header,
			Payload: &descendant.Payload,
		}
		proposals = append(proposals, &proposal)
		parent = &descendant
	}

	// reprogram the cache so we can add better check
	*cs.cache = module.PendingBlockBuffer{}
	cs.cache.On("ByParentID", mock.Anything).Return(
		func(parentID flow.Identifier) []*flow.PendingBlock {
			children := make([]*flow.PendingBlock, 0, len(cs.childrenDB[parentID]))
			for _, childID := range cs.childrenDB[parentID] {
				children = append(children, cs.pendingDB[childID])
			}
			return children
		},
		func(parentID flow.Identifier) bool {
			return len(cs.childrenDB[parentID]) > 0
		},
	)
	cs.cache.On("DropForParent", mock.Anything).Return(
		func(parentID flow.Identifier) {
			for _, childID := range cs.childrenDB[parentID] {
				delete(cs.pendingDB, childID)
			}
			delete(cs.childrenDB, parentID)
		},
	)

	// process/buffer all of the descendants
	for _, proposal := range proposals {

		// check that we store the pending block correctly
		cs.cache.On("Add", mock.Anything).Return(true).Once().Run(
			func(args mock.Arguments) {
				pending := args.Get(0).(*flow.PendingBlock)
				assert.Equal(cs.T(), originID, pending.OriginID, "should store correct origin in buffer")
				assert.Equal(cs.T(), proposal.Header, pending.Header, "should store correct header in buffer")
				assert.Equal(cs.T(), proposal.Payload, pending.Payload, "should store correct payload in buffer")
				cs.pendingDB[pending.Header.ID()] = pending
				cs.childrenDB[pending.Header.ParentID] = append(cs.childrenDB[pending.Header.ParentID], pending.Header.ID())
			},
		)

		// check that we request the ancestor block each time
		cs.sync.On("RequestBlock", mock.Anything).Once().Run(
			func(args mock.Arguments) {
				ancestorID := args.Get(0).(flow.Identifier)
				assert.Equal(cs.T(), rootBlock.ID(), ancestorID, "should always request root block")
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
	order := []flow.Identifier{rootBlock.ID(), descendants[0].ID(), descendants[1].ID(), descendants[2].ID()}
	cs.hotstuff.On("SubmitProposal", mock.Anything, mock.Anything).Times(4).Run(
		func(args mock.Arguments) {
			header := args.Get(0).(*flow.Header)
			assert.Equal(cs.T(), order[index], header.ID(), "should submit correct header to hotstuff")
			index++
		},
	)

	// process the root proposal
	err := cs.e.onBlockProposal(originID, &rootProposal)
	require.NoError(cs.T(), err, "root proposal should pass")

	// make sure we submitted all four proposals
	cs.hotstuff.AssertNumberOfCalls(cs.T(), "SubmitProposal", 4)
}
