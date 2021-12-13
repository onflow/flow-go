package integration

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/blockproducer"
	"github.com/onflow/flow-go/consensus/hotstuff/eventhandler"
	"github.com/onflow/flow-go/consensus/hotstuff/forks"
	"github.com/onflow/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/onflow/flow-go/consensus/hotstuff/forks/forkchoice"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	hsig "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/consensus/hotstuff/voter"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
	module "github.com/onflow/flow-go/module/mock"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

type Instance struct {

	// instance parameters
	participants flow.IdentityList
	localID      flow.Identifier
	blockVoteIn  VoteFilter
	blockVoteOut VoteFilter
	blockPropIn  ProposalFilter
	blockPropOut ProposalFilter
	stop         Condition

	// instance data
	queue   chan interface{}
	headers sync.Map //	headers map[flow.Identifier]*flow.Header

	// mocked dependencies
	committee    *mocks.Committee
	builder      *module.Builder
	finalizer    *module.Finalizer
	persist      *mocks.Persister
	signer       *mocks.Signer
	verifier     *mocks.Verifier
	communicator *mocks.Communicator

	// real dependencies
	pacemaker  hotstuff.PaceMaker
	producer   *blockproducer.BlockProducer
	forks      *forks.Forks
	aggregator *voteaggregator.VoteAggregator
	voter      *voter.Voter
	validator  *validator.Validator

	// main logic
	handler *eventhandler.EventHandler
}

