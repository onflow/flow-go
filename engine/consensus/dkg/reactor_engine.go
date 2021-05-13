package dkg

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine"
	dkgmodel "github.com/onflow/flow-go/model/dkg"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	dkgmodule "github.com/onflow/flow-go/module/dkg"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
)

// DefaultPollStep specifies the default number of views that separate two calls
// to the DKG smart-contract to read broadcast messages.
const DefaultPollStep = 10

// ATTENTION what should this be?
var SeedIndices = []uint32{1, 1, 1}

type epochInfo struct {
	identities      flow.IdentityList
	phase1FinalView uint64
	phase2FinalView uint64
	phase3FinalView uint64
	seed            []byte
}

// ReactorEngine is an engine that reacts to chain events to start new DKG runs,
// and manage subsequent phase transitions. Any unexpected error triggers a
// panic as it would undermine the security of the protocol.
type ReactorEngine struct {
	events.Noop
	unit              *engine.Unit
	log               zerolog.Logger
	me                module.Local
	State             protocol.State
	keyStorage        storage.DKGKeys
	controller        module.DKGController
	controllerFactory module.DKGControllerFactory
	viewEvents        events.Views
	pollStep          uint64
}

// NewReactorEngine return a new ReactorEngine.
func NewReactorEngine(
	log zerolog.Logger,
	me module.Local,
	state protocol.State,
	keyStorage storage.DKGKeys,
	controllerFactory module.DKGControllerFactory,
	viewEvents events.Views,
) *ReactorEngine {

	return &ReactorEngine{
		unit:              engine.NewUnit(),
		log:               log,
		me:                me,
		State:             state,
		keyStorage:        keyStorage,
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

	epochInfo, err := e.getNextEpochInfo(firstID)
	if err != nil {
		e.log.Fatal().Err(err).Msg("could not retrieve epoch info")
	}

	committee := epochInfo.identities.Filter(filter.IsVotingConsensusCommitteeMember)

	controller, err := e.controllerFactory.Create(
		fmt.Sprintf("dkg-%d", counter),
		committee,
		epochInfo.seed,
	)
	if err != nil {
		e.log.Fatal().Err(err).Msg("could not create DKG controller")
	}
	e.controller = controller

	e.unit.Launch(func() {
		e.log.Info().Msg("DKG Run")
		err := e.controller.Run()
		if err != nil {
			e.log.Fatal().Err(err).Msg("DKG Run error")
		}
	})

	// NOTE:
	// We register two callbacks for views that mark a state transition: one for
	// polling broadcast messages, and one for triggering the phase transition.
	// It is essential that all polled broadcast messages are processed before
	// starting the phase transition. Here we register the polling callback
	// before the phase transition, which guarantees that it will be called
	// before because callbacks for the same views are executed on a FIFO basis.
	// Moreover, the poll calback does not return until all received messages
	// are processed by the underlying DKG controller (as guaranteed by the
	// specifications and implementations of the DKGBroker and DKGController
	// interfaces).

	for view := epochInfo.phase1FinalView; view > first.View; view -= e.pollStep {
		e.registerPoll(view)
	}
	e.registerPhaseTransition(epochInfo.phase1FinalView, dkgmodule.Phase1, e.controller.EndPhase1)

	for view := epochInfo.phase2FinalView; view > epochInfo.phase1FinalView; view -= e.pollStep {
		e.registerPoll(view)
	}
	e.registerPhaseTransition(epochInfo.phase2FinalView, dkgmodule.Phase2, e.controller.EndPhase2)

	for view := epochInfo.phase3FinalView; view > epochInfo.phase2FinalView; view -= e.pollStep {
		e.registerPoll(view)
	}
	e.registerPhaseTransition(epochInfo.phase3FinalView, dkgmodule.Phase3, e.end(counter))
}

func (e *ReactorEngine) getNextEpochInfo(firstBlockID flow.Identifier) (*epochInfo, error) {
	epoch := e.State.AtBlockID(firstBlockID).Epochs().Next()
	identities, err := epoch.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch identities: %w", err)
	}
	phase1Final, err := epoch.DKGPhase1FinalView()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch phase 1 final view: %w", err)
	}
	phase2Final, err := epoch.DKGPhase2FinalView()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch phase 2 final view: %w", err)
	}
	phase3Final, err := epoch.DKGPhase3FinalView()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch phase 3 final view: %w", err)
	}
	seed, err := epoch.Seed(SeedIndices...)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch seed: %w", err)
	}
	info := &epochInfo{
		identities:      identities,
		phase1FinalView: phase1Final,
		phase2FinalView: phase2Final,
		phase3FinalView: phase3Final,
		seed:            seed,
	}
	return info, nil
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
				log.Fatal().Err(err).Msg("failed to poll DKG smart-contract")
			}
		})
	})
}

// registerPhaseTransition instructs the engine to change phases at the
// specified view.
func (e *ReactorEngine) registerPhaseTransition(view uint64, fromState dkgmodule.State, phaseTransition func() error) {
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
			err := phaseTransition()
			if err != nil {
				log.Fatal().Err(err).Msgf("node failed to end %s", fromState)
			}
			log.Info().Msgf("ended %s successfully", fromState)
		})
	})
}

// end returns a callback that is used to end the DKG protocol, save the
// resulting private key to storage, and publish the other results to the DKG
// smart-contract.
func (e *ReactorEngine) end(epochCounter uint64) func() error {
	return func() error {
		err := e.controller.End()
		if err != nil {
			return err
		}

		privateShare, _, _ := e.controller.GetArtifacts()

		privKeyInfo := dkgmodel.DKGParticipantPriv{
			NodeID: e.me.NodeID(),
			RandomBeaconPrivKey: encodable.RandomBeaconPrivKey{
				PrivateKey: privateShare,
			},
			GroupIndex: e.controller.GetIndex(),
		}
		err = e.keyStorage.InsertMyDKGPrivateInfo(epochCounter, &privKeyInfo)
		if err != nil {
			return fmt.Errorf("couldn't save DKG private key in db: %w", err)
		}

		err = e.controller.SubmitResult()
		if err != nil {
			return fmt.Errorf("couldn't publish DKG results: %w", err)
		}

		return nil
	}
}
