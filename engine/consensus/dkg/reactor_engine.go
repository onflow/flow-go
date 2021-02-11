package dkg

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	dkgmod "github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
)

// DefaultPollStep specifies the default number of views that separate two calls
// to the DKG smart-contract to read broadcast messages.
const DefaultPollStep = 10

// ReactorEngine is an engine that reacts to chain events to start new DKG runs,
// and manage subsequent phase transitions. Any unexpected error triggers a
// panic as it would undermine the security of the protocol.
type ReactorEngine struct {
	events.Noop
	unit              *engine.Unit
	log               zerolog.Logger
	me                module.Local
	setups            storage.EpochSetups
	statuses          storage.EpochStatuses
	controller        module.DKGController
	controllerFactory module.DKGControllerFactory
	viewEvents        events.Views
	pollStep          uint64
}

// NewReactorEngine return a new ReactorEngine.
func NewReactorEngine(
	log zerolog.Logger,
	me module.Local,
	setups storage.EpochSetups,
	statuses storage.EpochStatuses,
	controllerFactory module.DKGControllerFactory,
	viewEvents events.Views,
) *ReactorEngine {

	return &ReactorEngine{
		unit:              engine.NewUnit(),
		log:               log,
		me:                me,
		setups:            setups,
		statuses:          statuses,
		controllerFactory: controllerFactory,
		viewEvents:        viewEvents,
		pollStep:          DefaultPollStep,
	}
}

// Ready implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully
// started.
func (e *ReactorEngine) Ready() <-chan struct{} {
	return e.unit.Ready()
}

// Done implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully stopped.
func (e *ReactorEngine) Done() <-chan struct{} {
	return e.unit.Done()
}

// EpochSetupPhaseStarted handles the EpochSetupPhaseStared protocol event. It
// starts a new controller for the epoch and registers the triggers to regularly
// query the DKG smart-contract and transition between phases at the specified
// views.
func (e *ReactorEngine) EpochSetupPhaseStarted(counter uint64, first *flow.Header) {
	firstID := first.ID()
	log := e.log.With().
		Uint64("counter", counter).
		Uint64("view", first.View).
		Hex("block", firstID[:]).
		Logger()
	log.Info().Msg("EpochSetup received")

	epochStatus, err := e.statuses.ByBlockID(first.ID())
	if err != nil {
		log.Err(err).Msg("could not retrieve epoch state")
		panic(err)
	}
	setup, err := e.setups.ByID(epochStatus.CurrentEpoch.SetupID)
	if err != nil {
		log.Err(err).Msg("could not retrieve setup event for current epoch")
		panic(err)
	}

	committee := setup.Participants.NodeIDs()
	index := -1
	for i, id := range committee {
		if id == e.me.NodeID() {
			index = i
			break
		}
	}
	if index < 0 {
		err := fmt.Errorf("dkg-processor engine id does not belong to dkg committee")
		e.log.Err(err).Msg("bad committee")
		panic(err)
	}

	controller, err := e.controllerFactory.Create(
		fmt.Sprintf("dkg-%d", setup.Counter),
		committee,
		index,
		setup.RandomSource,
	)
	if err != nil {
		e.log.Err(err).Msg("could not create DKG controller")
		panic(err)
	}
	e.controller = controller

	e.unit.Launch(func() {
		e.log.Info().Msg("DKG Run")
		err := e.controller.Run()
		if err != nil {
			e.log.Err(err).Msg("DKG Run error")
			panic(err)
		}
	})

	for view := setup.DKGPhase1FinalView; view > first.View; view -= e.pollStep {
		e.registerPoll(view)
	}
	e.registerPhaseTransition(setup.DKGPhase1FinalView, dkgmod.Phase1, e.controller.EndPhase1)

	for view := setup.DKGPhase2FinalView; view > setup.DKGPhase1FinalView; view -= e.pollStep {
		e.registerPoll(view)
	}
	e.registerPhaseTransition(setup.DKGPhase2FinalView, dkgmod.Phase2, e.controller.EndPhase2)

	for view := setup.DKGPhase3FinalView; view > setup.DKGPhase2FinalView; view -= e.pollStep {
		e.registerPoll(view)
	}
	e.registerPhaseTransition(setup.DKGPhase3FinalView, dkgmod.Phase3, e.controller.End)
}

// registerPoll instructs the engine to query the DKG smart-contract for new
// broadcast messages at the specified view.
func (e *ReactorEngine) registerPoll(view uint64) {
	e.viewEvents.OnView(view, func(header *flow.Header) {
		e.unit.Launch(func() {
			e.unit.Lock()
			defer e.unit.Unlock()

			blockID := header.ID()
			log := e.log.With().
				Uint64("view", view).
				Uint64("height", header.Height).
				Hex("block_id", blockID[:]).
				Logger()

			log.Info().Msg("polling DKG smart-contract...")
			err := e.controller.Poll(header.ID())
			if err != nil {
				log.Error().Err(err).Msg("failed to poll DKG smart-contract")
				panic(err)
			}
		})
	})
}

// registerPhaseTransition instructs the engine to change phases at the
// specified view.
func (e *ReactorEngine) registerPhaseTransition(view uint64, fromState dkgmod.State, callback func() error) {
	e.viewEvents.OnView(view, func(header *flow.Header) {
		e.unit.Launch(func() {
			e.unit.Lock()
			defer e.unit.Unlock()

			blockID := header.ID()
			log := e.log.With().
				Uint64("view", view).
				Hex("block_id", blockID[:]).
				Logger()

			log.Info().Msgf("ending %s...", fromState)
			err := callback()
			if err != nil {
				log.Error().Err(err).Msgf("failed to end %s", fromState)
				panic(err)
			}
			log.Info().Msgf("ended %s successfully", fromState)
		})
	})
}
