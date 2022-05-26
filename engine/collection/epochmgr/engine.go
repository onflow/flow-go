package epochmgr

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/state/cluster"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

// DefaultStartupTimeout is the default time we wait when starting epoch
// components before giving up.
const DefaultStartupTimeout = 30 * time.Second

// ErrNotAuthorizedForEpoch is returned when we attempt to create epoch components
// for an epoch in which we are not an authorized network participant. This is the
// case for epochs during which this node is joining or leaving the network.
var ErrNotAuthorizedForEpoch = fmt.Errorf("we are not an authorized participant for the epoch")

// EpochComponents represents all dependencies for running an epoch.
type EpochComponents struct {
	*component.ComponentManager
	state      cluster.State
	prop       network.Engine
	sync       network.Engine
	hotstuff   module.HotStuff
	aggregator hotstuff.VoteAggregator
}

var _ component.Component = (*EpochComponents)(nil)

func NewEpochComponents(
	state cluster.State,
	prop network.Engine,
	sync network.Engine,
	hotstuff module.HotStuff,
	aggregator hotstuff.VoteAggregator,
) *EpochComponents {
	components := &EpochComponents{
		state:      state,
		prop:       prop,
		sync:       sync,
		hotstuff:   hotstuff,
		aggregator: aggregator,
	}

	builder := component.NewComponentManagerBuilder()
	// start new worker that will start child components and wait for them to finish
	builder.AddWorker(func(parentCtx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		// create a separate context that is not connected to parent, reason:
		// we want to stop vote aggregator after event loop and compliance engine have shutdown
		ctx, cancel := context.WithCancel(context.Background())
		signalerCtx, _ := irrecoverable.WithSignaler(ctx)
		// start aggregator, hotstuff will be started by compliance engine
		aggregator.Start(signalerCtx)
		// wait until all components start
		<-util.AllReady(components.prop, components.sync, components.aggregator)
		// signal that startup has finished and we are ready to go
		ready()
		// wait for shutdown to be commenced
		<-parentCtx.Done()
		// wait for compliance engine and event loop to shut down
		<-util.AllDone(components.prop, components.sync)
		// after event loop and engines were stopped proceed with stopping vote aggregator
		cancel()
		// wait until it stops
		<-components.aggregator.Done()
	})
	components.ComponentManager = builder.Build()

	return components
}

type StartableEpochComponents struct {
	*EpochComponents
	signalerCtx irrecoverable.SignalerContext // used to start the component
	cancel      context.CancelFunc            // used to stop the epoch components
}

func NewStartableEpochComponents(components *EpochComponents, signalerCtx irrecoverable.SignalerContext, cancel context.CancelFunc) *StartableEpochComponents {
	return &StartableEpochComponents{
		EpochComponents: components,
		signalerCtx:     signalerCtx,
		cancel:          cancel,
	}
}

// Engine is the epoch manager, which coordinates the lifecycle of other modules
// and processes that are epoch-dependent. The manager is responsible for
// spinning up engines when a new epoch is about to start and spinning down
// engines for an epoch that has ended.
type Engine struct {
	events.Noop // satisfy protocol events consumer interface

	unit             *engine.Unit
	log              zerolog.Logger
	me               module.Local
	state            protocol.State
	pools            *epochs.TransactionPools      // epoch-scoped transaction pools
	factory          EpochComponentsFactory        // consolidates creating epoch for an epoch
	voter            module.ClusterRootQCVoter     // manages process of voting for next epoch's QC
	heightEvents     events.Heights                // allows subscribing to particular heights
	irrecoverableCtx irrecoverable.SignalerContext // parent context for canceling all started epochs
	stopComponents   context.CancelFunc            // used to stop all components

	epochs         map[uint64]*StartableEpochComponents // epoch-scoped components per epoch
	startupTimeout time.Duration                        // how long we wait for epoch components to start up
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
	ctx, stopComponents := context.WithCancel(context.Background())
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	e := &Engine{
		unit:             engine.NewUnit(),
		log:              log.With().Str("engine", "epochmgr").Logger(),
		me:               me,
		state:            state,
		pools:            pools,
		voter:            voter,
		factory:          factory,
		heightEvents:     heightEvents,
		epochs:           make(map[uint64]*StartableEpochComponents),
		startupTimeout:   DefaultStartupTimeout,
		irrecoverableCtx: signalerCtx,
		stopComponents:   stopComponents,
	}

	// set up epoch-scoped epoch managed by this engine for the current epoch
	epoch := e.state.Final().Epochs().Current()
	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch counter: %w", err)
	}

	components, err := e.createEpochComponents(epoch)
	// don't set up consensus components if we aren't authorized in current epoch
	if errors.Is(err, ErrNotAuthorizedForEpoch) {
		return e, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not create epoch components for current epoch: %w", err)
	}

	ctx, cancel := context.WithCancel(e.irrecoverableCtx)
	signalerCtx, _ = irrecoverable.WithSignaler(ctx)

	e.epochs[counter] = NewStartableEpochComponents(components, signalerCtx, cancel)

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
			epoch.Start(epoch.signalerCtx) // start every component using its own context
		}
		// wait for all engines to start
		<-util.AllReady(epochs...)
	}, func() {
		// check the current phase on startup, in case we are in setup phase
		// and haven't yet voted for the next root QC
		finalSnapshot := e.state.Final()
		phase, err := finalSnapshot.Phase()
		if err != nil {
			e.log.Fatal().Err(err).Msg("could not check phase")
			return
		}
		if phase == flow.EpochPhaseSetup {
			e.unit.Launch(func() {
				e.onEpochSetupPhaseStarted(finalSnapshot.Epochs().Next())
			})
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
		e.stopComponents() // stop all components using parent context
		<-util.AllDone(epochs...)
	})
}