func NewInstance(t require.TestingT, options ...Option) *Instance {

	// generate random default identity
	identity := unittest.IdentityFixture()

	// initialize the default configuration
	cfg := Config{
		Root:              DefaultRoot(),
		Participants:      flow.IdentityList{identity},
		LocalID:           identity.NodeID,
		Timeouts:          timeout.DefaultConfig,
		IncomingVotes:     BlockNoVotes,
		OutgoingVotes:     BlockNoVotes,
		IncomingProposals: BlockNoProposals,
		OutgoingProposals: BlockNoProposals,
		StopCondition:     RightAway,
	}

	// apply the custom options
	for _, option := range options {
		option(&cfg)
	}

	// check the local ID is a participant
	var index uint
	takesPart := false
	for i, participant := range cfg.Participants {
		if participant.NodeID == cfg.LocalID {
			index = uint(i)
			takesPart = true
			break
		}
	}
	require.True(t, takesPart)

	// initialize the instance
	in := Instance{

		// instance parameters
		participants: cfg.Participants,
		localID:      cfg.LocalID,
		blockVoteIn:  cfg.IncomingVotes,
		blockVoteOut: cfg.OutgoingVotes,
		blockPropIn:  cfg.IncomingProposals,
		blockPropOut: cfg.OutgoingProposals,
		stop:         cfg.StopCondition,

		// instance data
		queue: make(chan interface{}, 1024),

		// instance mocks
		committee:    &mocks.Committee{},
		builder:      &module.Builder{},
		persist:      &mocks.Persister{},
		signer:       &mocks.Signer{},
		verifier:     &mocks.Verifier{},
		communicator: &mocks.Communicator{},
		finalizer:    &module.Finalizer{},
	}

	// insert root block into headers register
	in.headers.Store(cfg.Root.ID(), cfg.Root)

	// program the hotstuff committee state
	in.committee.On("Identities", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, selector flow.IdentityFilter) flow.IdentityList {
			return in.participants.Filter(selector)
		},
		nil,
	)
	for _, participant := range in.participants {
		in.committee.On("Identity", mock.Anything, participant.NodeID).Return(participant, nil)
	}
	in.committee.On("Self").Return(in.localID)
	in.committee.On("LeaderForView", mock.Anything).Return(
		func(view uint64) flow.Identifier {
			return in.participants[int(view)%len(in.participants)].NodeID
		}, nil,
	)

	// program the builder module behaviour
	in.builder.On("BuildOn", mock.Anything, mock.Anything).Return(
		func(parentID flow.Identifier, setter func(*flow.Header) error) *flow.Header {
			parent, ok := in.headers.Load(parentID)
			if !ok {
				return nil
			}
			header := &flow.Header{
				ChainID:     "chain",
				ParentID:    parentID,
				Height:      parent.(*flow.Header).Height + 1,
				PayloadHash: unittest.IdentifierFixture(),
				Timestamp:   time.Now().UTC(),
			}
			require.NoError(t, setter(header))
			in.headers.Store(header.ID(), header)
			return header
		},
		func(parentID flow.Identifier, setter func(*flow.Header) error) error {
			_, ok := in.headers.Load(parentID)
			if !ok {
				return fmt.Errorf("parent block not found (parent: %x)", parentID)
			}
			return nil
		},
	)

	// check on stop condition, stop the tests as soon as entering a certain view
	in.persist.On("PutStarted", mock.Anything).Return(nil)
	in.persist.On("PutVoted", mock.Anything).Return(nil)

	// program the hotstuff signer behaviour
	in.signer.On("CreateProposal", mock.Anything).Return(
		func(block *model.Block) *model.Proposal {
			proposal := &model.Proposal{
				Block:   block,
				SigData: nil,
			}
			return proposal
		},
		nil,
	)
	in.signer.On("CreateVote", mock.Anything).Return(
		func(block *model.Block) *model.Vote {
			vote := &model.Vote{
				View:     block.View,
				BlockID:  block.BlockID,
				SignerID: in.localID,
				SigData:  unittest.RandomBytes(hsig.SigLen * 2), // double sig, one staking, one beacon
			}
			return vote
		},
		nil,
	)
	in.signer.On("CreateQC", mock.Anything).Return(
		func(votes []*model.Vote) *flow.QuorumCertificate {
			voterIDs := make([]flow.Identifier, 0, len(votes))
			for _, vote := range votes {
				voterIDs = append(voterIDs, vote.SignerID)
			}
			qc := &flow.QuorumCertificate{
				View:      votes[0].View,
				BlockID:   votes[0].BlockID,
				SignerIDs: voterIDs,
				SigData:   nil,
			}
			return qc
		},
		nil,
	)

	// program the hotstuff verifier behaviour
	in.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	in.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything).Return(true, nil)

	// program the hotstuff communicator behaviour
	in.communicator.On("BroadcastProposalWithDelay", mock.Anything, mock.Anything).Return(
		func(header *flow.Header, delay time.Duration) error {

			// sender should always have the parent
			parentBlob, exists := in.headers.Load(header.ParentID)
			if !exists {
				return fmt.Errorf("parent for proposal not found (sender: %x, parent: %x)", in.localID, header.ParentID)
			}
			parent := parentBlob.(*flow.Header)

			// set the height and chain ID
			header.ChainID = parent.ChainID
			header.Height = parent.Height + 1

			// convert into proposal immediately
			proposal := model.ProposalFromFlow(header, parent.View)

			// store locally and loop back to engine for processing
			in.headers.Store(header.ID(), header)
			in.queue <- proposal

			return nil
		},
	)
	in.communicator.On("SendVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// program the finalizer module behaviour
	in.finalizer.On("MakeFinal", mock.Anything).Return(
		func(blockID flow.Identifier) error {

			// as we don't use mocks to assert expectations, but only to
			// simulate behaviour, we should drop the call data regularly
			block, found := in.headers.Load(blockID)
			if !found {
				return fmt.Errorf("can't broadcast with unknown parent")
			}
			if block.(*flow.Header).Height%100 == 0 {
				in.committee.Calls = nil
				in.builder.Calls = nil
				in.signer.Calls = nil
				in.verifier.Calls = nil
				in.communicator.Calls = nil
				in.finalizer.Calls = nil
			}

			// check on stop condition
			// TODO: we can remove that once the single instance stop
			// recursively calling into itself
			if in.stop(&in) {
				return errStopCondition
			}

			return nil
		},
	)

	in.finalizer.On("MakeValid", mock.Anything).Return(nil)

	// initialize error handling and logging
	var err error
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	// log with node index an ID
	log := unittest.Logger().With().
		Int("index", int(index)).
		Hex("node_id", in.localID[:]).
		Logger()
	notifier := notifications.NewLogConsumer(log)

	// initialize the pacemaker
	controller := timeout.NewController(cfg.Timeouts)
	in.pacemaker, err = pacemaker.New(DefaultStart(), controller, notifier)
	require.NoError(t, err)

	// initialize the block producer
	in.producer, err = blockproducer.New(in.signer, in.committee, in.builder)
	require.NoError(t, err)

	// initialize the finalizer
	rootBlock := model.BlockFromFlow(cfg.Root, 0)
	rootQC := &flow.QuorumCertificate{
		View:      rootBlock.View,
		BlockID:   rootBlock.BlockID,
		SignerIDs: in.participants.NodeIDs(),
	}
	rootBlockQC := &forks.BlockQC{Block: rootBlock, QC: rootQC}
	forkalizer, err := finalizer.New(rootBlockQC, in.finalizer, notifier)
	require.NoError(t, err)

	// initialize the forks choice
	choice, err := forkchoice.NewNewestForkChoice(forkalizer, notifier)
	require.NoError(t, err)

	// initialize the forks handler
	in.forks = forks.New(forkalizer, choice)

	// initialize the validator
	in.validator = validator.New(in.committee, in.forks, in.verifier)

	stake := uint64(1000)
	stakingSigAggtor := helper.MakeWeightedSignatureAggregator(stake)
	stakingSigAggtor.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()

	rbRector := helper.MakeRandomBeaconReconstructor(msig.RandomBeaconThreshold(int(in.participants.Count())))
	rbRector.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()

	packer := &mocks.Packer{}
	packer.On("Pack", mock.Anything, mock.Anything).Return(in.participants.NodeIDs(), unittest.RandomBytes(128), nil).Maybe()

	onQCCreated := func(qc *flow.QuorumCertificate) {
		in.queue <- qc
	}

	minRequiredStake := hotstuff.ComputeStakeThresholdForBuildingQC(uint64(in.participants.Count()) * stake)
	voteProcessorFactory := &mocks.VoteProcessorFactory{}
	voteProcessorFactory.On("Create", mock.Anything, mock.Anything).Return(
		func(log zerolog.Logger, proposal *model.Proposal) hotstuff.VerifyingVoteProcessor {
			return votecollector.NewCombinedVoteProcessor(
				log, proposal.Block,
				stakingSigAggtor, rbRector,
				onQCCreated,
				packer,
				minRequiredStake,
			)
		}, nil)

	createCollectorFactoryMethod := votecollector.NewStateMachineFactory(log, notifier, voteProcessorFactory.Create)
	voteCollectors := voteaggregator.NewVoteCollectors(DefaultPruned(), workerpool.New(2), createCollectorFactoryMethod)

	// initialize the vote aggregator
	in.aggregator, err = voteaggregator.NewVoteAggregator(log, notifier, DefaultPruned(), voteCollectors)
	require.NoError(t, err)

	// initialize the voter
	in.voter = voter.New(in.signer, in.forks, in.persist, in.committee, DefaultVoted())

	// initialize the event handler
	in.handler, err = eventhandler.NewEventHandler(log, in.pacemaker, in.producer, in.forks, in.persist, in.communicator, in.committee, in.aggregator, in.voter, in.validator, notifier)
	require.NoError(t, err)

	return &in
}

