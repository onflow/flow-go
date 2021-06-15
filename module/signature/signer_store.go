package signature

import (
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/storage"
)

// EpochAwareSignerStore implements the SignerStore interface. It is epoch
// aware, and provides the appropriate threshold signers on a per-view basis,
// using the database to retrieve the relevant DKG keys.
type EpochAwareSignerStore struct {
	epochLookup module.EpochLookup                // used to fetch epoch counter by view
	keys        storage.DKGKeys                   // used to fetch DKG private key by epoch
	signers     map[uint64]module.ThresholdSigner // cache of signers by epoch
}

// NewEpochAwareSignerStore instantiates a new EpochAwareSignerStore
func NewEpochAwareSignerStore(epochLookup module.EpochLookup, keys storage.DKGKeys) *EpochAwareSignerStore {
	return &EpochAwareSignerStore{
		epochLookup: epochLookup,
		keys:        keys,
		signers:     make(map[uint64]module.ThresholdSigner),
	}
}

// GetThresholdSigner returns the threshold-signer for signing objects at a
// given view. The view determines the epoch, which determines the DKG private
// key underlying the signer.
func (s *EpochAwareSignerStore) GetThresholdSigner(view uint64) (module.ThresholdSigner, error) {
	epoch, err := s.epochLookup.EpochForView(view)
	if err != nil {
		return nil, err
	}
	signer, ok := s.signers[epoch]
	if ok {
		return signer, nil
	}
	privDKGData, err := s.keys.RetrieveMyDKGPrivateInfo(epoch)
	if err != nil {
		return nil, err
	}
	signer = NewThresholdProvider(encoding.RandomBeaconTag, privDKGData.RandomBeaconPrivKey)
	s.signers[epoch] = signer
	return signer, nil
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// SingleSignerStore implements the ThresholdSignerStore interface. It only
// keeps one signer and is not epoch-aware. It is used only for the
// bootstrapping process.
type SingleSignerStore struct {
	signer module.ThresholdSigner
}

// NewSingleSignerStore instantiates a new SingleSignerStore.
func NewSingleSignerStore(signer module.ThresholdSigner) *SingleSignerStore {
	return &SingleSignerStore{
		signer: signer,
	}
}

// GetThresholdSigner returns the signer.
func (s *SingleSignerStore) GetThresholdSigner(view uint64) (module.ThresholdSigner, error) {
	return s.signer, nil
}
