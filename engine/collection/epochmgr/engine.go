package epochmgr

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/collection"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/epochs"
	"github.com/onflow/flow-go/module/irrecoverable"
	epochpool "github.com/onflow/flow-go/module/mempool/epochs"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

// DefaultStartupTimeout is the default time we wait when starting epoch components before giving up.
const DefaultStartupTimeout = time.Minute

// ErrNotAuthorizedForEpoch is returned when we attempt to create epoch components
// for an epoch in which we are not an authorized network participant. This is the
// case for epochs during which this node is joining or leaving the network.
var ErrNotAuthorizedForEpoch = fmt.Errorf("we are not an authorized participant for the epoch")

// Engine is the epoch manager, which coordinates the lifecycle of other modules
// and processes that are epoch-dependent. The manager is responsible for
// spinning up engines when a new epoch is about to start and spinning down
// engines for an epoch that has ended.
//
// The `epochmgr.Engine` implements the `protocol.Consumer` interface. In particular, it
// ingests the following notifications from the protocol state:
//   - EpochSetupPhaseStarted
//   - EpochTransition
//
// As part of the engine starting, it executes pending actions that should have been triggered
// by protocol events but those events were missed during a crash/restart. See respective
// consumer methods for further details.
type Engine struct {
	events.Noop // satisfy protocol events consumer interface

	log            zerolog.Logger
	me             module.Local
	state          protocol.State
	pools          *epochpool.TransactionPools // epoch-scoped transaction pools
	factory        EpochComponentsFactory      // consolidates creating epoch for an epoch
	voter          module.ClusterRootQCVoter   // manages process of voting for next epoch's QC
	heightEvents   events.Heights              // allows subscribing to particular heights
	startupTimeout time.Duration               // how long we wait for epoch components to start up

	mu     sync.RWMutex                       // protects epochs map
	epochs map[uint64]*RunningEpochComponents // epoch-scoped components per epoch

	// internal event notifications
	epochTransitionEvents        chan *flow.Header        // sends first block of new epoch
	epochSetupPhaseStartedEvents chan *flow.Header        // sends first block of EpochSetup phase
	epochStopEvents              chan uint64              // sends counter of epoch to stop
	clusterIDUpdateDistributor   collection.ClusterEvents // sends cluster ID updates to consumers
	cm                           *component.ComponentManager
	component.Component
}

var _ component.Component = (*Engine)(nil)
var _ protocol.Consumer = (*Engine)(nil)

func New(
	log zerolog.Logger,
	me module.Local,
	state protocol.State,
	pools *epochpool.TransactionPools,
	voter module.ClusterRootQCVoter,
	factory EpochComponentsFactory,
	heightEvents events.Heights,
	clusterIDUpdateDistributor collection.ClusterEvents,
) (*Engine, error) {
	e := &Engine{
		log:                          log.With().Str("engine", "epochmgr").Logger(),
		me:                           me,
		state:                        state,
		pools:                        pools,
		voter:                        voter,
		factory:                      factory,
		heightEvents:                 heightEvents,
		epochs:                       make(map[uint64]*RunningEpochComponents),
		startupTimeout:               DefaultStartupTimeout,
		epochTransitionEvents:        make(chan *flow.Header, 1),
		epochSetupPhaseStartedEvents: make(chan *flow.Header, 1),
		epochStopEvents:              make(chan uint64, 1),
		clusterIDUpdateDistributor:   clusterIDUpdateDistributor,
	}

	e.cm = component.NewComponentManagerBuilder().
		AddWorker(e.handleEpochEvents).
		Build()
	e.Component = e.cm

	return e, nil
}

