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
)

type BeaconKeyRecovery struct {
	events.Noop
	local         module.Local
	state         protocol.State
	localDKGState storage.EpochRecoveryDKGState
}

var _ protocol.Consumer = &BeaconKeyRecovery{}

func NewBeaconKeyRecovery(local module.Local,
	state protocol.State,
	localDKGState storage.EpochRecoveryDKGState,
) (*BeaconKeyRecovery, error) {
	recovery := &BeaconKeyRecovery{
		Noop:          events.Noop{},
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

func (b *BeaconKeyRecovery) EpochFallbackModeExited(_ uint64, _ *flow.Header) {
	finalEpochProtocolState, err := b.state.Final().EpochProtocolState()
	if err != nil {
		irrecoverable.Throw(context.TODO(), fmt.Errorf("failed to get final epoch protocol state: %w", err))
	}
	if finalEpochProtocolState.EpochPhase() != flow.EpochPhaseCommitted {
		return
	}

}

func (b *BeaconKeyRecovery) tryRecoverMyBeaconPrivateKey(final protocol.Snapshot) error {
	epochProtocolState, err := final.EpochProtocolState()
	if err != nil {
		return fmt.Errorf("could not get epoch protocol state: %w", err)
	}
	if epochProtocolState.EpochPhase() != flow.EpochPhaseCommitted {
		return nil
	}

	nextEpochCounter, err := final.Epochs().Next().Counter()
	if err != nil {
		return fmt.Errorf("could not get next epoch counter: %w", err)
	}
	_, safe, err := b.localDKGState.RetrieveMyBeaconPrivateKey(nextEpochCounter)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("could not retrieve my beacon private key for next epoch: %w", err)
	}
	if !safe {
		return nil
	}

	currentEpochCounter := epochProtocolState.Epoch()
	myBeaconPrivateKey, safe, err := b.localDKGState.RetrieveMyBeaconPrivateKey(currentEpochCounter)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			return nil
		}
		return fmt.Errorf("could not retrieve my beacon private key for current epoch: %w", err)
	}
	if !safe {
		return nil
	}

	nextEpochDKG, err := final.Epochs().Next().DKG()
	if err != nil {
		return fmt.Errorf("could not get DKG for next epoch : %w", err)
	}
	beaconPubKey, err := nextEpochDKG.KeyShare(b.local.NodeID())
	if err != nil {
		if protocol.IsIdentityNotFound(err) {
			return nil
		}
		return fmt.Errorf("could not get beacon key share for my node(%x): %w", b.local.NodeID(), err)
	}
	if beaconPubKey.Equals(myBeaconPrivateKey.PublicKey()) {
		err := b.localDKGState.OverwriteMyBeaconPrivateKey(nextEpochCounter, myBeaconPrivateKey)
		if err != nil {
			return fmt.Errorf("could not overwrite my beacon private key for the next epoch: %w", err)
		}
	}

	return nil
}
