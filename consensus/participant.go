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
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voter"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

func NewParticipant(log zerolog.Logger, notifier hotstuff.Consumer, metrics module.Metrics, headers storage.Headers,
	views storage.Views, state protocol.State, me module.Local, builder module.Builder, updater module.Finalizer,
	signer hotstuff.Signer, communicator hotstuff.Communicator, selector flow.IdentityFilter, rootHeader *flow.Header,
	rootQC *model.QuorumCertificate, options ...Option) (*hotstuff.EventLoop, error) {

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

	// initialize view state
	viewState, err := viewstate.New(state, me.NodeID(), selector)
	if err != nil {
		return nil, fmt.Errorf("could not initialize view state: %w", err)
	}

	// initialize internal finalizer
	rootBlock := &model.Block{
		View:        rootHeader.View,
		BlockID:     rootHeader.ID(),
		ProposerID:  rootHeader.ProposerID,
		QC:          nil,
		PayloadHash: rootHeader.PayloadHash,
		Timestamp:   rootHeader.Timestamp,
	}
	trustedRoot := &forks.BlockQC{
		QC:    rootQC,
		Block: rootBlock,
	}
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

	// initialize the validator
	validator := validator.New(viewState, forks, signer)

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

	// recover the hotstuff state
	err = Recover(log, forks, validator, headers, state)
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
	producer, err := blockproducer.New(signer, viewState, builder)
	if err != nil {
		return nil, fmt.Errorf("could not initialize block producer: %w", err)
	}

	// initialize the voter
	voter := voter.New(signer, forks, persist, lastVoted)

	// initialize the vote aggregator
	aggregator := voteaggregator.New(notifier, 0, viewState, validator, signer)

	// initialize the event handler
	handler, err := eventhandler.New(log, pacemaker, producer, forks, persist, communicator, viewState, aggregator, voter, validator, notifier)
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
