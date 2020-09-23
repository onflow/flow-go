package epochmgr

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/lifecycle"
	"github.com/dapperlabs/flow-go/state/cluster"
	"github.com/dapperlabs/flow-go/state/protocol"
	"github.com/dapperlabs/flow-go/state/protocol/events"
)

// EpochComponents represents all dependencies for running an epoch.
type EpochComponents struct {
	state    cluster.State
	prop     module.Engine
	sync     module.Engine
	hotstuff module.HotStuff
	// TODO: ingest/txpool should also be epoch-dependent, possibly managed by this engine
}

// Ready starts all epoch components.
func (ec *EpochComponents) Ready() <-chan struct{} {
	return lifecycle.AllReady(ec.prop, ec.sync, ec.hotstuff)
}

// Done stops all epoch components.
func (ec *EpochComponents) Done() <-chan struct{} {
	return lifecycle.AllDone(ec.prop, ec.sync, ec.hotstuff)
}

// Engine is the epoch manager, which coordinates the lifecycle of other modules
// and processes that are epoch-dependent. The manager is responsible for
// spinning up engines when a new epoch is about to start and spinning down
// engines for an epoch that has ended.
type Engine struct {
	unit *engine.Unit

	log     zerolog.Logger
	me      module.Local
	state   protocol.State
	factory EpochComponentsFactory    // consolidates creating epoch for an epoch
	voter   module.ClusterRootQCVoter // manages process of voting for next epoch's QC

	epochs map[uint64]*EpochComponents // epoch-scoped components per epoch

	events.Noop // satisfy protocol events consumer interface
}

func New(
	log zerolog.Logger,
	me module.Local,
	state protocol.State,
	voter module.ClusterRootQCVoter,
	factory EpochComponentsFactory,
) (*Engine, error) {

	e := &Engine{
		unit:    engine.NewUnit(),
		log:     log,
		me:      me,
		state:   state,
		voter:   voter,
		factory: factory,
		epochs:  make(map[uint64]*EpochComponents),
	}

	// set up epoch-scoped epoch managed by this engine for the current epoch
	epoch := e.state.Final().Epochs().Current()
	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch counter: %w", err)
	}
	components, err := e.createEpochComponents(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not create epoch components for current epoch: %w", err)
	}

	e.epochs[counter] = components

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For proposal engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready(func() {
		// Start up components for all epochs. This is typically a single epoch
		// but can be multiple near epoch boundaries
		epochs := make([]module.ReadyDoneAware, 0, len(e.epochs))
		for _, epoch := range e.epochs {
			epochs = append(epochs, epoch)
		}
		<-lifecycle.AllReady(epochs...)
	}, func() {
		// check the current phase on startup, in case we are in setup phase
		// and haven't yet voted for the next root QC
		phase, err := e.state.Final().Phase()
		if err != nil {
			e.log.Error().Err(err).Msg("could not check phase")
			return
		}
		if phase == flow.EpochPhaseSetup {
			e.unit.Launch(e.onEpochSetupPhaseStarted)
		}
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		// Stop components for all epochs. This is typically a single epoch
		// but can be multiple near epoch boundaries
		epochs := make([]module.ReadyDoneAware, 0, len(e.epochs))
		for _, epoch := range e.epochs {
			epochs = append(epochs, epoch)
		}
		<-lifecycle.AllDone(epochs...)
	})
}

func (e *Engine) createEpochComponents(epoch protocol.Epoch) (*EpochComponents, error) {

	state, prop, sync, hot, err := e.factory.Create(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not setup requirements for epoch (%d): %w", epoch, err)
	}

	components := &EpochComponents{
		state:    state,
		prop:     prop,
		sync:     sync,
		hotstuff: hot,
	}
	return components, err
}

// EpochTransition handles the epoch transition protocol event.
func (e *Engine) EpochTransition(_ uint64, first *flow.Header) {

	epoch := e.state.Final().Epochs().Current()
	_ = epoch

	// start components for this epoch
	// set up trigger to shut down previous epoch components after max expiry
}

// EpochSetupPhaseStarted handles the epoch setup phase started protocol event.
func (e *Engine) EpochSetupPhaseStarted(_ uint64, _ *flow.Header) {
	e.unit.Launch(e.onEpochSetupPhaseStarted)
}

// onEpochSetupPhaseStarted is called either when we transition into the epoch
// setup phase, or when the node is restarted during the epoch setup phase. It
// kicks off setup tasks for the phase, in particular submitting a vote for the
// next epoch's root cluster QC.
func (e *Engine) onEpochSetupPhaseStarted() {

	epoch := e.state.Final().Epochs().Next()

	ctx, cancel := context.WithCancel(e.unit.Ctx())
	defer cancel()
	err := e.voter.Vote(ctx, epoch)
	if err != nil {
		e.log.Error().Err(err).Msg("failed to submit QC vote for next epoch")
	}
}
