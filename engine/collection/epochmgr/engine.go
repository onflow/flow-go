package epochmgr

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/lifecycle"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

const DefaultStartupTimeout = 30 * time.Second

// EpochComponents represents all dependencies for running an epoch.
type EpochComponents struct {
	state    cluster.State
	prop     module.Engine
	sync     module.Engine
	hotstuff module.HotStuff
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
	events.Noop // satisfy protocol events consumer interface

	unit         *engine.Unit
	log          zerolog.Logger
	me           module.Local
	state        protocol.State
	pools        *epochs.TransactionPools  // epoch-scoped transaction pools
	factory      EpochComponentsFactory    // consolidates creating epoch for an epoch
	voter        module.ClusterRootQCVoter // manages process of voting for next epoch's QC
	heightEvents events.Heights            // allows subscribing to particular heights

	epochs         map[uint64]*EpochComponents // epoch-scoped components per epoch
	startupTimeout time.Duration               // how long we wait for epoch components to start up
}

func New(
	log zerolog.Logger,
	me module.Local,
	state protocol.State,
	pools *epochs.TransactionPools,
	voter module.ClusterRootQCVoter,
	factory EpochComponentsFactory,
	heightEvents events.Heights,
) (*Engine, error) {

	e := &Engine{
		unit:           engine.NewUnit(),
		log:            log,
		me:             me,
		state:          state,
		pools:          pools,
		voter:          voter,
		factory:        factory,
		heightEvents:   heightEvents,
		epochs:         make(map[uint64]*EpochComponents),
		startupTimeout: DefaultStartupTimeout,
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
	e.unit.Launch(func() {
		err := e.onEpochTransition(first)
		if err != nil {
			// failing to complete epoch transition is a fatal error
			e.log.Fatal().Err(err).Msg("failed to complete epoch transition")
		}
	})
}

// EpochSetupPhaseStarted handles the epoch setup phase started protocol event.
func (e *Engine) EpochSetupPhaseStarted(_ uint64, _ *flow.Header) {
	e.unit.Launch(e.onEpochSetupPhaseStarted)
}

// onEpochTransition is called when we transition to a new epoch. It arranges
// to shut down the last epoch's components and starts up the new epoch's.
func (e *Engine) onEpochTransition(first *flow.Header) error {
	e.unit.Lock()
	defer e.unit.Unlock()

	epoch := e.state.Final().Epochs().Current()
	counter, err := epoch.Counter()
	if err != nil {
		return fmt.Errorf("could not get epoch counter: %w", err)
	}

	log := e.log.With().
		Uint64("epoch_height", first.Height).
		Uint64("epoch_counter", counter).
		Logger()

	// exit early and log if the epoch already exists
	_, exists := e.epochs[counter]
	if exists {
		log.Warn().Msg("epoch transition: components for new epoch already setup")
		return nil
	}

	log.Info().Msg("epoch transition: creating components for new epoch...")

	// create components for new epoch
	components, err := e.createEpochComponents(epoch)
	if err != nil {
		return fmt.Errorf("could not create epoch components: %w", err)
	}

	// start up components
	err = e.startEpochComponents(counter, components)
	if err != nil {
		return fmt.Errorf("could not start epoch components: %w", err)
	}

	log.Info().Msg("epoch transition: new epoch components started successfully")

	// Finally we set up a trigger to shut down the components of the previous
	// epoch once we can no longer produce valid collections there. Transactions
	// referencing blocks from the previous epoch are only valid for inclusion
	// in collections built by clusters from that epoch. Consequently, it remains
	// possible for the previous epoch's cluster to produce valid collections
	// until all such transactions have expired. In fact, since these transactions
	// can NOT be included by clusters in the new epoch, we MUST continue producing
	// these collections within the previous epoch's clusters.
	e.heightEvents.OnHeight(first.Height+flow.DefaultTransactionExpiry, func() {
		e.unit.Launch(func() {
			e.unit.Lock()
			defer e.unit.Unlock()

			log.Info().Msg("epoch transition: stopping components for previous epoch...")

			err := e.stopEpochComponents(counter - 1)
			if err != nil {
				e.log.Error().Err(err).Msgf("epoch transition: failed to stop components for epoch %d", counter-1)
				return
			}

			log.Info().Msg("epoch transition: previous epoch components stopped successfully")
		})
	})

	return nil
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

// startEpochComponents starts the components for the given epoch and adds them
// to the engine's internal mapping.
//
// CAUTION: the caller MUST acquire the engine lock.
func (e *Engine) startEpochComponents(counter uint64, components *EpochComponents) error {

	select {
	case <-components.Ready():
		e.epochs[counter] = components
		return nil
	case <-time.After(e.startupTimeout):
		return fmt.Errorf("could not start epoch %d components after %s", counter, e.startupTimeout)
	}
}

// stopEpochComponents stops the components for the given epoch and removes them
// from the engine's internal mapping.
//
// CAUTION: the caller MUST acquire the engine lock.
func (e *Engine) stopEpochComponents(counter uint64) error {

	components, exists := e.epochs[counter]
	if !exists {
		return fmt.Errorf("can not stop non-existent epoch %d", counter)
	}

	select {
	case <-components.Done():
		delete(e.epochs, counter)
		e.pools.ForEpoch(counter).Clear()
		return nil
	case <-time.After(e.startupTimeout):
		return fmt.Errorf("could not stop epoch %d components after %s", counter, e.startupTimeout)
	}
}
