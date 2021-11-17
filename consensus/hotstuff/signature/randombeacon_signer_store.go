package signature

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// EpochAwareRandomBeaconKeyStore provides an abstraction to query random beacon private key
// for a given view.
// Internally it indexes and caches the private keys by epoch.
type EpochAwareRandomBeaconKeyStore struct {
	epochLookup module.EpochLookup // used to fetch epoch counter by view
	keys        storage.DKGKeys    // used to fetch DKG private key by epoch

	mu          sync.RWMutex                 // to ensure concurrent read/write to privateKeys
	privateKeys map[uint64]crypto.PrivateKey // cache of privateKeys by epoch
}

func NewEpochAwareRandomBeaconKeyStore(epochLookup module.EpochLookup, keys storage.DKGKeys) *EpochAwareRandomBeaconKeyStore {
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
	// 1. if a node successfully generated the DKG key, the private key will be stored in database.
	// 2. if a node failed to generate the DKG key, we will save a record in database to indicate this
	//       node has no private key for this epoch
	// within the epoch, we can lookup my random beacon private key for the epoch. There are 3 cases:
	// 1. DKG has completed, and the private key is stored in database, and we can retrieve it (happy path)
	// 2. DKG has completed, but we failed it, and we marked in the database
	// 		that there is no private key for this epoch (unhappy path)
	// 3. DKG has completed, but for some reason we don't find the private key in the database (exception)
	// 4. DKG was not completed (fatal error, we should not run into with EECC, becauwe we still stay at
	// 		the current Epoch where DKG has completed.
	key, ok := s.readKey(epoch)
	if ok {
		// A nil key means that we don't have a Random Beacon key for this epoch.
		if key == nil {
			return nil, fmt.Errorf("DKG for epoch %v failed, at view %v: %w",
				epoch, view, module.DKGFailError)
		}
		return key, nil
	}

	privBeaconKeyData, hasRandomBeaconKey, err := s.keys.RetrieveMyDKGPrivateInfo(epoch)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve random beacon private key for epoch counter %v, at view %v, err: %w",
			epoch, view, err)
	}

	// if DKG was not completed, there will be no DKG private key, since this fact
	// never change, we can cache a nil signer for this epoch, so that we this function
	// is called again for the same epoch, we don't need to query database.
	if !hasRandomBeaconKey {
		s.writeKey(epoch, nil)
		return nil, fmt.Errorf("DKG for epoch %v failed, at view %v: %w",
			epoch, view, module.DKGFailError)
	}

	// DKG succeeded and a random beacon key is available,
	// create a random beacon signer that holds the private key and cache it for the epoch
	key = privBeaconKeyData.RandomBeaconPrivKey
	s.writeKey(epoch, key)

	return key, nil
}

func (s *EpochAwareRandomBeaconKeyStore) readKey(epoch uint64) (crypto.PrivateKey, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	key, found := s.privateKeys[epoch]
	return key, found
}

func (s *EpochAwareRandomBeaconKeyStore) writeKey(epoch uint64, key crypto.PrivateKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.privateKeys[epoch] = key
}
