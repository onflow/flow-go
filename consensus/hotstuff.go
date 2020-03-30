package hotstuff

import (
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/blockproducer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/eventhandler"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/examples/notifications"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks"
	forkfin "github.com/dapperlabs/flow-go/consensus/hotstuff/forks/finalizer"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/forks/forkchoice"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/pacemaker/timeout"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/validator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/viewstate"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voteaggregator"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/voter"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/state/dkg"
	"github.com/dapperlabs/flow-go/state/protocol"
)

const (
	startView                      = 1
	startReplicaTimeout            = 1200 * time.Millisecond
	minReplicaTimeout              = 1200 * time.Millisecond
	voteAggregationTimeoutFraction = 0.5
	timeoutIncrease                = 1.5
	timeoutDecrease                = 800
	highestPrunedView              = 0
	lastVotedView                  = 0
)

func NewHotstuff(log zerolog.Logger, state protocol.State, me module.Local, builder module.Builder, finalizer module.Finalizer, nodeSet flow.IdentityFilter, communicator hotstuff.Communicator) (*hotstuff.EventLoop, error) {

	// initialize notification consumer
	// TODO: implement a logging notifications consumer
	notifier := notifications.NewPubSubDistributor()

	// initialize view state
	// TODO: inject real DKG public data
	var dkgState dkg.State
	viewState, err := viewstate.New(state, dkgState, me.NodeID(), nodeSet)
	if err != nil {
		return nil, fmt.Errorf("could not initialize view state: %w", err)
	}

	// initialize finalizer
	var trustedRoot *forks.BlockQC
	forkfinalizer, err := forkfin.New(trustedRoot, finalizer, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize finalizer: %w", err)
	}

	// initialize the fork choice
	forkchoice, err := forkchoice.NewNewestForkChoice(forkfinalizer, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize fork choice: %w", err)
	}

	// initialize timeout config
	// TODO: move into consistent config approach for all components
	timeoutConfig, err := timeout.NewConfig(
		startReplicaTimeout,
		minReplicaTimeout,
		voteAggregationTimeoutFraction,
		timeoutIncrease,
		timeoutDecrease,
	)
	if err != nil {
		return nil, fmt.Errorf("could not initialize timeout configuration: %w", err)
	}

	// initialize timeout controller
	timeout := timeout.NewController(timeoutConfig)

	// initialize the pacemaker
	pacemaker, err := pacemaker.New(startView, timeout, notifier)
	if err != nil {
		return nil, fmt.Errorf("could not initialize flow pacemaker: %w", err)
	}

	// TODO: initialize signer
	var signer hotstuff.Signer

	// initialize block producer
	producer, err := blockproducer.New(signer, viewState, builder)
	if err != nil {
		return nil, fmt.Errorf("could not initialize block producer: %w", err)
	}

	// initialize the forks manager
	forks := forks.New(forkfinalizer, forkchoice)

	// initialize the voter
	// TODO: load last voted view
	voter := voter.New(signer, forks, lastVotedView)

	// initialize the validator
	validator := validator.New(viewState, forks, signer)

	// initialize the vote aggregator
	aggregator := voteaggregator.New(notifier, highestPrunedView, viewState, validator, signer)

	// initialize the event handler
	handler, err := eventhandler.New(log, pacemaker, producer, forks, communicator, viewState, aggregator, voter, validator)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event handler: %w", err)
	}

	// initialize and return the event loop
	loop, err := hotstuff.NewEventLoop(log, handler)
	if err != nil {
		return nil, fmt.Errorf("could not initialize event loop: %w", err)
	}

	return loop, nil
}