// Start starts the engine.
func (e *Engine) Start(ctx irrecoverable.SignalerContext) {
	// (1) start engine-scoped workers
	e.cm.Start(ctx)

	// (2) Retrieve protocol state as of latest finalized block. We use this state
	// to catch up on events, whose execution was missed during crash-restart.
	finalSnapshot := e.state.Final()

	// (3) check if we should attempt to vote after startup
	err := e.checkShouldVoteOnStartup(finalSnapshot)
	if err != nil {
		ctx.Throw(fmt.Errorf("could not vote on startup: %w", err))
	}

	// (4) start epoch-scoped components:
	// (a) set up epoch-scoped epoch managed by this engine for the current epoch
	err = e.checkShouldStartCurrentEpochComponentsOnStartup(ctx, finalSnapshot)
	if err != nil {
		ctx.Throw(fmt.Errorf("could not check or start current epoch components: %w", err))
	}

	// (b) set up epoch-scoped epoch components for the previous epoch
	err = e.checkShouldStartPreviousEpochComponentsOnStartup(ctx, finalSnapshot)
	if err != nil {
		ctx.Throw(fmt.Errorf("could not check or start previous epoch components: %w", err))
	}
}

// checkShouldStartCurrentEpochComponentsOnStartup checks whether we should instantiate
// consensus components for the current epoch upon startup, and if so, starts them.
// We always start current epoch consensus components, unless this node is not an
// authorized participant in the current epoch.
// No errors are expected during normal operation.
func (e *Engine) checkShouldStartCurrentEpochComponentsOnStartup(ctx irrecoverable.SignalerContext, finalSnapshot protocol.Snapshot) error {
	currentEpoch := finalSnapshot.Epochs().Current()
	currentEpochCounter, err := currentEpoch.Counter()
	if err != nil {
		return fmt.Errorf("could not get epoch counter: %w", err)
	}

	components, err := e.createEpochComponents(currentEpoch)
	if err != nil {
		if errors.Is(err, ErrNotAuthorizedForEpoch) {
			// don't set up consensus components if we aren't authorized in current epoch
			e.log.Info().Msg("node is not authorized for current epoch - skipping initializing cluster consensus")
			return nil
		}
		return fmt.Errorf("could not create epoch components: %w", err)
	}
	err = e.startEpochComponents(ctx, currentEpochCounter, components)
	if err != nil {
		// all failures to start epoch components are critical
		return fmt.Errorf("could not start epoch components: %w", err)
	}
	return nil
}

// checkShouldStartPreviousEpochComponentsOnStartup checks whether we should re-instantiate
// consensus components for the previous epoch upon startup, and if so, starts them.
// One cluster is responsible for a portion of transactions with reference blocks
// with one epoch. Since transactions may use reference blocks up to flow.DefaultTransactionExpiry
// many heights old, clusters don't shut down until this many blocks have been finalized
// past the final block of the cluster's epoch.
// No errors are expected during normal operation.
func (e *Engine) checkShouldStartPreviousEpochComponentsOnStartup(engineCtx irrecoverable.SignalerContext, finalSnapshot protocol.Snapshot) error {
	finalHeader, err := finalSnapshot.Head()
	if err != nil {
		return fmt.Errorf("[unexpected] could not get finalized header: %w", err)
	}
	finalizedHeight := finalHeader.Height

	prevEpoch := finalSnapshot.Epochs().Previous()
	prevEpochCounter, err := prevEpoch.Counter()
	if err != nil {
		if errors.Is(err, protocol.ErrNoPreviousEpoch) {
			return nil
		}
		return fmt.Errorf("[unexpected] could not get previous epoch counter: %w", err)
	}
	prevEpochFinalHeight, err := prevEpoch.FinalHeight()
	if err != nil {
		// no expected errors because we are querying finalized snapshot
		return fmt.Errorf("[unexpected] could not get previous epoch final height: %w", err)
	}
	prevEpochClusterConsensusStopHeight := prevEpochFinalHeight + flow.DefaultTransactionExpiry + 1

	log := e.log.With().
		Uint64("finalized_height", finalizedHeight).
		Uint64("prev_epoch_counter", prevEpochCounter).
		Uint64("prev_epoch_final_height", prevEpochFinalHeight).
		Uint64("prev_epoch_cluster_stop_height", prevEpochClusterConsensusStopHeight).
		Logger()

	if finalizedHeight >= prevEpochClusterConsensusStopHeight {
		log.Debug().Msg("not re-starting previous epoch cluster consensus on startup - past stop height")
		return nil
	}

	components, err := e.createEpochComponents(prevEpoch)
	if err != nil {
		if errors.Is(err, ErrNotAuthorizedForEpoch) {
			// don't set up consensus components if we aren't authorized in previous epoch
			log.Info().Msg("node is not authorized for previous epoch - skipping re-initializing last epoch cluster consensus")
			return nil
		}
		return fmt.Errorf("[unexpected] could not create previous epoch components: %w", err)
	}
	err = e.startEpochComponents(engineCtx, prevEpochCounter, components)
	if err != nil {
		// all failures to start epoch components are critical
		return fmt.Errorf("[unexpected] could not epoch components: %w", err)
	}
	e.prepareToStopEpochComponents(prevEpochCounter, prevEpochFinalHeight)

	log.Info().Msgf("re-started last epoch cluster consensus - will stop at height %d", prevEpochClusterConsensusStopHeight)
	return nil
}

