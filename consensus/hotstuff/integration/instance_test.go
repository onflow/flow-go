package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gammazero/workerpool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/blockproducer"
	"github.com/onflow/flow-go/consensus/hotstuff/committees"
	"github.com/onflow/flow-go/consensus/hotstuff/eventhandler"
	"github.com/onflow/flow-go/consensus/hotstuff/forks"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/onflow/flow-go/consensus/hotstuff/safetyrules"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/timeoutcollector"
	"github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/votecollector"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/counters"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/metrics"
	module "github.com/onflow/flow-go/module/mock"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/utils/unittest"
)

type Instance struct {

	// instance parameters
	participants          flow.IdentityList
	localID               flow.Identifier
	blockVoteIn           VoteFilter
	blockVoteOut          VoteFilter
	blockPropIn           ProposalFilter
	blockPropOut          ProposalFilter
	blockTimeoutObjectIn  TimeoutObjectFilter
	blockTimeoutObjectOut TimeoutObjectFilter
	stop                  Condition

	// instance data
	queue          chan interface{}
	updatingBlocks sync.RWMutex
	headers        map[flow.Identifier]*flow.Header
	pendings       map[flow.Identifier]*model.Proposal // indexed by parent ID

	// mocked dependencies
	committee *mocks.DynamicCommittee
	builder   *module.Builder
	finalizer *module.Finalizer
	persist   *mocks.Persister
	signer    *mocks.Signer
	verifier  *mocks.Verifier
	notifier  *MockedCommunicatorConsumer

	// real dependencies
	pacemaker         hotstuff.PaceMaker
	producer          *blockproducer.BlockProducer
	forks             *forks.Forks
	voteAggregator    *voteaggregator.VoteAggregator
	timeoutAggregator *timeoutaggregator.TimeoutAggregator
	safetyRules       *safetyrules.SafetyRules
	validator         *validator.Validator

	// main logic
	handler *eventhandler.EventHandler
}

type MockedCommunicatorConsumer struct {
	notifications.NoopProposalViolationConsumer
	notifications.NoopParticipantConsumer
	notifications.NoopFinalizationConsumer
	*mocks.CommunicatorConsumer
}

func NewMockedCommunicatorConsumer() *MockedCommunicatorConsumer {
	return &MockedCommunicatorConsumer{
		CommunicatorConsumer: &mocks.CommunicatorConsumer{},
	}
}

var _ hotstuff.Consumer = (*MockedCommunicatorConsumer)(nil)
var _ hotstuff.TimeoutCollectorConsumer = (*Instance)(nil)

