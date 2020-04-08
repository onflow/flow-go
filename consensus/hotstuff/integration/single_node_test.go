package integration

import (
	"errors"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gotest.tools/assert"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/blockproducer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/eventhandler"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/forkchoice"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voter"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/filter"
	module "github.com/dapperlabs/flow-go/module/mock"
	protocol "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

var errEndReached = errors.New("end of test reached")

func TestSingle(t *testing.T) {
	suite.Run(t, new(SingleSuite))
}

type SingleSuite struct {
	suite.Suite

	// test parameters
	threshold uint64 // how many blocks we want to be finalized
	finalized uint64 // how many blocks have been finalized

	// hotstuff parameters
	identity   *flow.Identity
	startView  uint64
	prunedView uint64
	votedView  uint64

	// hotstuff trusted root
	block *model.Block
	qc    *model.QuorumCertificate

	// mocked data registries
	headers map[flow.Identifier]*flow.Header

	// mocked modules
	proto        *protocol.State
	snapshot     *protocol.Snapshot
	builder      *module.Builder
	finalizer    *module.Finalizer
	signer       *mocks.Signer
	verifier     *mocks.Verifier
	communicator *mocks.Communicator

	// tested modules
	viewstate  *viewstate.ViewState
	pacemaker  *pacemaker.FlowPaceMaker
	producer   *blockproducer.BlockProducer
	forks      *forks.Forks
	aggregator *voteaggregator.VoteAggregator
	voter      *voter.Voter
	validator  *validator.Validator

	// main logic
	handler *eventhandler.EventHandler
	loop    *hotstuff.EventLoop
}

func (ss *SingleSuite) SetupTest() {

	// set up some auxiliary stuff
	var err error
	log := zerolog.New(ioutil.Discard)

	// set test parameters
	ss.threshold = 128 // how many blocks to finalize before stopping

	// set default parameters
	ss.identity = unittest.IdentityFixture()
	ss.startView = 1
	ss.prunedView = 0
	ss.votedView = 0

	// set up trusted root block
	ss.block = &model.Block{
		View:        0,
		BlockID:     unittest.IdentifierFixture(),
		ProposerID:  flow.ZeroID,
		QC:          nil,
		PayloadHash: flow.ZeroID,
		Timestamp:   time.Now().UTC(),
	}
	ss.qc = &model.QuorumCertificate{
		View:      ss.block.View,
		BlockID:   ss.block.BlockID,
		SignerIDs: nil,
		SigData:   nil,
	}

	// initialize data registries
	ss.headers = make(map[flow.Identifier]*flow.Header)
	ss.headers[ss.block.BlockID] = &flow.Header{
		ChainID:     "chain",
		ParentID:    flow.ZeroID,
		Height:      0,
		PayloadHash: ss.block.PayloadHash,
		Timestamp:   ss.block.Timestamp,
		View:        ss.block.View,
	}

	// set up the protocol snapshot mock
	ss.snapshot = &protocol.Snapshot{}
	ss.snapshot.On("Identity", ss.identity.NodeID).Return(ss.identity, nil)
	ss.snapshot.On("Identities", mock.Anything).Return(flow.IdentityList{ss.identity}, nil)
	ss.snapshot.On("Identities", mock.Anything, mock.Anything).Return(flow.IdentityList{ss.identity}, nil)

	// set up the protocol state mock
	ss.proto = &protocol.State{}
	ss.proto.On("Final").Return(ss.snapshot)
	ss.proto.On("AtNumber", mock.Anything).Return(ss.snapshot)
	ss.proto.On("AtBlockID", mock.Anything).Return(ss.snapshot)

	// set up the module builder mock
	ss.builder = &module.Builder{}
	ss.builder.On("BuildOn", mock.Anything, mock.Anything).Return(
		func(parentID flow.Identifier, setter func(*flow.Header)) *flow.Header {
			parent, ok := ss.headers[parentID]
			if !ok {
				return nil
			}
			header := &flow.Header{
				ChainID:     "chain",
				ParentID:    parentID,
				Height:      parent.Height + 1,
				Timestamp:   time.Now().UTC(),
				PayloadHash: unittest.IdentifierFixture(),
			}
			setter(header)
			headerID := header.ID()
			ss.headers[headerID] = header
			return header
		},
		func(parentID flow.Identifier, setter func(*flow.Header)) error {
			_, ok := ss.headers[parentID]
			if !ok {
				return fmt.Errorf("building on unknown (parent: %x)", parentID)
			}
			return nil
		},
	)

	// set up the hotstuff verifier mock
	ss.signer = &mocks.Signer{}
	ss.signer.On("CreateProposal", mock.Anything).Return(
		func(block *model.Block) *model.Proposal {
			proposal := &model.Proposal{
				Block:   block,
				SigData: nil,
			}
			return proposal
		},
		nil,
	)
	ss.signer.On("CreateVote", mock.Anything).Return(
		func(block *model.Block) *model.Vote {
			vote := &model.Vote{
				View:     block.View,
				BlockID:  block.BlockID,
				SignerID: block.ProposerID,
				SigData:  nil,
			}
			return vote
		},
		nil,
	)
	ss.signer.On("CreateQC", mock.Anything).Return(
		func(votes []*model.Vote) *model.QuorumCertificate {
			voterIDs := make([]flow.Identifier, 0, len(votes))
			for _, vote := range votes {
				voterIDs = append(voterIDs, vote.SignerID)
			}
			qc := &model.QuorumCertificate{
				View:      votes[0].View,
				BlockID:   votes[0].BlockID,
				SignerIDs: voterIDs,
				SigData:   nil,
			}
			return qc
		},
		nil,
	)

	// set up the hotstuff signer
	ss.verifier = &mocks.Verifier{}
	ss.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// set up the hotstuff communicator mock
	ss.communicator = &mocks.Communicator{}
	ss.communicator.On("BroadcastProposal", mock.Anything).Return(nil)

	// set up the module finalizer mock
	ss.finalizer = &module.Finalizer{}
	ss.finalizer.On("MakeFinal", mock.Anything).Return(
		func(blockID flow.Identifier) error {

			// as we don't use mocks to assert expectations, but only to
			// simulate behaviour, we should drop the call data regularly
			if len(ss.headers)%100 == 0 {
				ss.snapshot.Calls = nil
				ss.proto.Calls = nil
				ss.builder.Calls = nil
				ss.signer.Calls = nil
				ss.verifier.Calls = nil
				ss.communicator.Calls = nil
				ss.finalizer.Calls = nil
			}

			// if we have reached the number of blocks we want to finalize, we
			// return a sentinel error to stop hotstuff execution
			ss.finalized++
			if ss.finalized >= ss.threshold {
				return errEndReached
			}

			return nil
		},
	)

	// initialize a noop notifier
	notifier := notifications.NewNoopConsumer()

	// initialize the viewstate
	ss.viewstate, err = viewstate.New(ss.proto, ss.identity.NodeID, filter.HasNodeID(ss.identity.NodeID))

	// initialize the pacemaker
	ss.pacemaker, err = pacemaker.New(ss.startView, timeout.DefaultController(), notifier)
	require.NoError(ss.T(), err)

	// initialize the block producer
	ss.producer, err = blockproducer.New(ss.signer, ss.viewstate, ss.builder)
	require.NoError(ss.T(), err)

	// initialize the finalizer
	root := &forks.BlockQC{Block: ss.block, QC: ss.qc}
	forkalizer, err := finalizer.New(root, ss.finalizer, notifier)
	require.NoError(ss.T(), err)

	// initialize the forks choice
	choice, err := forkchoice.NewNewestForkChoice(forkalizer, notifier)
	require.NoError(ss.T(), err)

	// initialize the forks handler
	ss.forks = forks.New(forkalizer, choice)

	// initialize the validator
	ss.validator = validator.New(ss.viewstate, ss.forks, ss.verifier)

	// initialize the vote aggregator
	ss.aggregator = voteaggregator.New(notifier, ss.prunedView, ss.viewstate, ss.validator, ss.signer)

	// initialize the voter
	ss.voter = voter.New(ss.signer, ss.forks, ss.votedView)

	// initialize the event handler
	ss.handler, err = eventhandler.New(log, ss.pacemaker, ss.producer, ss.forks, ss.communicator, ss.viewstate, ss.aggregator, ss.voter, ss.validator)
	require.NoError(ss.T(), err)

	// initialize and return the event loop
	ss.loop, err = hotstuff.NewEventLoop(log, ss.handler)
	require.NoError(ss.T(), err)
}

func (ss *SingleSuite) TestInitialization() {
	err := ss.loop.Start()
	require.True(ss.T(), errors.Is(err, errEndReached))
	assert.Equal(ss.T(), ss.finalized, ss.forks.FinalizedView())
	assert.Equal(ss.T(), ss.finalized+3, ss.pacemaker.CurView())
}
