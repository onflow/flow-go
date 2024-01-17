package signature

import (
	"errors"
	"fmt"

	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// EpochAwareRandomBeaconKeyStore provides an abstraction to query random beacon private key
// for a given view.
// Internally it indexes and caches the private keys by epoch.
type EpochAwareRandomBeaconKeyStore struct {
	epochLookup module.EpochLookup     // used to fetch epoch counter by view
	keys        storage.SafeBeaconKeys // used to fetch DKG private key by epoch

	privateKeys map[uint64]crypto.PrivateKey // cache of privateKeys by epoch
}

func NewEpochAwareRandomBeaconKeyStore(epochLookup module.EpochLookup, keys storage.SafeBeaconKeys) *EpochAwareRandomBeaconKeyStore {
	return &EpochAwareRandomBeaconKeyStore{
		epochLookup: epochLookup,
		keys:        keys,
		privateKeys: make(map[uint64]crypto.PrivateKey),
	}
}

// ByView returns the random beacon signer for signing objects at a given view.
// The view determines the epoch, which determines the DKG private key underlying the signer.
// It returns:
//   - (signer, nil) if DKG succeeded locally in the epoch of the view, signer is not nil
//   - (nil, model.ErrViewForUnknownEpoch) if no epoch found for given view
//   - (nil, module.ErrNoBeaconKeyForEpoch) if beacon key for epoch is unavailable
//   - (nil, error) if there is any exception
func (s *EpochAwareRandomBeaconKeyStore) ByView(view uint64) (crypto.PrivateKey, error) {
	epoch, err := s.epochLookup.EpochForViewWithFallback(view)
	if err != nil {
		return nil, fmt.Errorf("could not get epoch by view %v: %w", view, err)
	}

	// When DKG has completed,
	//   - if a node successfully generated the DKG key, the valid private key will be stored in database.
	//   - if a node failed to generate the DKG key, we will save a record in database to indicate this
	//      node has no private key for this epoch.
	// Within the epoch, we can look up my random beacon private key for the epoch. There are 3 cases:
	//   1. DKG has completed, and the private key is stored in database, and we can retrieve it (happy path)
	//   2. DKG has completed, but we failed to generate a private key (unhappy path)
	//   3. DKG has not completed locally yet
	key, found := s.privateKeys[epoch]
	if found {
		// a nil key means that we don't (and will never) have a beacon key for this epoch
		if key == nil {
			// case 2:
			return nil, fmt.Errorf("beacon key for epoch %d (queried view: %d) never available: %w", epoch, view, module.ErrNoBeaconKeyForEpoch)
		}
		// case 1:
		return key, nil
	}

	privKey, safe, err := s.keys.RetrieveMyBeaconPrivateKey(epoch)
	if err != nil {
		if errors.Is(err, storage.ErrNotFound) {
			// case 3:
			return nil, fmt.Errorf("beacon key for epoch %d (queried view: %d) not available yet: %w", epoch, view, module.ErrNoBeaconKeyForEpoch)
		}
		return nil, fmt.Errorf("[unexpected] could not retrieve beacon key for epoch %d (queried view: %d): %w", epoch, view, err)
	}

	// If DKG ended without a safe end state, there will never be a valid beacon key for this epoch.
	// Since this fact never changes, we cache a nil signer for this epoch, so that when this function
	// is called again for the same epoch, we don't need to query the database.
	if !safe {
		// case 2:
		s.privateKeys[epoch] = nil
		return nil, fmt.Errorf("DKG for epoch %d ended without safe beacon key (queried view: %d): %w", epoch, view, module.ErrNoBeaconKeyForEpoch)
	}

	// case 1: DKG succeeded and a beacon key is available -> cache the key for future queries
	s.privateKeys[epoch] = privKey

	return privKey, nil
}