func NewInstance(t *testing.T, options ...Option) *Instance {

	// generate random default identity
	identity := unittest.IdentityFixture()

	// initialize the default configuration
	cfg := Config{
		Root:                   DefaultRoot(),
		Participants:           flow.IdentityList{identity},
		LocalID:                identity.NodeID,
		Timeouts:               timeout.DefaultConfig,
		IncomingVotes:          BlockNoVotes,
		OutgoingVotes:          BlockNoVotes,
		IncomingProposals:      BlockNoProposals,
		OutgoingProposals:      BlockNoProposals,
		IncomingTimeoutObjects: BlockNoTimeoutObjects,
		OutgoingTimeoutObjects: BlockNoTimeoutObjects,
		StopCondition:          RightAway,
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
		participants:          cfg.Participants,
		localID:               cfg.LocalID,
		blockVoteIn:           cfg.IncomingVotes,
		blockVoteOut:          cfg.OutgoingVotes,
		blockPropIn:           cfg.IncomingProposals,
		blockPropOut:          cfg.OutgoingProposals,
		blockTimeoutObjectIn:  cfg.IncomingTimeoutObjects,
		blockTimeoutObjectOut: cfg.OutgoingTimeoutObjects,
		stop:                  cfg.StopCondition,

		// instance data
		pendings: make(map[flow.Identifier]*model.Proposal),
		headers:  make(map[flow.Identifier]*flow.Header),
		queue:    make(chan interface{}, 1024),

		// instance mocks
		committee: &mocks.DynamicCommittee{},
		builder:   &module.Builder{},
		persist:   &mocks.Persister{},
		signer:    &mocks.Signer{},
		verifier:  &mocks.Verifier{},
		notifier:  NewMockedCommunicatorConsumer(),
		finalizer: &module.Finalizer{},
	}

	// insert root block into headers register
	in.headers[cfg.Root.ID()] = cfg.Root

	// program the hotstuff committee state
	in.committee.On("IdentitiesByEpoch", mock.Anything).Return(
		func(_ uint64) flow.IdentityList {
			return in.participants
		},
		nil,
	)
	for _, participant := range in.participants {
		in.committee.On("IdentityByBlock", mock.Anything, participant.NodeID).Return(participant, nil)
		in.committee.On("IdentityByEpoch", mock.Anything, participant.NodeID).Return(participant, nil)
	}
	in.committee.On("Self").Return(in.localID)
	in.committee.On("LeaderForView", mock.Anything).Return(
		func(view uint64) flow.Identifier {
			return in.participants[int(view)%len(in.participants)].NodeID
		}, nil,
	)
	in.committee.On("QuorumThresholdForView", mock.Anything).Return(committees.WeightThresholdToBuildQC(in.participants.TotalWeight()), nil)
	in.committee.On("TimeoutThresholdForView", mock.Anything).Return(committees.WeightThresholdToTimeout(in.participants.TotalWeight()), nil)

	// program the builder module behaviour
	in.builder.On("BuildOn", mock.Anything, mock.Anything).Return(
		func(parentID flow.Identifier, setter func(*flow.Header) error) *flow.Header {
			in.updatingBlocks.Lock()
			defer in.updatingBlocks.Unlock()

			parent, ok := in.headers[parentID]
			if !ok {
				return nil
			}
			header := &flow.Header{
				ChainID:     "chain",
				ParentID:    parentID,
				ParentView:  parent.View,
				Height:      parent.Height + 1,
				PayloadHash: unittest.IdentifierFixture(),
				Timestamp:   time.Now().UTC(),
			}
			require.NoError(t, setter(header))
			in.headers[header.ID()] = header
			return header
		},
		func(parentID flow.Identifier, setter func(*flow.Header) error) error {
			in.updatingBlocks.RLock()
			_, ok := in.headers[parentID]
			in.updatingBlocks.RUnlock()
			if !ok {
				return fmt.Errorf("parent block not found (parent: %x)", parentID)
			}
			return nil
		},
	)

	// check on stop condition, stop the tests as soon as entering a certain view
	in.persist.On("PutSafetyData", mock.Anything).Return(nil)
	in.persist.On("PutLivenessData", mock.Anything).Return(nil)

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
				SigData:  unittest.RandomBytes(msig.SigLen * 2), // double sig, one staking, one beacon
			}
			return vote
		},
		nil,
	)
	in.signer.On("CreateTimeout", mock.Anything, mock.Anything, mock.Anything).Return(
		func(curView uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) *model.TimeoutObject {
			timeoutObject := &model.TimeoutObject{
				View:       curView,
				NewestQC:   newestQC,
				LastViewTC: lastViewTC,
				SignerID:   in.localID,
				SigData:    unittest.RandomBytes(msig.SigLen),
			}
			return timeoutObject
		},
		nil,
	)
	in.signer.On("CreateQC", mock.Anything).Return(
		func(votes []*model.Vote) *flow.QuorumCertificate {
			voterIDs := make(flow.IdentifierList, 0, len(votes))
			for _, vote := range votes {
				voterIDs = append(voterIDs, vote.SignerID)
			}

			signerIndices, err := msig.EncodeSignersToIndices(in.participants.NodeIDs(), voterIDs)
			require.NoError(t, err, "could not encode signer indices")

			qc := &flow.QuorumCertificate{
				View:          votes[0].View,
				BlockID:       votes[0].BlockID,
				SignerIndices: signerIndices,
				SigData:       nil,
			}
			return qc
		},
		nil,
	)

	// program the hotstuff verifier behaviour
	in.verifier.On("VerifyVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	in.verifier.On("VerifyQC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	in.verifier.On("VerifyTC", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)

	// program the hotstuff communicator behaviour
	in.notifier.On("OnOwnProposal", mock.Anything, mock.Anything).Run(
		func(args mock.Arguments) {
			header, ok := args[0].(*flow.Header)
			require.True(t, ok)

			// sender should always have the parent
			in.updatingBlocks.RLock()
			_, exists := in.headers[header.ParentID]
			in.updatingBlocks.RUnlock()

			if !exists {
				t.Fatalf("parent for proposal not found parent: %x", header.ParentID)
			}

			// convert into proposal immediately
			proposal := model.ProposalFromFlow(header)

			// store locally and loop back to engine for processing
			in.ProcessBlock(proposal)
		},
	)
	in.notifier.On("OnOwnTimeout", mock.Anything).Run(func(args mock.Arguments) {
		timeoutObject, ok := args[0].(*model.TimeoutObject)
		require.True(t, ok)
		in.queue <- timeoutObject
	},
	)
	// in case of single node setup we should just forward vote to our own node
	// for multi-node setup this method will be overridden
	in.notifier.On("OnOwnVote", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
		in.queue <- &model.Vote{
			View:     args[1].(uint64),
			BlockID:  args[0].(flow.Identifier),
			SignerID: in.localID,
			SigData:  args[2].([]byte),
		}
	})

	// program the finalizer module behaviour
	in.finalizer.On("MakeFinal", mock.Anything).Return(
		func(blockID flow.Identifier) error {

			// as we don't use mocks to assert expectations, but only to
			// simulate behaviour, we should drop the call data regularly
			in.updatingBlocks.RLock()
			block, found := in.headers[blockID]
			in.updatingBlocks.RUnlock()
			if !found {
				return fmt.Errorf("can't broadcast with unknown parent")
			}
			if block.Height%100 == 0 {
				in.committee.Calls = nil
				in.builder.Calls = nil
				in.signer.Calls = nil
				in.verifier.Calls = nil
				in.notifier.Calls = nil
				in.finalizer.Calls = nil
			}

			return nil
		},
	)

	// initialize error handling and logging
	var err error
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }
	// log with node index an ID
	log := unittest.Logger().With().
		Int("index", int(index)).
		Hex("node_id", in.localID[:]).
		Logger()
	notifier := pubsub.NewDistributor()
	logConsumer := notifications.NewLogConsumer(log)
	notifier.AddConsumer(logConsumer)
	notifier.AddConsumer(in.notifier)

	// initialize the block producer
	in.producer, err = blockproducer.New(in.signer, in.committee, in.builder)
	require.NoError(t, err)

	// initialize the finalizer
	rootBlock := model.BlockFromFlow(cfg.Root)

	signerIndices, err := msig.EncodeSignersToIndices(in.participants.NodeIDs(), in.participants.NodeIDs())
	require.NoError(t, err, "could not encode signer indices")

	rootQC := &flow.QuorumCertificate{
		View:          rootBlock.View,
		BlockID:       rootBlock.BlockID,
		SignerIndices: signerIndices,
	}
	certifiedRootBlock, err := model.NewCertifiedBlock(rootBlock, rootQC)
	require.NoError(t, err)

	livenessData := &hotstuff.LivenessData{
		CurrentView: rootQC.View + 1,
		NewestQC:    rootQC,
	}

	in.persist.On("GetLivenessData").Return(livenessData, nil).Once()

	// initialize the pacemaker
	controller := timeout.NewController(cfg.Timeouts)
	in.pacemaker, err = pacemaker.New(controller, pacemaker.NoProposalDelay(), notifier, in.persist)
	require.NoError(t, err)

	// initialize the forks handler
	in.forks, err = forks.New(&certifiedRootBlock, in.finalizer, notifier)
	require.NoError(t, err)

	// initialize the validator
	in.validator = validator.New(in.committee, in.verifier)

	weight := uint64(flow.DefaultInitialWeight)

	indices, err := msig.EncodeSignersToIndices(in.participants.NodeIDs(), []flow.Identifier(in.participants.NodeIDs()))
	require.NoError(t, err)

	packer := &mocks.Packer{}
	packer.On("Pack", mock.Anything, mock.Anything).Return(indices, unittest.RandomBytes(128), nil).Maybe()

	onQCCreated := func(qc *flow.QuorumCertificate) {
		in.queue <- qc
	}

	minRequiredWeight := committees.WeightThresholdToBuildQC(uint64(in.participants.Count()) * weight)
	voteProcessorFactory := mocks.NewVoteProcessorFactory(t)
	voteProcessorFactory.On("Create", mock.Anything, mock.Anything).Return(
		func(log zerolog.Logger, proposal *model.Proposal) hotstuff.VerifyingVoteProcessor {
			stakingSigAggtor := helper.MakeWeightedSignatureAggregator(weight)
			stakingSigAggtor.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()

			rbRector := helper.MakeRandomBeaconReconstructor(msig.RandomBeaconThreshold(int(in.participants.Count())))
			rbRector.On("Verify", mock.Anything, mock.Anything).Return(nil).Maybe()

			return votecollector.NewCombinedVoteProcessor(
				log, proposal.Block,
				stakingSigAggtor, rbRector,
				onQCCreated,
				packer,
				minRequiredWeight,
			)
		}, nil).Maybe()

	voteAggregationDistributor := pubsub.NewVoteAggregationDistributor()
	createCollectorFactoryMethod := votecollector.NewStateMachineFactory(log, voteAggregationDistributor, voteProcessorFactory.Create)
	voteCollectors := voteaggregator.NewVoteCollectors(log, livenessData.CurrentView, workerpool.New(2), createCollectorFactoryMethod)

	metricsCollector := metrics.NewNoopCollector()

	// initialize the vote aggregator
	in.voteAggregator, err = voteaggregator.NewVoteAggregator(
		log,
		metricsCollector,
		metricsCollector,
		metricsCollector,
		voteAggregationDistributor,
		livenessData.CurrentView,
		voteCollectors,
	)
	require.NoError(t, err)

	// initialize factories for timeout collector and timeout processor
	timeoutAggregationDistributor := pubsub.NewTimeoutAggregationDistributor()
	timeoutProcessorFactory := mocks.NewTimeoutProcessorFactory(t)
	timeoutProcessorFactory.On("Create", mock.Anything).Return(
		func(view uint64) hotstuff.TimeoutProcessor {
			// mock signature aggregator which doesn't perform any crypto operations and just tracks total weight
			aggregator := &mocks.TimeoutSignatureAggregator{}
			totalWeight := atomic.NewUint64(0)
			newestView := counters.NewMonotonousCounter(0)
			aggregator.On("View").Return(view).Maybe()
			aggregator.On("TotalWeight").Return(func() uint64 {
				return totalWeight.Load()
			}).Maybe()
			aggregator.On("VerifyAndAdd", mock.Anything, mock.Anything, mock.Anything).Return(
				func(signerID flow.Identifier, _ crypto.Signature, newestQCView uint64) uint64 {
					newestView.Set(newestQCView)
					identity, ok := in.participants.ByNodeID(signerID)
					require.True(t, ok)
					return totalWeight.Add(identity.Weight)
				}, nil,
			).Maybe()
			aggregator.On("Aggregate", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
				func() []hotstuff.TimeoutSignerInfo {
					signersData := make([]hotstuff.TimeoutSignerInfo, 0, len(in.participants))
					newestQCView := newestView.Value()
					for _, signer := range in.participants.NodeIDs() {
						signersData = append(signersData, hotstuff.TimeoutSignerInfo{
							NewestQCView: newestQCView,
							Signer:       signer,
						})
					}
					return signersData
				},
				unittest.SignatureFixture(),
				nil,
			).Maybe()

			p, err := timeoutcollector.NewTimeoutProcessor(
				unittest.Logger(),
				in.committee,
				in.validator,
				aggregator,
				timeoutAggregationDistributor,
			)
			require.NoError(t, err)
			return p
		}, nil).Maybe()
	timeoutCollectorFactory := timeoutcollector.NewTimeoutCollectorFactory(
		unittest.Logger(),
		timeoutAggregationDistributor,
		timeoutProcessorFactory,
	)
	timeoutCollectors := timeoutaggregator.NewTimeoutCollectors(
		log,
		metricsCollector,
		livenessData.CurrentView,
		timeoutCollectorFactory,
	)

	// initialize the timeout aggregator
	in.timeoutAggregator, err = timeoutaggregator.NewTimeoutAggregator(
		log,
		metricsCollector,
		metricsCollector,
		metricsCollector,
		livenessData.CurrentView,
		timeoutCollectors,
	)
	require.NoError(t, err)

	safetyData := &hotstuff.SafetyData{
		LockedOneChainView:      rootBlock.View,
		HighestAcknowledgedView: rootBlock.View,
	}
	in.persist.On("GetSafetyData", mock.Anything).Return(safetyData, nil).Once()

	// initialize the safety rules
	in.safetyRules, err = safetyrules.New(in.signer, in.persist, in.committee)
	require.NoError(t, err)

	// initialize the event handler
	in.handler, err = eventhandler.NewEventHandler(
		log,
		in.pacemaker,
		in.producer,
		in.forks,
		in.persist,
		in.committee,
		in.safetyRules,
		notifier,
	)
	require.NoError(t, err)

	timeoutAggregationDistributor.AddTimeoutCollectorConsumer(logConsumer)
	timeoutAggregationDistributor.AddTimeoutCollectorConsumer(&in)

	voteAggregationDistributor.AddVoteCollectorConsumer(logConsumer)

	return &in
}

