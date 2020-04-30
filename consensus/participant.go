package consensus

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/blockproducer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/eventhandler"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/forkchoice"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/persister"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voter"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

// NewParticipant initialize the EventLoop instance and recover the forks' state with all unfinalized block
func NewParticipant(log zerolog.Logger, notifier hotstuff.Consumer, metrics module.Metrics, headers storage.Headers,
	views storage.Views, committee hotstuff.Committee, protocolState protocol.State, builder module.Builder, updater module.Finalizer,
	signer hotstuff.Signer, communicator hotstuff.Communicator, rootHeader *flow.Header, rootQC *model.QuorumCertificate,
	finalized *flow.Header, unfinalized []*flow.Header, options ...Option) (*hotstuff.EventLoop, error) {

	// initialize the default configuration
	defTimeout := timeout.DefaultConfig
	cfg := ParticipantConfig{
		TimeoutInitial:             time.Duration(defTimeout.ReplicaTimeout) * time.Millisecond,
		TimeoutMinimum:             time.Duration(defTimeout.MinReplicaTimeout) * time.Millisecond,
		TimeoutAggregationFraction: defTimeout.VoteAggregationTimeoutFraction,
		TimeoutIncreaseFactor:      defTimeout.TimeoutIncrease,
		TimeoutDecreaseStep:        time.Duration(defTimeout.TimeoutDecrease) * time.Millisecond,
	}

	// apply the configuration options
	for _, option := range options {
		option(&cfg)
	}

	// initialize forks with only finalized block.
	// unfinalized blocks was not recovered yet
	forks, err := initForks(finalized, unfinalized, headers, updater, notifier, rootHeader, rootQC)
	if err != nil {
		return nil, fmt.Errorf("could not recover forks: %w", err)
	}

	// initialize the validator
	validator := validator.New(committee, forks, signer)

	// get the last view we started
	lastStarted, err := views.Retrieve(persister.ActionStarted)
	if err != nil {
		return nil, fmt.Errorf("could not recover last started: %w", err)
	}

	// get the last view we voted
	lastVoted, err := views.Retrieve(persister.ActionVoted)
	if err != nil {
		return nil, fmt.Errorf("could not recover last voted: %w", err)
	}

	// recover the hotstuff state, mainly to recover all unfinalized blocks
	// in forks
	err = Recover(log, forks, validator, unfinalized)
	if err != nil {
		return nil, fmt.Errorf("could not recover hotstuff state: %w", err)
	}

	// initialize the timeout config
	timeoutConfig, err := timeout.NewConfig(
		cfg.TimeoutInitial,
		cfg.TimeoutMinimum,
		cfg.TimeoutAggregationFraction,
		cfg.TimeoutIncreaseFactor,
		cfg.TimeoutDecreaseStep,
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize timeout config: %w", err)
	}

	// initialize the pacemaker
	controller := timeout.NewController(timeoutConfig)
	persist := persister.New(views)
	pacemaker, err := pacemaker.New(lastStarted+1, controller, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize flow pacemaker: %w", err)
	}

	// initialize block producer
	producer, err := blockproducer.New(signer, committee, builder)
	if err != nil {
		return nil, fmt.Errorf("could not initialize block producer: %w", err)
	}

	// initialize the voter
	voter := voter.New(signer, forks, persist, lastVoted)

	// initialize the vote aggregator
	aggregator := voteaggregator.New(notifier, 0, committee, validator, signer)

	// initialize the event handler
	handler, err := eventhandler.New(log, pacemaker, producer, forks, persist, communicator, committee, aggregator, voter, validator, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event handler: %w", err)
	}

	// initialize and return the event loop
	loop, err := hotstuff.NewEventLoop(log, metrics, handler)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event loop: %w", err)
	}

	return loop, nil
}

func initForks(final *flow.Header, unfinalized []*flow.Header, headers storage.Headers, updater module.Finalizer, notifier hotstuff.Consumer, rootHeader *flow.Header, rootQC *model.QuorumCertificate) (*forks.Forks, error) {
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

	// initialize the fork choice
	forkchoice, err := forkchoice.NewNewestForkChoice(finalizer, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize fork choice: %w", err)
	}

	// initialize the forks manager
	forks := forks.New(finalizer, forkchoice)
	return forks, nil
}

func recoverTrustedRoot(final *flow.Header, headers storage.Headers, rootHeader *flow.Header, rootQC *model.QuorumCertificate) (*forks.BlockQC, error) {
	if final == nil {
		return makeRootBlockQC(rootHeader, rootQC), nil
	}

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
	childHeader, err := headers.ByParentID(final.ID())
	if err != nil {
		// a finalized block must have a valid child, if err happens, we exit
		return nil, fmt.Errorf("could not get a valid child for finalized block: %w", err)
	}

	child := model.BlockFromFlow(childHeader, final.View)

	// create the root block to use
	trustedRoot := &forks.BlockQC{
		Block: model.BlockFromFlow(final, parent.View),
		QC:    child.QC,
	}

	return trustedRoot, nil
}

func makeRootBlockQC(header *flow.Header, qc *model.QuorumCertificate) *forks.BlockQC {
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
