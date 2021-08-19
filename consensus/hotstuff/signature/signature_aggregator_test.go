package signature

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestSignatureAggregatorImpl_TrustedAdd tests that TrustedAdd adds only one signature from each signer will be collected
func TestSignatureAggregatorImpl_TrustedAdd(t *testing.T) {
	aggregator := NewSignatureAggregator()

	signerID := unittest.IdentifierFixture()
	weight := uint64(1000)
	added, totalWeight := aggregator.TrustedAdd(signerID, weight, unittest.SignatureFixture())
	require.True(t, added)
	require.Equal(t, totalWeight, weight)

	added, _ = aggregator.TrustedAdd(signerID, weight, unittest.SignatureFixture())
	require.False(t, added)
	added, totalWeight = aggregator.TrustedAdd(unittest.IdentifierFixture(), weight, unittest.SignatureFixture())
	require.True(t, added)
	require.Equal(t, totalWeight, weight*2)
}

// TestSignatureAggregatorImpl_TotalWeight test that SignatureAggregatorImpl correctly tracks weight of collected signatures
func TestSignatureAggregatorImpl_TotalWeight(t *testing.T) {
	aggregator := NewSignatureAggregator()
	weight := uint64(1000)
	signers := unittest.IdentifierListFixture(5)
	for _, signerID := range signers {
		_, totalWeight := aggregator.TrustedAdd(signerID, weight, unittest.SignatureFixture())
		require.Equal(t, totalWeight, aggregator.TotalWeight())
	}

	expectedWeight := weight * uint64(len(signers))
	require.Equal(t, expectedWeight, aggregator.TotalWeight())

	// when adding duplicated entry TotalWeight shouldn't be affected
	_, _ = aggregator.TrustedAdd(signers[0], weight, unittest.SignatureFixture())
	require.Equal(t, expectedWeight, aggregator.TotalWeight())
}

// TestSignatureAggregatorImpl_Aggregate tests that Aggregate returns valid aggregated signature
func TestSignatureAggregatorImpl_Aggregate(t *testing.T) {
	aggregator := NewSignatureAggregator()
	weight := uint64(1000)
	signers := unittest.IdentifierListFixture(5)
	expectedSig := flow.AggregatedSignature{}
	for _, signerID := range signers {
		sig := unittest.SignatureFixture()
		_, _ = aggregator.TrustedAdd(signerID, weight, sig)
		expectedSig.VerifierSignatures = append(expectedSig.VerifierSignatures, sig)
		expectedSig.SignerIDs = append(expectedSig.SignerIDs, signerID)
	}

	aggregatedSig := aggregator.Aggregate()
	require.Equal(t, expectedSig, aggregatedSig)
}

// TestSignatureAggregatorImpl_ConcurrentUsage tests that multiple goroutines can add signatures and then retrieve correct
// aggregated signature.
func TestSignatureAggregatorImpl_ConcurrentUsage(t *testing.T) {
	aggregator := NewSignatureAggregator()
	weight := uint64(1000)
	signers := unittest.IdentifierListFixture(100)
	signatures := unittest.SignaturesFixture(len(signers))
	expectedSig := flow.AggregatedSignature{}

	// build expected signatures
	for signerIndex, signerID := range signers {
		expectedSig.SignerIDs = append(expectedSig.SignerIDs, signerID)
		expectedSig.VerifierSignatures = append(expectedSig.VerifierSignatures, signatures[signerIndex])
	}

	workers := 5

	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for signerIndex, signerID := range signers {
				aggregator.TrustedAdd(signerID, weight, signatures[signerIndex])
			}
		}()
	}

	wg.Wait()

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			aggregated := aggregator.Aggregate()
			// in this scenario order can be different from expected sig
			require.ElementsMatch(t, expectedSig.VerifierSignatures, aggregated.VerifierSignatures)
			require.ElementsMatch(t, expectedSig.SignerIDs, aggregated.SignerIDs)
		}()
	}

	wg.Wait()
}
