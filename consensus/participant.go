package consensus

import (
	"fmt"
	"github.com/onflow/flow-go/consensus/hotstuff/eventloop"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/blockproducer"
	"github.com/onflow/flow-go/consensus/hotstuff/eventhandler"
	"github.com/onflow/flow-go/consensus/hotstuff/forks"
	"github.com/onflow/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/onflow/flow-go/consensus/hotstuff/forks/forkchoice"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker"
	"github.com/onflow/flow-go/consensus/hotstuff/pacemaker/timeout"
	validatorImpl "github.com/onflow/flow-go/consensus/hotstuff/validator"
	"github.com/onflow/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/onflow/flow-go/consensus/hotstuff/voter"
	"github.com/onflow/flow-go/consensus/recovery"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// NewParticipant initialize the EventLoop instance and recover the forks' state with all pending block
func NewParticipant(
	log zerolog.Logger,
	notifier hotstuff.Consumer,
	metrics module.HotstuffMetrics,
	headers storage.Headers,
	committee hotstuff.Committee,
	builder module.Builder,
	updater module.Finalizer,
	persist hotstuff.Persister,
	signer hotstuff.SignerVerifier,
	communicator hotstuff.Communicator,
	rootHeader *flow.Header,
	rootQC *flow.QuorumCertificate,
	finalized *flow.Header,
	pending []*flow.Header,
	options ...Option,
) (*hotstuff.EventLoop, error) {

	// initialize the default configuration
	defTimeout := timeout.DefaultConfig
	cfg := ParticipantConfig{
		TimeoutInitial:             time.Duration(defTimeout.ReplicaTimeout) * time.Millisecond,
		TimeoutMinimum:             time.Duration(defTimeout.MinReplicaTimeout) * time.Millisecond,
		TimeoutAggregationFraction: defTimeout.VoteAggregationTimeoutFraction,
		TimeoutIncreaseFactor:      defTimeout.TimeoutIncrease,
		TimeoutDecreaseFactor:      defTimeout.TimeoutDecrease,
		BlockRateDelay:             time.Duration(defTimeout.BlockRateDelayMS) * time.Millisecond,
	}

	// apply the configuration options
	for _, option := range options {
		option(&cfg)
	}

	// initialize forks with only finalized block.
	// pending blocks was not recovered yet
	forks, err := initForks(finalized, headers, updater, notifier, rootHeader, rootQC)
	if err != nil {
		return nil, fmt.Errorf("could not recover forks: %w", err)
	}

	// initialize the validator
	var validator hotstuff.Validator
	validator = validatorImpl.New(committee, forks, signer)
	validator = validatorImpl.NewMetricsWrapper(validator, metrics) // wrapper for measuring time spent in Validator component

	// get the last view we started
	started, err := persist.GetStarted()
	if err != nil {
		return nil, fmt.Errorf("could not recover last started: %w", err)
	}

	// get the last view we voted
	voted, err := persist.GetVoted()
	if err != nil {
		return nil, fmt.Errorf("could not recover last voted: %w", err)
	}

	// initialize the vote aggregator
	aggregator := voteaggregator.New(notifier, 0, committee, validator, signer)

	// recover the hotstuff state, mainly to recover all pending blocks
	// in forks
	err = recovery.Participant(log, forks, aggregator, validator, finalized, pending)
	if err != nil {
		return nil, fmt.Errorf("could not recover hotstuff state: %w", err)
	}

	// initialize the timeout config
	timeoutConfig, err := timeout.NewConfig(
		cfg.TimeoutInitial,
		cfg.TimeoutMinimum,
		cfg.TimeoutAggregationFraction,
		cfg.TimeoutIncreaseFactor,
		cfg.TimeoutDecreaseFactor,
		cfg.BlockRateDelay,
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize timeout config: %w", err)
	}

	// initialize the pacemaker
	controller := timeout.NewController(timeoutConfig)
	pacemaker, err := pacemaker.New(started+1, controller, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize flow pacemaker: %w", err)
	}

	// initialize block producer
	producer, err := blockproducer.New(signer, committee, builder)
	if err != nil {
		return nil, fmt.Errorf("could not initialize block producer: %w", err)
	}

	// initialize the voter
	voter := voter.New(signer, forks, persist, committee, voted)

	// initialize the event handler
	handler, err := eventhandler.New(log, pacemaker, producer, forks, persist, communicator, committee, aggregator, voter, validator, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event handler: %w", err)
	}

	// initialize and return the event loop
	// TODO: add proper event handler when it's replaced
	loop, err := eventloop.NewEventLoop(log, metrics, nil, cfg.StartupTime)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event loop: %w", err)
	}

	return loop, nil
}