func (in *Instance) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		<-util.AllDone(in.voteAggregator, in.timeoutAggregator)
	}()
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)
	in.voteAggregator.Start(signalerCtx)
	in.timeoutAggregator.Start(signalerCtx)
	<-util.AllReady(in.voteAggregator, in.timeoutAggregator)

	// start the event handler
	err := in.handler.Start(ctx)
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
				// add block to aggregator
				in.voteAggregator.AddBlock(m)
				// then pass to event handler
				err := in.handler.OnReceiveProposal(m)
				if err != nil {
					return fmt.Errorf("could not process proposal: %w", err)
				}
			case *model.Vote:
				in.voteAggregator.AddVote(m)
			case *model.TimeoutObject:
				in.timeoutAggregator.AddTimeout(m)
			case *flow.QuorumCertificate:
				err := in.handler.OnReceiveQc(m)
				if err != nil {
					return fmt.Errorf("could not process received QC: %w", err)
				}
			case *flow.TimeoutCertificate:
				err := in.handler.OnReceiveTc(m)
				if err != nil {
					return fmt.Errorf("could not process received TC: %w", err)
				}
			case *hotstuff.PartialTcCreated:
				err := in.handler.OnPartialTcCreated(m)
				if err != nil {
					return fmt.Errorf("could not process partial TC: %w", err)
				}
			}
		}
	}
}

