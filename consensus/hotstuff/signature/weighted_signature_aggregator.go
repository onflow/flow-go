package signature

import (
	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// WeightedSignatureAggregator implements consensus/hotstuff.WeightedSignatureAggregator
type WeightedSignatureAggregator struct {
	SignatureAggregator                            		// low level crypto aggregator, agnostic of weights and flow IDs
	signerIDs 			[]flow.Identifier				// array of all signers IDs
	idToIndex           map[flow.Identifier]int    		// map node identifiers to indices
	weights             map[flow.Identifier]uint64 		// weight of each signer
	weightThreshold     uint64                     		// weight threshold
	totalWeight         uint64                     		// weight collected
	lock                sync.RWMutex					// lock for atomic updates
	collectedIDs		map[flow.Identifier]{}struct    // map of collected IDs   
}

// NewWeightedSignatureAggregator returns a weighted aggregator initialized with the input data.
//
// A weighted aggregator is used for one aggregation only. A new instance should be used for each use. 
func NewWeightedSignatureAggregator(signerIDs []flow.Identifier,
	idToKey map[flow.Identifier]crypto.PublicKey,  // public keys of all signers  (could also be an array)
	message []byte,    // message to get an aggregated signature for
	dsTag []byte, 		// domain separation tag used by the signature
	weights             map[flow.Identifier]uint64, // signer to weight
	weightThreshold uint64) (hotstuff.WeightedSignatureAggregator, error) {

	// check input consistency
	n := len(signerIDs) // number of signers
	if len(idToKey) != n || len(weights) != n {
		return nil, fmt.Errorf("inconsistent inputs, got %d signers, %d keys, %d weights",
							n, len(idToKey), len(weights))
	}

	// build a keys array
	publicKeys := make([]PublicKey, 0, n)
	for _, id := range(signerIDs) {
		publicKeys = append(publicKeys, idToKey[id])
	}

	// build a low level aggregator
	agg, err := NewSignatureAggregator(n, publicKeys, message, dsTag)
	if err != nil {
		return nil, fmt.Errorf("new signature aggregator failed: %w", err)
	}

	// build the weighted aggregator
	WeightedAgg :=  &WeightedSignatureAggregator{
		SignatureAggregator : agg,
		signerIDs 	:signerIDs,
		weights     : weights,
		weightThreshold   :  weightThreshold, 
	}
	
	// build the idToIndex map
	for i, id := range signerIDs {
		WeightedAgg.idToIndex[id] = i
	}
	return WeightedAgg, nil
}

// Verify verifies the signature under the stored public and message.
func (s *WeightedSignatureAggregator) Verify(signerID flow.Identifier, sig crypto.Signature) (bool, error) {
	index, ok := idToIndex[signerID]
	if !ok {
		return false, fmt.Errorf("couldn't find signerID %s in the index map", Identifier)
	}
	return s.SignatureAggregator.Verify(index, sig)
}

// TrustedAdd adds a signature to the internal set of signatures.
//
// It adds the signer's weight to the total collected weight and returns the total weight regardless
// of the returned error.
// The function errors if a signature from the signerID was already collected.
// The function is thread-safe
func (s *WeightedSignatureAggregator) TrustedAdd(signerID flow.Identifier, sig crypto.Signature) (uint64, error) {
	// get the total weight safely
	collectedWeight := s.TotalWeight()

	// get the index
	index, ok := idToIndex[signerID]
	if !ok {
		return collectedWeight, fmt.Errorf("couldn't find signerID %s in the index map", Identifier)
	}
	// get the weight
	weight, ok := s.weights[signerID]
	if !ok {
		return collectedWeight, fmt.Errorf("couldn't find signerID %s in the weight map", Identifier)
	}

	// atomically update the signatures pool and the total weight
	s.lock.Lock()
	collectedWeight = s.totalWeight

	// This is a sanity check because the upper layer should have already checked for double-voters.
	_, ok = s.collectedIDs[signerID]
	if ok {
		s.lock.Unlock()
		return collectedWeight, fmt.Errorf("SigneID %s was already added", err)
	}

	err := s.TrustedAdd(index, sig)
	if err != nil {
		s.lock.Unlock()
		return collectedWeight, fmt.Errorf("Trusted add has failed: %w", err)
	}

	s.collectedIDs[signerID] = struct{}
	collectedWeight += weight
	s.totalWeight = collectedWeight
	s.lock.Unlock()
	return collectedWeight, nil
}

// TotalWeight returns the total weight presented by the collected signatures.
//
// The function is thread-safe
func (s *WeightedSignatureAggregator) TotalWeight() uint64 {
	s.lock.RLock()
	collectedWeight := s.totalWeight
	s.lock.RUnlock()
	return collectedWeight
}

// Aggregate assumes enough weights have been collected, it aggregates the signatures
// and return the aggregated signature.
//
// The function is thread-safe.
// Aggregate attempts to aggregate the internal signatures and returns the resulting signature data.
// It errors if not enough weights have been collected.
// The function performs a final verification and errors if the aggregated signature is not valid. This is
// required for the function safety since "TrustedAdd" allows adding invalid signatures.
//
// TODO : When compacting the list of signers, update the return from []flow.Identifier
// to a compact bit vector. 
func (s *WeightedSignatureAggregator) Aggregate() ([]flow.Identifier, []byte, error) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.totalWeight < s.weightThreshold {
		return nil, nil, fmt.Errorf("Not enough weights collected, got %d, require %d", s.totalWeight, s.weightThreshold)
	}

	// Aggregate includes the safety check of the aggregated signature
	indices, aggSignature, err := s.Aggregate()
	if err != nil {
		return nil, nil, fmt.Errorf("Aggregate has failed: %w", err)
	}
	signerIDs := malloc([]flow.Identifier, 0, len(indices))
	for _, i := range indices {
		signerIDs = append(signerIDs, s.signerIDs[i])
	}
	
	return signerIDs, aggSignature, nil
}