// checkShouldVoteOnStartup checks whether we should vote, and if so, sends a signal
// to the worker thread responsible for voting.
// No errors are expected during normal operation.
func (e *Engine) checkShouldVoteOnStartup(finalSnapshot protocol.Snapshot) error {
	// check the current phase on startup, in case we are in setup phase
	// and haven't yet voted for the next root QC
	phase, err := finalSnapshot.Phase()
	if err != nil {
		return fmt.Errorf("could not get epoch phase for finalized snapshot: %w", err)
	}
	if phase == flow.EpochPhaseSetup {
		header, err := finalSnapshot.Head()
		if err != nil {
			return fmt.Errorf("could not get header for finalized snapshot: %w", err)
		}
		e.epochSetupPhaseStartedEvents <- header
	}
	return nil
}

// Ready returns a ready channel that is closed once the engine has fully started.
// This is true when the engine-scoped worker threads have started, and all presently
// running epoch components (max 2) have started.
func (e *Engine) Ready() <-chan struct{} {
	e.mu.RLock()
	components := make([]module.ReadyDoneAware, 0, len(e.epochs)+1)
	components = append(components, e.cm)
	for _, epoch := range e.epochs {
		components = append(components, epoch)
	}
	e.mu.RUnlock()

	return util.AllReady(components...)
}

// Done returns a done channel that is closed once the engine has fully stopped.
// This is true when the engine-scoped worker threads have stopped, and all presently
// running epoch components (max 2) have stopped.
func (e *Engine) Done() <-chan struct{} {
	e.mu.RLock()
	components := make([]module.ReadyDoneAware, 0, len(e.epochs)+1)
	components = append(components, e.cm)
	for _, epoch := range e.epochs {
		components = append(components, epoch)
	}
	e.mu.RUnlock()

	return util.AllDone(components...)
}

// createEpochComponents instantiates and returns epoch-scoped components for
// the given epoch, using the configured factory.
// Error returns:
// - ErrNotAuthorizedForEpoch if this node is not authorized in the epoch.
func (e *Engine) createEpochComponents(epoch protocol.Epoch) (*EpochComponents, error) {
	counter, err := epoch.Counter()
	if err != nil {
		return nil, fmt.Errorf("could not get epoch counter: %w", err)
	}
	state, prop, sync, hot, voteAggregator, timeoutAggregator, messageHub, err := e.factory.Create(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not setup requirements for epoch (%d): %w", counter, err)
	}

	components := NewEpochComponents(state, prop, sync, hot, voteAggregator, timeoutAggregator, messageHub)
	return components, nil
}

// EpochTransition handles the epoch transition protocol event.
// NOTE: epochmgr.Engine will not restart trailing cluster consensus instances from previous epoch,
// therefore no need to handle dropped protocol events here (see issue below).
// TODO gracefully handle restarts in first 600 blocks of epoch https://github.com/dapperlabs/flow-go/issues/5659
func (e *Engine) EpochTransition(_ uint64, first *flow.Header) {
	e.epochTransitionEvents <- first
}