func (in *Instance) ProcessBlock(proposal *model.Proposal) {
	in.updatingBlocks.Lock()
	defer in.updatingBlocks.Unlock()
	_, parentExists := in.headers[proposal.Block.QC.BlockID]

	if parentExists {
		next := proposal
		for next != nil {
			in.headers[next.Block.BlockID] = model.ProposalToFlow(next)

			in.queue <- next
			// keep processing the pending blocks
			next = in.pendings[next.Block.QC.BlockID]
		}
	} else {
		// cache it in pendings by ParentID
		in.pendings[proposal.Block.QC.BlockID] = proposal
	}
}

func (in *Instance) OnTcConstructedFromTimeouts(tc *flow.TimeoutCertificate) {
	in.queue <- tc
}

func (in *Instance) OnPartialTcCreated(view uint64, newestQC *flow.QuorumCertificate, lastViewTC *flow.TimeoutCertificate) {
	in.queue <- &hotstuff.PartialTcCreated{
		View:       view,
		NewestQC:   newestQC,
		LastViewTC: lastViewTC,
	}
}

func (in *Instance) OnNewQcDiscovered(qc *flow.QuorumCertificate) {
	in.queue <- qc
}

func (in *Instance) OnNewTcDiscovered(tc *flow.TimeoutCertificate) {
	in.queue <- tc
}

func (in *Instance) OnTimeoutProcessed(*model.TimeoutObject) {}
