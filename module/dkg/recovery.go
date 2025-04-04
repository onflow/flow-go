package dkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

// the nextEpochNotYetCommitted is an entirely internal sentinel error, which indicates that no
// private Random Beacon Key for the next epoch could be recovered, as the next epoch is not
// yet committed.
var nextEpochNotYetCommitted = errors.New("next Epoch not yet committed")

// BeaconKeyRecovery is a specific module that attempts automatic recovery of the random beacon private key
// when exiting Epoch Fallback Mode [EFM].
// In the happy path of the protocol, each node that takes part in the DKG obtains a random beacon
// private key, which is stored in storage.DKGState. If the network enters EFM, the network can be recovered
// via the flow.EpochRecover service event, which exits EFM by specifying the subsequent epoch ("recovery epoch").
// This recovery epoch must have a Random Beacon committee with valid keys, but no successful DKG occurred.
// To solve this, by convention, we require the recovery epoch to re-use the Random Beacon public keys from
// the most recent successful DKG.
// Upon observing that EFM was exited, this component:
//   - looks up its Random Beacon private key from the last DKG
//   - looks up the Random Beacon public keys specified for the recovery epoch
//   - validates that its private key is compatible (matches the public key specified)
//   - if valid, persists that private key as the safe beacon key for the recovery epoch
type BeaconKeyRecovery struct {
	events.Noop
	log           zerolog.Logger
	local         module.Local
	state         protocol.State
	localDKGState storage.EpochRecoveryMyBeaconKey
}

var _ protocol.Consumer = &BeaconKeyRecovery{}

// NewBeaconKeyRecovery creates a new BeaconKeyRecovery instance and tries to recover the random beacon private key.
// This method ensures that we try to recover the random beacon private key even if we have missed the `EpochFallbackModeExited`
// protocol event (this could happen if the node has crashed between emitting and delivering the event).
// No errors are expected during normal operations.
func NewBeaconKeyRecovery(
	log zerolog.Logger,
	local module.Local,
	state protocol.State,
	localDKGState storage.EpochRecoveryMyBeaconKey,
) (*BeaconKeyRecovery, error) {
	recovery := &BeaconKeyRecovery{
		Noop:          events.Noop{},
		log:           log.With().Str("module", "my_beacon_key_recovery").Logger(),
		local:         local,
		state:         state,
		localDKGState: localDKGState,
	}

	err := recovery.recoverMyBeaconPrivateKey(state.Final())
	if err != nil && !errors.Is(err, nextEpochNotYetCommitted) {
		return nil, fmt.Errorf("could not recover my beacon private key when initializing: %w", err)
	}

	return recovery, nil
}

// EpochFallbackModeExited implements handler from protocol.Consumer to perform recovery of the beacon private key when
// this node has exited the epoch fallback mode.
func (b *BeaconKeyRecovery) EpochFallbackModeExited(epochCounter uint64, refBlock *flow.Header) {
	b.log.Info().Msgf("epoch fallback mode exited for epoch %d", epochCounter)
	err := b.recoverMyBeaconPrivateKey(b.state.AtHeight(refBlock.Height)) // refBlock must be finalized
	if err != nil {
		irrecoverable.Throw(context.TODO(), fmt.Errorf("failed to recovery my beacon private key: %w", err))
	}
}