func (in *Instance) Run() error {
	ctx, cancel := context.WithCancel(context.TODO())
	defer func() {
		cancel()
		<-in.aggregator.Done()
	}()
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)
	in.aggregator.Start(signalerCtx)
	<-in.aggregator.Ready()

	// start the event handler
	err := in.handler.Start()
	if err != nil {
		return fmt.Errorf("could not start event handler: %w", err)
	}

	// run until an error or stop condition is reached
	for {

		// check on stop conditions
		if in.stop(in) {
			return errStopCondition
		}

		// we handle timeouts with priority
		select {
		case <-in.handler.TimeoutChannel():
			err := in.handler.OnLocalTimeout()
			if err != nil {
				return fmt.Errorf("could not process timeout: %w", err)
			}
		default:
		}

		// check on stop conditions
		if in.stop(in) {
			return errStopCondition
		}

		// otherwise, process first received event
		select {
		case <-in.handler.TimeoutChannel():
			err := in.handler.OnLocalTimeout()
			if err != nil {
				return fmt.Errorf("could not process timeout: %w", err)
			}
		case msg := <-in.queue:
			switch m := msg.(type) {
			case *model.Proposal:
				err := in.handler.OnReceiveProposal(m)
				if err != nil {
					return fmt.Errorf("could not process proposal: %w", err)
				}
			case *model.Vote:
				in.aggregator.AddVote(m)
			case *flow.QuorumCertificate:
				err := in.handler.OnQCConstructed(m)
				if err != nil {
					return fmt.Errorf("could not process created qc: %w", err)
				}
			}
		}

	}
}
