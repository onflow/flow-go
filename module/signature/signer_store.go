package signature

import (
	"fmt"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// EpochAwareSignerStore implements the SignerStore interface. It is epoch
// aware, and provides the appropriate threshold signers on a per-view basis,
// using the database to retrieve the relevant DKG keys.
type EpochAwareSignerStore struct {
	state   protocol.State                    // used to fetch epoch counter by view
	keys    storage.DKGKeys                   // used to fetch DKG private key by epoch
	signers map[uint64]module.ThresholdSigner // cache of signers by epoch
}

// NewEpochAwareSignerStore instantiates a new EpochAwareSignerStore
func NewEpochAwareSignerStore(state protocol.State, keys storage.DKGKeys) *EpochAwareSignerStore {
	return &EpochAwareSignerStore{
		state:   state,
		keys:    keys,
		signers: make(map[uint64]module.ThresholdSigner),
	}
}

// GetSigner returns the threshold-signer for signing objects at a given view.
// The view determines the epoch, which determines the DKG private key
// underlying the signer.
func (s *EpochAwareSignerStore) GetSigner(view uint64) (module.ThresholdSigner, error) {
	epoch, err := s.epochForView(view)
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

// epochForView returns the counter of the epoch that the view belongs to.
// The protocol.State#Epochs object exposes previous, current, and next epochs,
// which should be all we need. In general we can't guarantee that a node will
// have access to epoch data beyond these three, so it is safe to throw an error
// for a query that doesn't fit within the view bounds of these three epochs
// (even if the node does happen to have that stored in the underlying storage)
// -- these queries indicate a bug in the querier.
func (s *EpochAwareSignerStore) epochForView(view uint64) (epochCounter uint64, err error) {
	current := s.state.Final().Epochs().Current()
	next := s.state.Final().Epochs().Next()
	previous := s.state.Final().Epochs().Previous()

	for _, epoch := range []protocol.Epoch{previous, current, next} {
		firstView, _ := epoch.FirstView()
		finalView, _ := epoch.FinalView()
		if firstView <= view && view <= finalView {
			return epoch.Counter()
		}
	}

	return 0, fmt.Errorf("couldn't get epoch for view %d", view)
}

/*~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~*/

// SingleSignerStore implements the SignerStore interface. It only keeps one
// signer and is not epoch-aware. It is used only for the bootstrapping process.
type SingleSignerStore struct {
	signer module.ThresholdSigner
}

// NewSingleSignerStore instantiates a new SingleSignerStore.
func NewSingleSignerStore(signer module.ThresholdSigner) *SingleSignerStore {
	return &SingleSignerStore{
		signer: signer,
	}
}

// GetSigner returns the signer.
func (s *SingleSignerStore) GetSigner(view uint64) (module.ThresholdSigner, error) {
	return s.signer, nil
}
