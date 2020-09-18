package epochmgr

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/dapperlabs/flow-go/engine"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/module/epochs"
	"github.com/dapperlabs/flow-go/module/mempool"
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

// Engine is the epoch manager, which coordinates the lifecycle of other modules
// and processes that are epoch-dependent. The manager is responsible for
// spinning up engines when a new epoch is about to start and spinning down
// engines for an epoch that has ended.
type Engine struct {
	unit  *engine.Unit
	epoch *EpochComponents          // requirements for the current epoch
	voter module.ClusterRootQCVoter // manages process of voting for next epoch's QC

	log     zerolog.Logger
	me      module.Local
	state   protocol.State
	factory EpochComponentsFactory // consolidates creating components for an epoch

	events.Noop // satisfy protocol events consumer interface

	// TODO should be per-epoch eventually, cache here for now
	pool mempool.Transactions
}

func New(
	log zerolog.Logger,
	me module.Local,
	state protocol.State,
	pool mempool.Transactions,
	voter *epochs.RootQCVoter,
	factory EpochComponentsFactory,
) (*Engine, error) {

	e := &Engine{
		unit:    engine.NewUnit(),
		log:     log,
		me:      me,
		state:   state,
		pool:    pool,
		voter:   voter,
		factory: factory,
	}

	// set up epoch-scoped components managed by this engine for the current epoch
	epoch := e.state.Final().Epochs().Current()
	reqs, err := e.createEpochComponents(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not create epoch components for current epoch: %w", err)
	}
	e.epoch = reqs
	_ = e.epoch.state // TODO lint

	return e, nil
}

// Ready returns a ready channel that is closed once the engine has fully
// started. For proposal engine, this is true once the underlying consensus
// algorithm has started.
func (e *Engine) Ready() <-chan struct{} {
	return e.unit.Ready(func() {
		// start up dependencies
		<-e.epoch.hotstuff.Ready()
		<-e.epoch.prop.Ready()
		<-e.epoch.sync.Ready()
	}, func() {
		// check the current phase on startup, in case we are in setup phase
		// and haven't yet voted for the next root QC
		phase, err := e.state.Final().Phase()
		if err != nil {
			e.log.Error().Err(err).Msg("could not check phase")
			return
		}
		if phase == flow.EpochPhaseSetup {
			e.prepareNextEpoch()
		}
	})
}

// Done returns a done channel that is closed once the engine has fully stopped.
func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done(func() {
		<-e.epoch.hotstuff.Done()
		<-e.epoch.prop.Done()
		<-e.epoch.sync.Done()
	})
}

func (e *Engine) createEpochComponents(epoch protocol.Epoch) (*EpochComponents, error) {

	state, prop, sync, hot, err := e.factory.Create(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not setup requirements for epoch (%d): %w", epoch, err)
	}

	reqs := &EpochComponents{
		state:    state,
		prop:     prop,
		sync:     sync,
		hotstuff: hot,
	}
	return reqs, err
}

// EpochSetupPhaseStarted handles the epoch setup phase started protocol event.
func (e *Engine) EpochSetupPhaseStarted(_ uint64, _ *flow.Header) {
	e.prepareNextEpoch()
}

// prepareNextEpoch is called either when we transition into the epoch
// setup phase, or when the node is restarted during the epoch setup phase. It
// kicks off setup tasks for the phase, in particular submitting a vote for the
// next epoch's root cluster QC.
func (e *Engine) prepareNextEpoch() {
	e.unit.Launch(func() {

		epoch := e.state.Final().Epochs().Next()

		ctx, cancel := context.WithCancel(e.unit.Ctx())
		defer cancel()
		err := e.voter.Vote(ctx, epoch)
		if err != nil {
			e.log.Error().Err(err).Msg("failed to submit QC vote for next epoch")
		}
	})
}
