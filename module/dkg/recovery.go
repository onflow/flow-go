package dkg

import (
	"context"
	"errors"
	"fmt"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
	"github.com/rs/zerolog"
)

// BeaconKeyRecovery is a specific module that performs automatic recovery of the random beacon private key
// when exiting the epoch fallback mode.
// When following the happy path of the protocol, each node that takes part in the DKG eventually obtains a random beacon
// private key which is stored in storage.DKGState. When the network enters EFM, then eventually the network will be recovered
// by submitting the recovery epoch which will effectively exit the EFM. When recovering the next epoch,
// the operator will use a result of last successful DKG. However, each node needs to track this event and correctly fill
// the storage.DKGState using the last obtained 'my beacon key' for recovered(next) epoch.
// This module checks if there is a valid 'my beacon key' for the next epoch, and if not, it tries to recover it from the previous epoch.
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

	err := recovery.tryRecoverMyBeaconPrivateKey(state.Final())
	if err != nil {
		return nil, fmt.Errorf("could not recover my beacon private key when initializing: %w", err)
	}

	return recovery, nil
}

// EpochFallbackModeExited implements handler from protocol.Consumer to perform recovery of the beacon private key when
// this node has exited the epoch fallback mode.
func (b *BeaconKeyRecovery) EpochFallbackModeExited(epochCounter uint64, _ *flow.Header) {
	b.log.Info().Msgf("epoch fallback mode exited for epoch %d", epochCounter)
	err := b.tryRecoverMyBeaconPrivateKey(b.state.Final())
	if err != nil {
		irrecoverable.Throw(context.TODO(), fmt.Errorf("failed to get final epoch protocol state: %w", err))
	}
}

// tryRecoverMyBeaconPrivateKey performs the recovery of the random beacon private key for the next epoch by trying to use
// a safe 'my beacon key' from the current epoch (it is expected that this method will be called before entering the recovered epoch).
// If a safe 'my beacon key' is found, it will be stored in the storage.EpochRecoveryMyBeaconKey for the next epoch
// concluding the 'my beacon key' recovery.
// If there is a safe 'my beacon key' for the next epoch, or we are not in committed phase (DKG for next epoch is not available)
// then calling this method is no-op.
// No errors are expected during normal operations.
func (b *BeaconKeyRecovery) tryRecoverMyBeaconPrivateKey(final protocol.Snapshot) error {
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
	if epochProtocolState.EpochPhase() != flow.EpochPhaseCommitted {
		log.Info().Msgf("epoch is in phase %s", epochProtocolState.EpochPhase())
		return nil
	}

	nextEpochCounter, err := final.Epochs().Next().Counter()
	if err != nil {
		return fmt.Errorf("could not get next epoch counter: %w", err)
	}
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
			log.Warn().Str(logging.KeySuspicious, "true").Msgf("no my beacon key for the current epoch has been found")
			return nil
		}
		return fmt.Errorf("could not retrieve my beacon private key for the current epoch: %w", err)
	}
	if !safe {
		log.Warn().Str(logging.KeySuspicious, "true").Msgf("my beacon key for the current epoch is not safe")
		return nil
	}

	nextEpochDKG, err := final.Epochs().Next().DKG()
	if err != nil {
		return fmt.Errorf("could not get DKG for next epoch : %w", err)
	}
	beaconPubKey, err := nextEpochDKG.KeyShare(b.local.NodeID())
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			log.Warn().Str(logging.KeySuspicious, "true").Msgf("current node is not part of the next epoch DKG")
			return nil
		}
		return fmt.Errorf("could not get beacon key share for my node(%x): %w", b.local.NodeID(), err)
	}
	if beaconPubKey.Equals(myBeaconPrivateKey.PublicKey()) {
		err := b.localDKGState.OverwriteMyBeaconPrivateKey(nextEpochCounter, myBeaconPrivateKey)
		if err != nil {
			return fmt.Errorf("could not overwrite my beacon private key for the next epoch: %w", err)
		}
		log.Info().Msgf("succesfully recovered my beacon private key for the next epoch")
	} else {
		log.Warn().Str(logging.KeySuspicious, "true").Msgf("available my beacon key is not part of the next epoch DKG")
	}

	return nil
}
