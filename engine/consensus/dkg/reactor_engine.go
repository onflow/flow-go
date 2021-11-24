package dkg

import (
	"crypto/rand"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
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

// dkgInfo consolidates information about the current DKG protocol instance.
type dkgInfo struct {
	identities      flow.IdentityList
	phase1FinalView uint64
	phase2FinalView uint64
	phase3FinalView uint64
	// seed must be generated for each DKG instance, using a randomness source that is independent from all other nodes.
	seed []byte
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
	dkgState          storage.DKGState
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
	dkgState storage.DKGState,
	controllerFactory module.DKGControllerFactory,
	viewEvents events.Views,
) *ReactorEngine {

	logger := log.With().
		Str("engine", "dkg_reactor").
		Logger()

	return &ReactorEngine{
		unit:              engine.NewUnit(),
		log:               logger,
		me:                me,
		State:             state,
		keyStorage:        keyStorage,
		dkgState:          dkgState,
		controllerFactory: controllerFactory,
		viewEvents:        viewEvents,
		pollStep:          DefaultPollStep,
	}
}

// Ready implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully
// started.
func (e *ReactorEngine) Ready() <-chan struct{} {
	return e.unit.Ready(func() {
		// If we are starting up in the EpochSetup phase, try to start the DKG.
		// If the DKG for this epoch has been started previously, we will exit
		// and fail this epoch's DKG.
		snap := e.State.Final()

		phase, err := snap.Phase()
		if err != nil {
			// unexpected storage-level error
			e.log.Fatal().Err(err).Msg("failed to check epoch phase when starting DKG reactor engine")
			return
		}
		if phase != flow.EpochPhaseSetup {
			// start up in a non-setup phase - this is the typical path
			return
		}

		currentCounter, err := snap.Epochs().Current().Counter()
		if err != nil {
			// unexpected storage-level error
			e.log.Fatal().Err(err).Msg("failed to retrieve current epoch counter when starting DKG reactor engine")
			return
		}
		first, err := snap.Head()
		if err != nil {
			// unexpected storage-level error
			e.log.Fatal().Err(err).Msg("failed to retrieve finalized header when starting DKG reactor engine")
			return
		}

		e.startDKGForEpoch(currentCounter, first)
	})
}

// Done implements the module ReadyDoneAware interface. It returns a channel
// that will close when the engine has successfully stopped.
func (e *ReactorEngine) Done() <-chan struct{} {
	return e.unit.Done()
}

// EpochSetupPhaseStarted handles the EpochSetupPhaseStarted protocol event by
// starting the DKG process.
func (e *ReactorEngine) EpochSetupPhaseStarted(currentEpochCounter uint64, first *flow.Header) {
	e.startDKGForEpoch(currentEpochCounter, first)
}

// EpochCommittedPhaseStarted handles the EpochCommittedPhaseStarted protocol
// event by checking the consistency of our locally computed key share.
func (e *ReactorEngine) EpochCommittedPhaseStarted(currentEpochCounter uint64, _ *flow.Header) {
	e.handleEpochCommittedPhaseStarted(currentEpochCounter)
}

// startDKGForEpoch starts the DKG instance for the given epoch, only if we have
// never started the DKG during setup phase for the given epoch. This allows consensus nodes which
// boot from a state snapshot within the EpochSetup phase to run the DKG.
//
// It starts a new controller for the epoch and registers the triggers to regularly
// query the DKG smart-contract and transition between phases at the specified views.
//
func (e *ReactorEngine) startDKGForEpoch(currentEpochCounter uint64, first *flow.Header) {

	firstID := first.ID()
	nextEpochCounter := currentEpochCounter + 1
	log := e.log.With().
		Uint64("current_epoch", currentEpochCounter).
		Uint64("view", first.View).
		Hex("block", firstID[:]).
		Logger()

	// if we have started the dkg for this epoch already, exit
	started, err := e.dkgState.GetDKGStarted(nextEpochCounter)
	if err != nil {
		// unexpected storage-level error
		log.Fatal().Err(err).Msg("could not check whether DKG is started")
	}
	if started {
		log.Warn().Msg("DKG already started, skipping starting the DKG for this epoch")
		return
	}

	// flag that we are starting the dkg for this epoch
	err = e.dkgState.SetDKGStarted(nextEpochCounter)
	if err != nil {
		// unexpected storage-level error
		log.Fatal().Err(err).Msg("could not set dkg started")
	}

	curDKGInfo, err := e.getDKGInfo(firstID)
	if err != nil {
		// unexpected storage-level error
		log.Fatal().Err(err).Msg("could not retrieve epoch info")
	}

	committee := curDKGInfo.identities.Filter(filter.IsVotingConsensusCommitteeMember)

	log.Info().
		Uint64("phase1", curDKGInfo.phase1FinalView).
		Uint64("phase2", curDKGInfo.phase2FinalView).
		Uint64("phase3", curDKGInfo.phase3FinalView).
		Interface("members", committee.NodeIDs()).
		Msg("epoch info")

	controller, err := e.controllerFactory.Create(
		dkgmodule.CanonicalInstanceID(first.ChainID, nextEpochCounter),
		committee,
		curDKGInfo.seed,
	)
	if err != nil {
		// no expected errors in controller factory
		log.Fatal().Err(err).Msg("could not create DKG controller")
	}
	e.controller = controller

	e.unit.Launch(func() {
		log.Info().Msg("DKG Run")
		err := e.controller.Run()
		if err != nil {
			// TODO handle crypto sentinels and do not crash here
			log.Fatal().Err(err).Msg("DKG Run error")
		}
	})

	// NOTE:
	// We register two callbacks for views that mark a state transition: one for
	// polling broadcast messages, and one for triggering the phase transition.
	// It is essential that all polled broadcast messages are processed before
	// starting the phase transition. Here we register the polling callback
	// before the phase transition, which guarantees that it will be called
	// before because callbacks for the same views are executed on a FIFO basis.
	// Moreover, the poll callback does not return until all received messages
	// are processed by the underlying DKG controller (as guaranteed by the
	// specifications and implementations of the DKGBroker and DKGController
	// interfaces).

	for view := curDKGInfo.phase1FinalView; view > first.View; view -= e.pollStep {
		e.registerPoll(view)
	}
	e.registerPhaseTransition(curDKGInfo.phase1FinalView, dkgmodule.Phase1, e.controller.EndPhase1)

	for view := curDKGInfo.phase2FinalView; view > curDKGInfo.phase1FinalView; view -= e.pollStep {
		e.registerPoll(view)
	}
	e.registerPhaseTransition(curDKGInfo.phase2FinalView, dkgmodule.Phase2, e.controller.EndPhase2)

	for view := curDKGInfo.phase3FinalView; view > curDKGInfo.phase2FinalView; view -= e.pollStep {
		e.registerPoll(view)
	}
	e.registerPhaseTransition(curDKGInfo.phase3FinalView, dkgmodule.Phase3, e.end(nextEpochCounter))
}

// handleEpochCommittedPhaseStarted checks that the local DKG completed and
// that our locally computed key share is consistent with the canonical key vector.
//
// If our key is inconsistent, we log a warning message.
// CAUTION: we will still sign using the inconsistent key, to be fixed with Crypto
// v2 by falling back to staking signatures in this case.
//
func (e *ReactorEngine) handleEpochCommittedPhaseStarted(currentEpochCounter uint64) {
	nextDKG, err := e.State.Final().Epochs().Next().DKG()
	if err != nil {
		e.log.Err(err).Msg("checking DKG key consistency: could not retrieve next DKG info")
		return
	}

	dkgPrivInfo, err := e.keyStorage.RetrieveMyDKGPrivateInfo(currentEpochCounter + 1)
	if err != nil {
		e.log.Err(err).Msg("checking DKG key consistency: could not retrieve DKG private info for next epoch")
		return
	}

	nextDKGPubKey, err := nextDKG.KeyShare(dkgPrivInfo.NodeID)
	if err != nil {
		e.log.Err(err).Msg("checking DKG key consistency: could not retrieve DKG public key for next epoch")
		return
	}

	localPubKey := dkgPrivInfo.RandomBeaconPrivKey.PublicKey()

	if !nextDKGPubKey.Equals(localPubKey) {
		e.log.Warn().Msg("checking DKG key consistency: locally computed dkg public key does not match dkg public key for next epoch")
	}
}

func (e *ReactorEngine) getDKGInfo(firstBlockID flow.Identifier) (*dkgInfo, error) {
	currEpoch := e.State.AtBlockID(firstBlockID).Epochs().Current()
	nextEpoch := e.State.AtBlockID(firstBlockID).Epochs().Next()

	identities, err := nextEpoch.InitialIdentities()
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch identities: %w", err)
	}
	phase1Final, phase2Final, phase3Final, err := protocol.DKGPhaseViews(currEpoch)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve epoch dkg final views: %w", err)
	}
	seed := make([]byte, crypto.SeedMinLenDKG)
	_, err = rand.Read(seed)
	if err != nil {
		return nil, fmt.Errorf("could not generate random seed: %w", err)
	}

	info := &dkgInfo{
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
				log.Err(err).Msg("failed to poll DKG smart-contract")
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
