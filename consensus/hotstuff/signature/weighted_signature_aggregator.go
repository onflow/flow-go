package signature

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
)

// WeightedSignatureAggregator implements consensus/hotstuff.WeightedSignatureAggregator
type WeightedSignatureAggregator struct {
	*signature.SignatureAggregatorSameMessage                              // low level crypto aggregator, agnostic of weights and flow IDs
	signers                                   []flow.Identity              // all possible signers, defining a canonical order
	idToIndex                                 map[flow.Identifier]int      // map node identifiers to indices
	idToWeights                               map[flow.Identifier]uint64   // weight of each signer
	totalWeight                               uint64                       // weight collected
	lock                                      sync.RWMutex                 // lock for atomic updates
	collectedIDs                              map[flow.Identifier]struct{} // map of collected IDs
}

// NewWeightedSignatureAggregator returns a weighted aggregator initialized with the input data.
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
		SignatureAggregatorSameMessage: agg,
		signers:                        signers,
	}

	// build the internal maps for a faster look-up
	for i, id := range signers {
		weightedAgg.idToIndex[id.NodeID] = i
		weightedAgg.idToWeights[id.NodeID] = id.Stake
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
	w.lock.RLock()
	index, ok := w.idToIndex[signerID]
	if !ok {
		return engine.NewInvalidInputErrorf("couldn't find signerID %s in the index map", signerID)
	}
	w.lock.RUnlock()

	ok, err := w.SignatureAggregatorSameMessage.Verify(index, sig)
	if err != nil {
		return fmt.Errorf("couldn't verify signature from %s: %w", signerID, err)
	}
	if !ok {
		return signature.ErrInvalidFormat
	}
	return nil
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

	// read data
	w.lock.RLock()

	// get the total weight safely
	collectedWeight := w.totalWeight

	// get index and weight
	index, idOk := w.idToIndex[signerID]
	weight, weightOk := w.idToWeights[signerID]
	newCollectedWeight := collectedWeight + weight

	if !(idOk && weightOk) {
		w.lock.RUnlock()
		return collectedWeight, engine.NewInvalidInputErrorf("couldn't find signerID %s in the map", signerID)
	}

	// check for double-voters.
	_, ok := w.collectedIDs[signerID]
	if ok {
		w.lock.RUnlock()
		return collectedWeight, engine.NewDuplicatedEntryErrorf("SigneID %s was already added", signerID)
	}
	w.lock.RUnlock()

	// atomically update the signatures pool and the total weight
	w.lock.Lock()
	defer w.lock.Unlock()

	err := w.SignatureAggregatorSameMessage.TrustedAdd(index, sig)
	if err != nil {
		return collectedWeight, fmt.Errorf("Trusted add has failed: %w", err)
	}

	w.collectedIDs[signerID] = struct{}{}
	w.totalWeight = newCollectedWeight
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
	indices, aggSignature, err := w.SignatureAggregatorSameMessage.Aggregate()
	if err != nil {
		return nil, nil, fmt.Errorf("Aggregate has failed: %w", err)
	}
	signerIDs := make([]flow.Identifier, 0, len(indices))
	for _, index := range indices {
		signerIDs = append(signerIDs, w.signers[index].NodeID)
	}

	return signerIDs, aggSignature, nil
}
