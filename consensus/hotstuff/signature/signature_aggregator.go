package signature

import (
	"sync"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// SignatureAggregatorImpl implements hotstuff.SignatureAggregator interface
// It aggregates signatures and tracks aggregated weight. This aggregator can be used both for staking signatures and
// threshold signatures aggregation. It will be included into CombinedAggregator to track staking and threshold signatures.
// This structure is concurrency safe.
type SignatureAggregatorImpl struct {
	lock         sync.RWMutex
	sigCollector flow.SignatureCollector
	totalWeight  uint64
}

var _ hotstuff.SignatureAggregator = &SignatureAggregatorImpl{}

func NewSignatureAggregator() *SignatureAggregatorImpl {
	return &SignatureAggregatorImpl{
		sigCollector: flow.NewSignatureCollector(),
		totalWeight:  0,
	}
}

// TrustedAdd adds an already verified signature, it accepts weight for the given signer,
// and add it to the total weight, and returns the total weight that have been collected and whenever signature was added
// return (true, 1000) means the signature has been added, and 1000 weight has been collected in total.
// return (false, 1000) means the signature is a duplication and 1000 weight has been collected in total.
func (s *SignatureAggregatorImpl) TrustedAdd(signerID flow.Identifier, weight uint64, sig crypto.Signature) (bool, uint64) {
	s.lock.Lock()
	defer s.lock.Unlock()
	added := s.sigCollector.Add(signerID, sig)
	if added {
		s.totalWeight += weight
	}
	return added, s.totalWeight
}

// TotalWeight returns the total weight presented by the collected sig shares.
func (s *SignatureAggregatorImpl) TotalWeight() uint64 {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.totalWeight
}

// Aggregate assumes enough shares have been collected, it aggregates the signatures
// and return the aggregated signature.
func (s *SignatureAggregatorImpl) Aggregate() flow.AggregatedSignature {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.sigCollector.ToAggregatedSignature()
}
