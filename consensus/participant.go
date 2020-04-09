package consensus

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/blockproducer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/eventhandler"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/forkchoice"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/model"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voter"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	consensusmetrics "github.com/dapperlabs/flow-go/module/metrics/consensus"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/storage"
)

const (
	startView  = 1
	prunedView = 0
	votedView  = 0
)

// TODO: this needs to be integrated with proper configuration and bootstrapping.

func CreateConsumer(log zerolog.Logger, metrics module.Metrics, guarantees storage.Guarantees, seals storage.Seals) hotstuff.Consumer {
	logConsumer := notifications.NewLogConsumer(log)
	metricsConsumer := consensusmetrics.NewMetricsConsumer(metrics, guarantees, seals)
	dis := pubsub.NewDistributor()
	dis.AddConsumer(logConsumer)
	dis.AddConsumer(metricsConsumer)
	return dis
}

func NewParticipant(log zerolog.Logger, notifier hotstuff.Consumer, metrics module.Metrics, state protocol.State, me module.Local, builder module.Builder, updater module.Finalizer, signer hotstuff.Signer, communicator hotstuff.Communicator, selector flow.IdentityFilter, rootHeader *flow.Header, rootQC *model.QuorumCertificate) (*hotstuff.EventLoop, error) {

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

	// initialize the pacemaker
	pacemaker, err := pacemaker.New(startView, timeout.DefaultController(), notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize flow pacemaker: %w", err)
	}

	// initialize block producer
	producer, err := blockproducer.New(signer, viewState, builder)
	if err != nil {
		return nil, fmt.Errorf("could not initialize block producer: %w", err)
	}

	// initialize the forks manager
	forks := forks.New(finalizer, forkchoice)

	// initialize the voter
	// TODO: load last voted view
	voter := voter.New(signer, forks, votedView)

	// initialize the validator
	validator := validator.New(viewState, forks, signer)

	// initialize the vote aggregator
	aggregator := voteaggregator.New(notifier, prunedView, viewState, validator, signer)

	// initialize the event handler
	handler, err := eventhandler.New(log, pacemaker, producer, forks, communicator, viewState, aggregator, voter, validator, notifier)
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
