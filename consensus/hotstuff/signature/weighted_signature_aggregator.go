package signature

import (
	"fmt"
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
)

// signerInfo holds information about a signer, its weight and index
type signerInfo struct {
	weight uint64
	index  int
}

// WeightedSignatureAggregator implements consensus/hotstuff.WeightedSignatureAggregator.
// It is a wrapper around signature.SignatureAggregatorSameMessage, which implements a
// mapping from node IDs (as used by HotStuff) to index-based addressing of authorized
// signers (as used by SignatureAggregatorSameMessage).
type WeightedSignatureAggregator struct {
	aggregator  *signature.SignatureAggregatorSameMessage // low level crypto BLS aggregator, agnostic of weights and flow IDs
	ids         flow.IdentityList                         // all possible ids (only gets updated by constructor)
	idToInfo    map[flow.Identifier]signerInfo            // auxiliary map to lookup signer weight and index by ID (only gets updated by constructor)
	totalWeight uint64                                    // weight collected (gets updated)
	lock        sync.RWMutex                              // lock for atomic updates to totalWeight and collectedIDs

	// collectedIDs tracks the Identities of all nodes whose signatures have been collected so far.
	// The reason for tracking the duplicate signers at this module level is that having no duplicates
	// is a Hotstuff constraint, rather than a cryptographic aggregation constraint. We are planning to
	// extend the cryptographic primitives to support multiplicity higher than 1 in the future.
	// Therefore, we already add the logic for identifying duplicates here.
	collectedIDs map[flow.Identifier]struct{} // map of collected IDs (gets updated)
}

var _ hotstuff.WeightedSignatureAggregator = (*WeightedSignatureAggregator)(nil)

// NewWeightedSignatureAggregator returns a weighted aggregator initialized with a list of flow
// identities, their respective public keys, a message and a domain separation tag. The identities
// represent the list of all possible signers.
// The constructor errors if:
// - the list of identities is empty
// - if the length of keys does not match the length of identities
// - if one of the keys is not a valid public key.
//
// A weighted aggregator is used for one aggregation only. A new instance should be used for each
// signature aggregation task in the protocol.
func NewWeightedSignatureAggregator(
	ids flow.IdentityList, // list of all authorized signers
	pks []crypto.PublicKey, // list of corresponding public keys used for signature verifications
	message []byte, // message to get an aggregated signature for
	dsTag string, // domain separation tag used by the signature
) (*WeightedSignatureAggregator, error) {
	if len(ids) != len(pks) {
		return nil, fmt.Errorf("keys length %d and identities length %d do not match", len(pks), len(ids))
	}

	// build the internal map for a faster look-up
	idToInfo := make(map[flow.Identifier]signerInfo)
	for i, id := range ids {
		idToInfo[id.NodeID] = signerInfo{
			weight: id.Weight,
			index:  i,
		}
	}

	// instantiate low-level crypto aggregator, which works based on signer indices instead of nodeIDs
	agg, err := signature.NewSignatureAggregatorSameMessage(message, dsTag, pks)
	if err != nil {
		return nil, fmt.Errorf("instantiating index-based signature aggregator failed: %w", err)
	}

	return &WeightedSignatureAggregator{
		aggregator:   agg,
		ids:          ids,
		idToInfo:     idToInfo,
		collectedIDs: make(map[flow.Identifier]struct{}),
	}, nil
}

// Verify verifies the signature under the stored public keys and message.
// Expected errors during normal operations:
//  - model.InvalidSignerError if signerID is invalid (not a consensus participant)
//  - model.ErrInvalidSignature if signerID is valid but signature is cryptographically invalid
// The function is thread-safe.
func (w *WeightedSignatureAggregator) Verify(signerID flow.Identifier, sig crypto.Signature) error {
	info, ok := w.idToInfo[signerID]
	if !ok {
		return model.NewInvalidSignerErrorf("%v is not an authorized signer", signerID)
	}

	ok, err := w.aggregator.Verify(info.index, sig) // no error expected during normal operation
	if err != nil {
		return fmt.Errorf("couldn't verify signature from %s: %w", signerID, err)
	}
	if !ok {
		return fmt.Errorf("invalid signature from %s: %w", signerID, model.ErrInvalidSignature)
	}
	return nil
}

// TrustedAdd adds a signature to the internal set of signatures and adds the signer's
// weight to the total collected weight, iff the signature is _not_ a duplicate.
//
// The total weight of all collected signatures (excluding duplicates) is returned regardless
// of any returned error.
// The function errors with:
//  - model.InvalidSignerError if signerID is invalid (not a consensus participant)
//  - model.DuplicatedSignerError if the signer has been already added
// The function is thread-safe.
func (w *WeightedSignatureAggregator) TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (uint64, error) {
	info, found := w.idToInfo[signerID]
	if !found {
		return w.TotalWeight(), model.NewInvalidSignerErrorf("%v is not an authorized signer", signerID)
	}

	// atomically update the signatures pool and the total weight
	w.lock.Lock()
	defer w.lock.Unlock()

	// check for repeated occurrence of signerID (in anticipation of aggregator supporting multiplicities larger than 1 in the future)
	if _, duplicate := w.collectedIDs[signerID]; duplicate {
		return w.totalWeight, model.NewDuplicatedSignerErrorf("signature from %v was already added", signerID)
	}

	err := w.aggregator.TrustedAdd(info.index, sig)
	if err != nil {
		// During normal operations, signature.InvalidSignerIdxError or signature.DuplicatedSignerIdxError should never occur.
		return w.totalWeight, fmt.Errorf("unexpected exception while trusted add of signature from %v: %w", signerID, err)
	}
	w.totalWeight += info.weight
	w.collectedIDs[signerID] = struct{}{}

	return w.totalWeight, nil
}

// TotalWeight returns the total weight presented by the collected signatures.
// The function is thread-safe
func (w *WeightedSignatureAggregator) TotalWeight() uint64 {
	w.lock.RLock()
	defer w.lock.RUnlock()
	return w.totalWeight
}

// Aggregate aggregates the signatures and returns the aggregated signature.
// The function performs a final verification and errors if the aggregated signature is not valid. This is
// required for the function safety since "TrustedAdd" allows adding invalid signatures.
// The function errors with:
//  - model.InsufficientSignaturesError if no signatures have been added yet
//  - model.InvalidSignatureIncludedError if some signature(s), included via TrustedAdd, are invalid
// The function is thread-safe.
//
// TODO : When compacting the list of signers, update the return from []flow.Identifier
//        to a compact bit vector.
func (w *WeightedSignatureAggregator) Aggregate() ([]flow.Identifier, []byte, error) {
	w.lock.Lock()
	defer w.lock.Unlock()

	// Aggregate includes the safety check of the aggregated signature
	indices, aggSignature, err := w.aggregator.Aggregate()
	if err != nil {
		if signature.IsInsufficientSignaturesError(err) {
			return nil, nil, model.NewInsufficientSignaturesError(err)
		}
		if signature.IsInvalidSignatureIncludedError(err) {
			return nil, nil, model.NewInvalidSignatureIncludedError(err)
		}
		return nil, nil, fmt.Errorf("unexpected error during signature aggregation: %w", err)
	}
	signerIDs := make([]flow.Identifier, 0, len(indices))
	for _, index := range indices {
		signerIDs = append(signerIDs, w.ids[index].NodeID)
	}

	return signerIDs, aggSignature, nil
}
