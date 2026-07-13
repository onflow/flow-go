package dkg

import (
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// VerifyBeaconKeyForEpoch verifies that the beacon private key for the current epoch exists,
// is safe to use, and matches the expected public key from the protocol state.
// This function is intended to be called at node startup. When the --require-beacon-key flag is set,
// this function returns an error (and should crash the node). Otherwise, logs a warning and returns nil.
//
// Parameters:
//   - log: logger for outputting verification status
//   - nodeID: the node's identifier
//   - protocolState: the protocol state to query epoch and DKG information
//   - beaconKeys: storage for retrieving the beacon private key
//   - requireKeyPresent: if false, verification failures are logged as warnings and the function returns nil instead of an error
//
// Returns nil if:
//   - requireKeyPresent is false and a verification failure occurs (logged as warning), OR
//   - the beacon key exists, is safe, and matches the expected public key, OR
//   - the node is not a DKG participant for the current epoch (nothing to verify)
//
// This is a binary validation function and all errors indicate that validation failed, which should be interpreted by the upper layer as an exception.
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
	requireKeyPresent bool,
) error {
	log = log.With().Str("component", "startup_beacon_key_verifier").Logger()
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
		if errors.Is(err, storage.ErrNotFound) {
			if !requireKeyPresent {
				log.Warn().Uint64("epoch", epochCounter).
					Msg("beacon key not found for current epoch, but --require-beacon-key flag is not set, skipping verification failure")
				return nil
			}
		}
		return fmt.Errorf("could not retrieve beacon key for epoch %d from secrets database - cannot participate in consensus: %w", epochCounter, err)
	}

	if !requireKeyPresent {
		log.Warn().Uint64("epoch", epochCounter).
			Msg("beacon key verification failed for current epoch, but --require-beacon-key flag is not set, skipping verification failure")
		return nil
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
