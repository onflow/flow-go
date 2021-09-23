package signature

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
)

// signerInfo holds information about a signer, its stake and index
type signerInfo struct {
	weight uint64 //nolint:unused
	index  int
}

// WeightedSignatureAggregator implements consensus/hotstuff.WeightedSignatureAggregator
type WeightedSignatureAggregator struct {
	aggregator   *signature.SignatureAggregatorSameMessage // low level crypto aggregator, agnostic of weights and flow IDs
	signers      []flow.Identity                           //nolint:unused
	idToSigner   map[flow.Identifier]signerInfo
	totalWeight  uint64                       // weight collected
	lock         sync.RWMutex                 // lock for atomic updates
	collectedIDs map[flow.Identifier]struct{} // map of collected IDs
}

var _ hotstuff.WeightedSignatureAggregator = &WeightedSignatureAggregator{}

// NewWeightedSignatureAggregator returns a weighted aggregator initialized with a list of flow
// identities, a message and a domain separation tag. The identities represent the list of all
// possible signers.
//
// The constructor errors if the list of identities is empty, or if any identity doesn't
// hold a valid staking key.
//
// A weighted aggregator is used for one aggregation only. A new instance should be used for each use.
func NewWeightedSignatureAggregator(
	signers []flow.Identity, // list of all possible signers
	message []byte, // message to get an aggregated signature for
	dsTag string, // domain separation tag used by the signature
) (*WeightedSignatureAggregator, error) {

	// build a low level crypto aggregator
	publicKeys := make([]crypto.PublicKey, 0, len(signers))
	for _, id := range signers {
		publicKeys = append(publicKeys, id.StakingPubKey)
	}
	agg, err := signature.NewSignatureAggregatorSameMessage(message, dsTag, publicKeys)
	if err != nil {
		return nil, fmt.Errorf("new signature aggregator failed: %w", err)
	}

	// build the weighted aggregator
	weightedAgg := &WeightedSignatureAggregator{
		aggregator:   agg,
		signers:      signers,
		idToSigner:   make(map[flow.Identifier]signerInfo),
		collectedIDs: make(map[flow.Identifier]struct{}),
	}

	// build the internal map for a faster look-up
	for i, id := range signers {
		weightedAgg.idToSigner[id.NodeID] = signerInfo{
			weight: id.Stake,
			index:  i,
		}
	}
	return weightedAgg, nil
}

// Verify verifies the signature under the stored public and message.
//
// The function errors:
//  - engine.InvalidInputError if signerID is invalid (not a consensus participant)
//  - module/signature.ErrInvalidFormat if signerID is valid but signature is cryptographically invalid
//  - random error if the execution failed
// The function is thread-safe.
func (w *WeightedSignatureAggregator) Verify(signerID flow.Identifier, sig crypto.Signature) error {
	info, ok := w.idToSigner[signerID]
	if !ok {
		return engine.NewInvalidInputErrorf("couldn't find signerID %s in the index map", signerID)
	}

	ok, err := w.aggregator.Verify(info.index, sig)
	if err != nil {
		return fmt.Errorf("couldn't verify signature from %s: %w", signerID, err)
	}
	if !ok {
		return signature.ErrInvalidFormat
	}
	return nil
}

// hasSignature returs true if the input ID already included a signature
// and false otherwise.
// The function is thread safe.
func (w *WeightedSignatureAggregator) hasSignature(signerID flow.Identifier) bool {
	w.lock.RLock()
	defer w.lock.RUnlock()
	_, ok := w.collectedIDs[signerID]
	return ok
}

// TrustedAdd adds a signature to the internal set of signatures and adds the signer's
// weight to the total collected weight, iff the signature is _not_ a duplicate.
//
// The total weight of all collected signatures (excluding duplicates) is returned regardless
// of any returned error.
// The function errors
//  - engine.InvalidInputError if signerID is invalid (not a consensus participant)
//  - engine.DuplicatedEntryError if the signer has been already added
// The function is thread-safe.
func (w *WeightedSignatureAggregator) TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (uint64, error) {

	currentWeight := w.TotalWeight()

	info, found := w.idToSigner[signerID]
	if !found {
		return currentWeight, engine.NewInvalidInputErrorf("couldn't find signerID %s in the map", signerID)
	}

	if w.hasSignature(signerID) {
		return currentWeight, engine.NewDuplicatedEntryErrorf("SigneID %s was already added", signerID)
	}

	// atomically update the signatures pool and the total weight
	w.lock.Lock()
	defer w.lock.Unlock()

	err := w.aggregator.TrustedAdd(info.index, sig)
	if err != nil {
		return currentWeight, fmt.Errorf("Trusted add has failed: %w", err)
	}

	w.collectedIDs[signerID] = struct{}{}
	w.totalWeight += info.weight
	return w.totalWeight, nil
}

// TotalWeight returns the total weight presented by the collected signatures.
//
// The function is thread-safe
func (w *WeightedSignatureAggregator) TotalWeight() uint64 {
	w.lock.RLock()
	defer w.lock.RUnlock()
	return w.totalWeight
}

// Aggregate aggregates the signatures and returns the aggregated signature.
//
// The function is thread-safe.
// Aggregate attempts to aggregate the internal signatures and returns the resulting signature data.
// The function performs a final verification and errors if the aggregated signature is not valid. This is
// required for the function safety since "TrustedAdd" allows adding invalid signatures.
//
// TODO : When compacting the list of signers, update the return from []flow.Identifier
// to a compact bit vector.
func (w *WeightedSignatureAggregator) Aggregate() ([]flow.Identifier, []byte, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	// Aggregate includes the safety check of the aggregated signature
	indices, aggSignature, err := w.aggregator.Aggregate()
	if err != nil {
		return nil, nil, fmt.Errorf("aggregate has failed: %w", err)
	}
	signerIDs := make([]flow.Identifier, 0, len(indices))
	for _, index := range indices {
		signerIDs = append(signerIDs, w.signers[index].NodeID)
	}

	return signerIDs, aggSignature, nil
}
