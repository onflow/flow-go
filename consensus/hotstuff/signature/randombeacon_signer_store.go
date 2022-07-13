package signature

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
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

// ByView returns the random beacon signer for signing objects at a
// given view. The view determines the epoch, which determines the DKG private
// key underlying the signer.
// It returns:
//  - (signer, nil) if DKG succeeded locally in the epoch of the view, signer is not nil
//  - (nil, protocol.ErrEpochNotCommitted) if no epoch found for given view
//  - (nil, DKGFailError) if DKG failed locally in the epoch of the view
//  - (nil, error) if there is any exception
func (s *EpochAwareRandomBeaconKeyStore) ByView(view uint64) (crypto.PrivateKey, error) {
	// fetching the epoch by view, if epoch is found, then DKG must have been completed
	epoch, err := s.epochLookup.EpochForViewWithFallback(view)
	if err != nil {
		return nil, fmt.Errorf("could not get epoch by view %v: %w", view, err)
	}

	// when DKG has completed,
	// 1. if a node successfully generated the DKG key, the valid private key will be stored in database.
	// 2. if a node failed to generate the DKG key, we will save a record in database to indicate this
	//       node has no private key for this epoch
	// within the epoch, we can lookup my random beacon private key for the epoch. There are 3 cases:
	// 1. DKG has completed, and the private key is stored in database, and we can retrieve it (happy path)
	// 2. DKG has completed, but we failed it, and we marked in the database
	// 		that there is no private key for this epoch (unhappy path)
	// 3. DKG has completed, but for some reason we don't find the private key in the database (exception)
	// 4. DKG was not completed (fatal error, we should not run into with EECC, because we still stay at
	// 		the current Epoch where DKG has completed.
	key, found := s.privateKeys[epoch]
	if found {
		// A nil key means that we don't have a Random Beacon key for this epoch.
		if key == nil {
			return nil, fmt.Errorf("DKG for epoch %v failed, at view %v: %w",
				epoch, view, module.DKGFailError)
		}
		return key, nil
	}

	privKey, safe, err := s.keys.RetrieveMyBeaconPrivateKey(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve random beacon private key for epoch counter %v, at view %v, err: %w",
			epoch, view, err)
	}

	// if DKG failed, there will be no valid random beacon private key, since this fact
	// never change, we can cache a nil signer for this epoch, so that when this function
	// is called again for the same epoch, we don't need to query database.
	if !safe {
		s.privateKeys[epoch] = nil
		return nil, fmt.Errorf("DKG for epoch %v failed, at view %v: %w",
			epoch, view, module.DKGFailError)
	}

	// DKG succeeded and a random beacon key is available,
	// create a random beacon signer that holds the private key and cache it for the epoch
	s.privateKeys[epoch] = privKey

	return privKey, nil
}
