package signature

import (
	"errors"
	"fmt"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

type EpochAwareRandomBeaconSignerStore struct {
	epochLookup module.EpochLookup          // used to fetch epoch counter by view
	keys        storage.DKGKeys             // used to fetch DKG private key by epoch
	signers     map[uint64]module.MsgSigner // cache of signers by epoch
}

func NewEpochAwareRandomBeaconSignerStore(epochLookup module.EpochLookup, keys storage.DKGKeys) *EpochAwareRandomBeaconSignerStore {
	return &EpochAwareRandomBeaconSignerStore{
		epochLookup: epochLookup,
		keys:        keys,
		signers:     make(map[uint64]module.MsgSigner),
	}
}

// GetSigner returns the threshold-signer for signing objects at a
// given view. The view determines the epoch, which determines the DKG private
// key underlying the signer.
// It returns:
//  - (signer, nil) if DKG was completed in the epoch of the view, signer is not nil
//  - (nil, DKGIncompleteError) if DKG was not completed in the epoch of the view
//  - (nil, error) if there is any exception
func (s *EpochAwareRandomBeaconSignerStore) GetSigner(view uint64) (module.MsgSigner, error) {
	// fetching the epoch by view, if epoch is found, then DKG must have been completed
	epoch, err := s.epochLookup.EpochForViewWithFallback(view)
	if err != nil {
		return nil, fmt.Errorf("could not get epoch by view %v: %w", view, err)
	}

	// when DKG has completed,
	// 		1. if a node successfully generated the DKG key, the private key will be stored in database.
	//    2. if a node failed to generate the DKG key, we will save a record in database to indicate this
	//       node has no DKG key for this epoch
	// with the epoch, we can lookup my DKG key for the epoch. There are 3 cases:
	// 1. DKG has completed, and the private key is stored in database, and we can retrieve it (happy path)
	// 2. DKG has completed, but we failed to generate our private key, and we marked in the database
	// 		that there is no DKG key for this epoch
	// 3. DKG has completed, but we
	signer, ok := s.signers[epoch]
	if ok {
		// A nil signer means that we don't have a Random Beacon key for this epoch.
		if signer == nil {
			return nil, fmt.Errorf("did not complete DKG for epoch %v, at view %v: %w",
				epoch, view, module.DKGIncompleteError)
		}
		return signer, nil
	}

	privDKGData, hasDKGKey, err := s.keys.RetrieveMyDKGPrivateInfo(epoch)
	// this is an edge case where the epoch has determined, but the result of whether we have the DKG
	// private key info or not is not found in the database.
	// in this case, we will trigger as if we failed the DKG.
	if errors.Is(err, storage.ErrNotFound) {
		s.signers[epoch] = nil
		return nil, fmt.Errorf("DKG result not found in database for epoch %v, at view %v: %w",
			epoch, view, module.DKGIncompleteError)
	}

	if err != nil {
		return nil, fmt.Errorf("could not retrieve DKG private key for epoch counter %v, at view %v, err: %w",
			epoch, view, err)
	}

	// if DKG was not completed, there will be no DKG private key, since this fact
	// never change, we can cache a nil signer for this epoch, so that we this function
	// is called again for the same epoch, we don't need to query database.
	if !hasDKGKey {
		s.signers[epoch] = nil
		return nil, fmt.Errorf("didn't complete DKG for epoch %v, at view %v: %w",
			epoch, view, module.DKGIncompleteError)
	}

	// DKG was completed and a random beacon key is available,
	// create a random beacon signer that holds the private key and cache it for the epoch
	signer = NewRandomBeaconSigner(encoding.RandomBeaconTag, privDKGData.RandomBeaconPrivKey)
	s.signers[epoch] = signer

	return signer, nil
}