// EpochSetupPhaseStarted handles the epoch setup phase started protocol event.
// NOTE: Ready will check if we start up in the EpochSetup phase at initialization and trigger QC voting.
// This handles dropped protocol events and restarts interrupting QC voting.
func (e *Engine) EpochSetupPhaseStarted(_ uint64, first *flow.Header) {
	e.epochSetupPhaseStartedEvents <- first
}

// handleEpochEvents handles events relating to the epoch lifecycle:
//   - EpochTransition protocol event - we start epoch components for the starting epoch,
//     and schedule shutdown for the ending epoch
//   - EpochSetupPhaseStarted protocol event - we submit our node's vote for our cluster's
//     root block in the next epoch
//   - epochStopEvents - signalled when a previously scheduled shutdown height is reached.
//     We shut down components associated with the epoch.
func (e *Engine) handleEpochEvents(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
	ready()

	for {
		select {
		case <-ctx.Done():
			return
		case firstBlock := <-e.epochTransitionEvents:
			err := e.onEpochTransition(ctx, firstBlock)
			if err != nil {
				ctx.Throw(err)
			}
		case firstBlock := <-e.epochSetupPhaseStartedEvents:
			nextEpoch := e.state.AtBlockID(firstBlock.ID()).Epochs().Next()
			e.onEpochSetupPhaseStarted(ctx, nextEpoch)
		case epochCounter := <-e.epochStopEvents:
			err := e.stopEpochComponents(epochCounter)
			if err != nil {
				ctx.Throw(err)
			}
		}
	}
}

// handleEpochErrors checks for irrecoverable errors thrown from any components from
// some epoch, and handles them. Currently, handling them means simply throwing them
// to the engine-level signaller context, which should cause the node to crash.
// In the future, we could restart the failed epoch's components instead.
// Must be run as a goroutine.
func (e *Engine) handleEpochErrors(ctx irrecoverable.SignalerContext, errCh <-chan error) {
	select {
	case <-ctx.Done():
		return
	case err := <-errCh:
		if err != nil {
			ctx.Throw(err)
		}
	}
}

