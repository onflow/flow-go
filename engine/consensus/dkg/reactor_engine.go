package dkg

import (
	"crypto/rand"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
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
	keyStorage        storage.BeaconPrivateKeys
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
	keyStorage storage.BeaconPrivateKeys,
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
func (e *ReactorEngine) EpochSetupPhaseStarted(currentEpochCounter uint64, first *flow.Header) {
	firstID := first.ID()

	lg := e.log.With().
		Uint64("current_epoch", currentEpochCounter).
		Uint64("view", first.View).
		Hex("block", firstID[:]).
		Logger()

	curDKGInfo, err := e.getDKGInfo(firstID)
	if err != nil {
		lg.Fatal().Err(err).Msg("could not retrieve epoch info")
	}

	committee := curDKGInfo.identities.Filter(filter.IsVotingConsensusCommitteeMember)

	lg.Info().
		Uint64("phase1", curDKGInfo.phase1FinalView).
		Uint64("phase2", curDKGInfo.phase2FinalView).
		Uint64("phase3", curDKGInfo.phase3FinalView).
		Interface("members", committee.NodeIDs()).
		Msg("epoch info")

	nextEpochCounter := currentEpochCounter + 1
	controller, err := e.controllerFactory.Create(
		dkgmodule.CanonicalInstanceID(first.ChainID, nextEpochCounter),
		committee,
		curDKGInfo.seed,
	)
	if err != nil {
		lg.Fatal().Err(err).Msg("could not create DKG controller")
	}
	e.controller = controller

	e.unit.Launch(func() {
		lg.Info().Msg("DKG Run")
		err := e.controller.Run()
		if err != nil {
			lg.Fatal().Err(err).Msg("DKG Run error")
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

// EpochCommittedPhaseStarted handles the EpochCommittedPhaseStarted protocol event. It
// compares the key vector locally produced by Consensus nodes against the FlowDKG smart contract
// key vectors. If the keys don't match a log statement will be invoked. In the happy case the locally
// produced key should match, if the keys do not match the node will have a invalid random beacon key.
func (e *ReactorEngine) EpochCommittedPhaseStarted(currentEpochCounter uint64, first *flow.Header) {
	nextDKG, err := e.State.Final().Epochs().Next().DKG()
	if err != nil {
		e.log.Err(err).Msg("checking DKG key consistency: could not retrieve next DKG info")
		return
	}

	dkgPrivInfo, err := e.keyStorage.RetrieveMyBeaconPrivateKey(currentEpochCounter + 1)
	if err != nil {
		e.log.Err(err).Msg("checking DKG key consistency: could not retrieve DKG private info for next epoch")
		return
	}

	nextDKGPubKey, err := nextDKG.KeyShare(e.me.NodeID())
	if err != nil {
		e.log.Err(err).Msg("checking DKG key consistency: could not retrieve DKG public key for next epoch")
		return
	}

	localPubKey := dkgPrivInfo.PublicKey()

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
			if crypto.IsDKGFailureError(err) {
				e.log.Warn().Msgf("node %s with index %d failed DKG locally: %s", e.me.NodeID(), e.controller.GetIndex(), err.Error())
			} else {
				return err
			}
		}

		privateShare, _, _ := e.controller.GetArtifacts()

		privKeyInfo := encodable.RandomBeaconPrivKey{
			PrivateKey: privateShare,
		}
		err = e.keyStorage.InsertMyBeaconPrivateKey(epochCounter, &privKeyInfo)
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
