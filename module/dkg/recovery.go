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

type BeaconKeyRecovery struct {
	events.Noop
	log           zerolog.Logger
	local         module.Local
	state         protocol.State
	localDKGState storage.EpochRecoveryMyBeaconKey
}

var _ protocol.Consumer = &BeaconKeyRecovery{}

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

func (b *BeaconKeyRecovery) EpochFallbackModeExited(epochCounter uint64, _ *flow.Header) {
	b.log.Info().Msgf("epoch fallback mode exited for epoch %d", epochCounter)
	err := b.tryRecoverMyBeaconPrivateKey(b.state.Final())
	if err != nil {
		irrecoverable.Throw(context.TODO(), fmt.Errorf("failed to get final epoch protocol state: %w", err))
	}
}

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
		return fmt.Errorf("could not retrieve my beacon private key for next epoch: %w", err)
	}
	if safe {
		log.Info().Msg("my beacon private key for next epoch is safe, nothing to do")
		return nil
	}

	myBeaconPrivateKey, safe, err := b.localDKGState.RetrieveMyBeaconPrivateKey(currentEpochCounter)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			log.Warn().Str(logging.KeySuspicious, "true").Msgf("no my beacon key for current epoch has been found")
			return nil
		}
		return fmt.Errorf("could not retrieve my beacon private key for current epoch: %w", err)
	}
	if !safe {
		log.Warn().Str(logging.KeySuspicious, "true").Msgf("my beacon key for current epoch is not safe")
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
		log.Info().Msgf("succesfully recovered my beacon private key for next epoch")
	} else {
		log.Warn().Str(logging.KeySuspicious, "true").Msgf("available my beacon key is not part of the next epoch DKG")
	}

	return nil
}
