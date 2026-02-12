package dkg

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// VerifyBeaconKeyForEpoch verifies that the beacon private key for the current epoch exists,
// is safe to use, and matches the expected public key from the protocol state.
// This function is intended to be called at node startup when the --require-beacon-key flag is set.
//
// Parameters:
//   - log: logger for outputting verification status
//   - nodeID: the node's identifier
//   - protocolState: the protocol state to query epoch and DKG information
//   - beaconKeys: storage for retrieving the beacon private key
//
// Returns nil if:
//   - the beacon key exists, is safe, and matches the expected public key, OR
//   - the node is not a DKG participant for the current epoch (nothing to verify)
//
// Returns an error if:
//   - the beacon key is missing from storage
//   - the beacon key exists but is marked unsafe
//   - the beacon key does not match the expected public key
//   - any unexpected error occurs while querying state
func VerifyBeaconKeyForEpoch(
	log zerolog.Logger,
	nodeID flow.Identifier,
	protocolState protocol.State,
	beaconKeys storage.SafeBeaconKeys,
) error {
	// Get current epoch
	currentEpoch, err := protocolState.Final().Epochs().Current()
	if err != nil {
		return fmt.Errorf("could not get current epoch for beacon key verification: %w", err)
	}
	epochCounter := currentEpoch.Counter()

	// Check if we're in the DKG committee for this epoch
	dkg, err := currentEpoch.DKG()
	if err != nil {
		return fmt.Errorf("could not get DKG info for epoch %d: %w", epochCounter, err)
	}

	// Check if this node is a DKG participant
	expectedPubKey, err := dkg.KeyShare(nodeID)
	if protocol.IsIdentityNotFound(err) {
		log.Info().Uint64("epoch", epochCounter).
			Msg("node is not a DKG participant for current epoch, skipping beacon key verification")
		return nil
	}
	if err != nil {
		return fmt.Errorf("could not get DKG key share for node %s in epoch %d: %w", nodeID, epochCounter, err)
	}

	// Verify beacon key exists and is safe
	key, safe, err := beaconKeys.RetrieveMyBeaconPrivateKey(epochCounter)
	if err != nil {
		return fmt.Errorf("beacon key for epoch %d not found in secrets database - cannot participate in consensus: %w", epochCounter, err)
	}
	if !safe {
		return fmt.Errorf("beacon key for epoch %d exists but is marked unsafe - cannot participate in consensus", epochCounter)
	}
	if key == nil {
		return fmt.Errorf("beacon key for epoch %d is nil - cannot participate in consensus", epochCounter)
	}

	// Verify key matches expected public key from protocol state
	if !expectedPubKey.Equals(key.PublicKey()) {
		return fmt.Errorf("beacon private key does not match expected public key for epoch %d (expected=%s, got=%s)",
			epochCounter, expectedPubKey, key.PublicKey())
	}

	log.Info().
		Uint64("epoch", epochCounter).
		Str("public_key", expectedPubKey.String()).
		Msg("beacon key verified successfully")

	return nil
}