// createEpochComponents instantiates and returns epoch-scoped components for
// the given epoch, using the configured factory.
//
// Returns ErrNotAuthorizedForEpoch if this node is not authorized in the epoch.
func (e *Engine) createEpochComponents(epoch protocol.Epoch) (*EpochComponents, error) {

	state, prop, sync, hot, aggregator, err := e.factory.Create(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not setup requirements for epoch (%d): %w", epoch, err)
	}

	components := NewEpochComponents(state, prop, sync, hot, aggregator)
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
func (e *Engine) EpochSetupPhaseStarted(_ uint64, first *flow.Header) {
	e.unit.Launch(func() {
		nextEpoch := e.state.AtBlockID(first.ID()).Epochs().Next()
		e.onEpochSetupPhaseStarted(nextEpoch)
	})
}

// onEpochTransition is called when we transition to a new epoch. It arranges
// to shut down the last epoch's components and starts up the new epoch's.
func (e *Engine) onEpochTransition(first *flow.Header) error {
	e.unit.Lock()
	defer e.unit.Unlock()

	epoch := e.state.AtBlockID(first.ID()).Epochs().Current()
	counter, err := epoch.Counter()
	if err != nil {
		return fmt.Errorf("could not get epoch counter: %w", err)
	}

	// greatest block height in the previous epoch is one less than the first
	// block in current epoch
	lastEpochMaxHeight := first.Height - 1

	log := e.log.With().
		Uint64("last_epoch_max_height", lastEpochMaxHeight).
		Uint64("cur_epoch_counter", counter).
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
	// if we are not authorized in this epoch, skip starting up cluster consensus
	if errors.Is(err, ErrNotAuthorizedForEpoch) {
		e.prepareToStopEpochComponents(counter-1, lastEpochMaxHeight)
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not create epoch components: %w", err)
	}

	// start up components
	err = e.startEpochComponents(counter, components)
	if err != nil {
		return fmt.Errorf("could not start epoch components: %w", err)
	}

	log.Info().Msg("epoch transition: new epoch components started successfully")

	// set up callback to stop previous epoch
	e.prepareToStopEpochComponents(counter-1, lastEpochMaxHeight)

	return nil
}

// prepareToStopEpochComponents registers a callback to stop the epoch with the
// given counter once it is no longer possible to receive transactions from that
// epoch. This occurs when we finalize sufficiently many blocks in the new epoch
// that a transaction referencing any block from the previous epoch would be
// considered immediately expired.
//
// Transactions referencing blocks from the previous epoch are only valid for
// inclusion in collections built by clusters from that epoch. Consequently, it
// remains possible for the previous epoch's cluster to produce valid collections
// until all such transactions have expired. In fact, since these transactions
// can NOT be included by clusters in the new epoch, we MUST continue producing
// these collections within the previous epoch's clusters.
//
func (e *Engine) prepareToStopEpochComponents(epochCounter, epochMaxHeight uint64) {

	stopAtHeight := epochMaxHeight + flow.DefaultTransactionExpiry + 1

	log := e.log.With().
		Uint64("stopping_epoch_max_height", epochMaxHeight).
		Uint64("stopping_epoch_counter", epochCounter).
		Uint64("stop_at_height", stopAtHeight).
		Str("step", "epoch_transition").
		Logger()

	log.Info().Msgf("preparing to stop epoch components at height %d", stopAtHeight)

	e.heightEvents.OnHeight(stopAtHeight, func() {
		e.unit.Launch(func() {
			e.unit.Lock()
			defer e.unit.Unlock()

			log.Info().Msg("stopping components for previous epoch...")

			err := e.stopEpochComponents(epochCounter)
			if err != nil {
				e.log.Error().Err(err).Msgf("failed to stop components for epoch %d", epochCounter)
				return
			}

			log.Info().Msg("previous epoch components stopped successfully")
		})
	})
}

// onEpochSetupPhaseStarted is called either when we transition into the epoch
// setup phase, or when the node is restarted during the epoch setup phase. It
// kicks off setup tasks for the phase, in particular submitting a vote for the
// next epoch's root cluster QC.
func (e *Engine) onEpochSetupPhaseStarted(nextEpoch protocol.Epoch) {

	ctx, cancel := context.WithCancel(e.unit.Ctx())
	defer cancel()
	err := e.voter.Vote(ctx, nextEpoch)
	if err != nil {
		e.log.Error().Err(err).Msg("failed to submit QC vote for next epoch")
	}
}

// startEpochComponents starts the components for the given epoch and adds them
// to the engine's internal mapping.
//
// CAUTION: the caller MUST acquire the engine lock.
func (e *Engine) startEpochComponents(counter uint64, components *EpochComponents) error {

	ctx, cancel := context.WithCancel(e.irrecoverableCtx)
	signalerCtx, _ := irrecoverable.WithSignaler(ctx)

	// start component using its own context
	components.Start(signalerCtx)

	select {
	case <-components.Ready():
		e.epochs[counter] = NewStartableEpochComponents(components, signalerCtx, cancel)
		return nil
	case <-time.After(e.startupTimeout):
		cancel() // cancel current context if we didn't start in time
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

	// stop individual component
	components.cancel()

	select {
	case <-components.Done():
		delete(e.epochs, counter)
		e.pools.ForEpoch(counter).Clear()
		return nil
	case <-time.After(e.startupTimeout):
		return fmt.Errorf("could not stop epoch %d components after %s", counter, e.startupTimeout)
	}
}