func initForks(final *flow.Header, headers storage.Headers, updater module.Finalizer, notifier hotstuff.Consumer, rootHeader *flow.Header, rootQC *flow.QuorumCertificate) (*forks.Forks, error) {
	finalizer, err := initFinalizer(final, headers, updater, notifier, rootHeader, rootQC)
	if err != nil {
		return nil, fmt.Errorf("could not initialize finalizer: %w", err)
	}

	// initialize the fork choice
	forkchoice, err := forkchoice.NewNewestForkChoice(finalizer, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize fork choice: %w", err)
	}

	// initialize the forks manager
	forks := forks.New(finalizer, forkchoice)
	return forks, nil
}

func initFinalizer(final *flow.Header, headers storage.Headers, updater module.Finalizer, notifier hotstuff.FinalizationConsumer, rootHeader *flow.Header, rootQC *flow.QuorumCertificate) (*finalizer.Finalizer, error) {
	// recover the trusted root
	trustedRoot, err := recoverTrustedRoot(final, headers, rootHeader, rootQC)
	if err != nil {
		return nil, fmt.Errorf("could not recover trusted root: %w", err)
	}

	// initialize the finalizer
	finalizer, err := finalizer.New(trustedRoot, updater, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize finalizer: %w", err)
	}

	return finalizer, nil
}

func recoverTrustedRoot(final *flow.Header, headers storage.Headers, rootHeader *flow.Header, rootQC *flow.QuorumCertificate) (*forks.BlockQC, error) {
	if final.View < rootHeader.View {
		return nil, fmt.Errorf("finalized Block has older view than trusted root")
	}

	// if finalized view is genesis block, then use genesis block as the trustedRoot
	if final.View == rootHeader.View {
		if final.ID() != rootHeader.ID() {
			return nil, fmt.Errorf("finalized Block conflicts with trusted root")
		}
		return makeRootBlockQC(rootHeader, rootQC), nil
	}

	// get the parent for the latest finalized block
	parent, err := headers.ByBlockID(final.ParentID)
	if err != nil {
		return nil, fmt.Errorf("could not get parent for finalized: %w", err)
	}

	// find a valid child of the finalized block in order to get its QC
	children, err := headers.ByParentID(final.ID())
	if err != nil {
		// a finalized block must have a valid child, if err happens, we exit
		return nil, fmt.Errorf("could not get children for finalized block (ID: %v, view: %v): %w", final.ID(), final.View, err)
	}
	if len(children) == 0 {
		return nil, fmt.Errorf("finalized block has no children")
	}

	child := model.BlockFromFlow(children[0], final.View)

	// create the root block to use
	trustedRoot := &forks.BlockQC{
		Block: model.BlockFromFlow(final, parent.View),
		QC:    child.QC,
	}

	return trustedRoot, nil
}

func makeRootBlockQC(header *flow.Header, qc *flow.QuorumCertificate) *forks.BlockQC {
	// By convention of Forks, the trusted root block does not need to have a qc
	// (as is the case for the genesis block). For simplify of the implementation, we always omit
	// the QC of the root block. Thereby, we have one algorithm which handles all cases,
	// instead of having to distinguish between a genesis block without a qc
	// and a later-finalized root block where we can retrieve the qc.
	rootBlock := &model.Block{
		View:        header.View,
		BlockID:     header.ID(),
		ProposerID:  header.ProposerID,
		QC:          nil, // QC is omitted
		PayloadHash: header.PayloadHash,
		Timestamp:   header.Timestamp,
	}
	return &forks.BlockQC{
		QC:    qc,
		Block: rootBlock,
	}
}