// onEpochTransition is called when we transition to a new epoch. It arranges
// to shut down the last epoch's components and starts up the new epoch's.
//
// No errors are expected during normal operation.
func (e *Engine) onEpochTransition(ctx irrecoverable.SignalerContext, first *flow.Header) error {
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
	_, exists := e.getEpochComponents(counter)
	if exists {
		log.Warn().Msg("epoch transition: components for new epoch already setup, exiting...")
		return nil
	}

	// register a callback to stop the just-ended epoch at the appropriate block height
	e.prepareToStopEpochComponents(counter-1, lastEpochMaxHeight)

	log.Info().Msg("epoch transition: creating components for new epoch...")

	// create components for new epoch
	components, err := e.createEpochComponents(epoch)
	if err != nil {
		if errors.Is(err, ErrNotAuthorizedForEpoch) {
			// if we are not authorized in this epoch, skip starting up cluster consensus
			log.Info().Msg("epoch transition: we are not authorized for new epoch, exiting...")
			return nil
		}
		return fmt.Errorf("could not create epoch components: %w", err)
	}

	// start up components
	err = e.startEpochComponents(ctx, counter, components)
	if err != nil {
		return fmt.Errorf("unexpected failure starting epoch components: %w", err)
	}

	log.Info().Msg("epoch transition: new epoch components started successfully")

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
func (e *Engine) prepareToStopEpochComponents(epochCounter, epochMaxHeight uint64) {
	stopAtHeight := epochMaxHeight + flow.DefaultTransactionExpiry + 1
	e.log.Info().
		Uint64("stopping_epoch_max_height", epochMaxHeight).
		Uint64("stopping_epoch_counter", epochCounter).
		Uint64("stop_at_height", stopAtHeight).
		Str("step", "epoch_transition").
		Msgf("preparing to stop epoch components at height %d", stopAtHeight)

	e.heightEvents.OnHeight(stopAtHeight, func() {
		e.epochStopEvents <- epochCounter
	})
}

// onEpochSetupPhaseStarted is called either when we transition into the epoch
// setup phase, or when the node is restarted during the epoch setup phase. It
// kicks off setup tasks for the phase, in particular submitting a vote for the
// next epoch's root cluster QC.
func (e *Engine) onEpochSetupPhaseStarted(ctx irrecoverable.SignalerContext, nextEpoch protocol.Epoch) {
	err := e.voter.Vote(ctx, nextEpoch)
	if err != nil {
		if epochs.IsClusterQCNoVoteError(err) {
			e.log.Warn().Err(err).Msg("unable to submit QC vote for next epoch")
			return
		}
		ctx.Throw(fmt.Errorf("unexpected failure to submit QC vote for next epoch: %w", err))
	}
}

// startEpochComponents starts the components for the given epoch and adds them
// to the engine's internal mapping.
// No errors are expected during normal operation.
func (e *Engine) startEpochComponents(engineCtx irrecoverable.SignalerContext, counter uint64, components *EpochComponents) error {
	epochCtx, cancel, errCh := irrecoverable.WithSignallerAndCancel(engineCtx)
	// start component using its own context
	components.Start(epochCtx)
	go e.handleEpochErrors(engineCtx, errCh)

	select {
	case <-components.Ready():
		e.storeEpochComponents(counter, NewRunningEpochComponents(components, cancel))
		activeClusterIDS, err := e.activeClusterIDs()
		if err != nil {
			return fmt.Errorf("failed to get active cluster IDs: %w", err)
		}
		e.clusterIDUpdateDistributor.ActiveClustersChanged(activeClusterIDS)
		return nil
	case <-time.After(e.startupTimeout):
		cancel() // cancel current context if we didn't start in time
		return fmt.Errorf("could not start epoch %d components after %s", counter, e.startupTimeout)
	}
}

// stopEpochComponents stops the components for the given epoch and removes them
// from the engine's internal mapping. If no components exit for the given epoch,
// this is a no-op and a warning is logged.
// No errors are expected during normal operation.
func (e *Engine) stopEpochComponents(counter uint64) error {
	components, exists := e.getEpochComponents(counter)
	if !exists {
		e.log.Warn().Msgf("attempted to stop non-existent epoch %d", counter)
		return nil
	}

	// stop individual component
	components.cancel()

	select {
	case <-components.Done():
		e.removeEpoch(counter)
		e.pools.ForEpoch(counter).Clear()
		activeClusterIDS, err := e.activeClusterIDs()
		if err != nil {
			return fmt.Errorf("failed to get active cluster IDs: %w", err)
		}
		e.clusterIDUpdateDistributor.ActiveClustersChanged(activeClusterIDS)
		return nil
	case <-time.After(e.startupTimeout):
		return fmt.Errorf("could not stop epoch %d components after %s", counter, e.startupTimeout)
	}
}

// getEpochComponents retrieves the stored (running) epoch components for the given epoch counter.
// If no epoch with the counter is stored, returns (nil, false).
// Safe for concurrent use.
func (e *Engine) getEpochComponents(counter uint64) (*RunningEpochComponents, bool) {
	e.mu.RLock()
	epoch, ok := e.epochs[counter]
	e.mu.RUnlock()
	return epoch, ok
}

// storeEpochComponents stores the given epoch components in the engine's mapping.
// Safe for concurrent use.
func (e *Engine) storeEpochComponents(counter uint64, components *RunningEpochComponents) {
	e.mu.Lock()
	e.epochs[counter] = components
	e.mu.Unlock()
}

// removeEpoch removes the epoch components with the given counter.
// Safe for concurrent use.
func (e *Engine) removeEpoch(counter uint64) {
	e.mu.Lock()
	delete(e.epochs, counter)
	e.mu.Unlock()
}

// activeClusterIDs returns the active canonical cluster ID's for the assigned collection clusters.
// No errors are expected during normal operation.
func (e *Engine) activeClusterIDs() (flow.ChainIDList, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	clusterIDs := make(flow.ChainIDList, 0)
	for _, epoch := range e.epochs {
		chainID := epoch.state.Params().ChainID() // cached, does not hit database
		clusterIDs = append(clusterIDs, chainID)
	}
	return clusterIDs, nil
}