// recoverMyBeaconPrivateKey performs the recovery of the random beacon private key for the next epoch by trying to use
// a safe 'my beacon key' from the current epoch (it is expected that this method will be called before entering the recovered epoch).
// If a safe 'my beacon key' is found, it will be stored in the storage.EpochRecoveryMyBeaconKey for the next epoch
// concluding the 'my beacon key' recovery.
// If there is a safe 'my beacon key' for the next epoch, or we are not in committed phase (DKG for next epoch is not available)
// then calling this method is no-op.
// Expected Errors under normal operations:
//   - `nextEpochNotYetCommitted` if the next epoch is not yet committed, hence we can't confirm whether we have a usable
//     Random Beacon key.
func (b *BeaconKeyRecovery) recoverMyBeaconPrivateKey(final protocol.Snapshot) error {
	head, err := final.Head()
	if err != nil {
		return fmt.Errorf("could not get head of snapshot: %w", err)
	}
	epochProtocolState, err := final.EpochProtocolState()
	if err != nil {
		return fmt.Errorf("could not get epoch protocol state: %w", err)
	}
	currentEpochCounter := epochProtocolState.Epoch()

	log := b.log.With().
		Uint64("height", head.Height).
		Uint64("view", head.View).
		Uint64("epochCounter", currentEpochCounter).
		Logger()
	// Only when the next epoch is committed, Random Beacon keys for that epoch's consensus participants are finally persisted
	// in `localDKGState`. It is important to wait until this point, so each node can locally check whether their private key
	// is consistent with its respective entry in the public key vector `EpochCommit.DKGParticipantKeys` (assuming the node
	// concluded the DKG). This mechanic is identical to the happy path (`EpochCommit` event emitted by Epoch System Smart
	// Contract) and the recovery epoch (`EpochCommit` event part of `EpochRecover` event, whose content is effectively
	// provided by the human governance committee).
	// The next epoch *not yet* being committed is only possible on the happy path, when the node boots up and checks
	// that it hasn't lost a `FallbackModeExited` notification. However, when observing the `EpochFallbackModeExited`
	// notification, the Epoch Phase *must* be `flow.EpochPhaseCommitted`, otherwise the state is corrupted.
	// Diligently verifying this case is important to avoid a situation where the node runs into the next epoch and failes
	// to access its Random Beacon key, just because this recovery here has failed before.
	if epochProtocolState.EpochPhase() != flow.EpochPhaseCommitted {
		log.Info().
			Str("EpochPhase", epochProtocolState.EpochPhase().String()).
			Msgf("Cannot (yet) determine dkg key for next epoch, as next epoch is not yet committed")
		return nextEpochNotYetCommitted
	}

	nextEpoch, err := final.Epochs().NextCommitted() // guaranteed to be committed
	if err != nil {
		return fmt.Errorf("could not get next epoch counter: %w", err)
	}
	nextEpochCounter := nextEpoch.Counter()
	_, safe, err := b.localDKGState.RetrieveMyBeaconPrivateKey(nextEpochCounter)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
		return fmt.Errorf("could not retrieve my beacon private key for the next epoch: %w", err)
	}
	if safe {
		log.Info().Msg("my beacon private key for the next epoch is safe, nothing to do")
		return nil
	}

	myBeaconPrivateKey, safe, err := b.localDKGState.RetrieveMyBeaconPrivateKey(currentEpochCounter)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Warn().Str(logging.KeyPotentialConfigurationProblem, "true").Msgf("no my beacon key for the current epoch has been found")
			return nil
		}
		return fmt.Errorf("could not retrieve my beacon private key for the current epoch: %w", err)
	}
	if !safe {
		log.Warn().Str(logging.KeyPotentialConfigurationProblem, "true").Msgf("my beacon key for the current epoch is not safe")
		return nil
	}

	nextEpochDKG, err := nextEpoch.DKG()
	if err != nil {
		return fmt.Errorf("could not get DKG for next epoch %d: %w", nextEpochCounter, err)
	}
	beaconPubKey, err := nextEpochDKG.KeyShare(b.local.NodeID())
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			log.Warn().Str(logging.KeyPotentialConfigurationProblem, "true").Msgf("current node is not part of the next epoch DKG")
			return nil
		}
		return fmt.Errorf("could not get beacon key share for my node(%x): %w", b.local.NodeID(), err)
	}
	if beaconPubKey.Equals(myBeaconPrivateKey.PublicKey()) {
		err := b.localDKGState.UpsertMyBeaconPrivateKey(nextEpochCounter, myBeaconPrivateKey, epochProtocolState.Entry().NextEpochCommit)
		if err != nil {
			return fmt.Errorf("could not overwrite my beacon private key for the next epoch: %w", err)
		}
		log.Warn().Msgf("succesfully recovered my beacon private key for the next epoch")
	} else {
		log.Debug().Msgf("my beacon key is not part of the next epoch DKG")
	}

	return nil
}
