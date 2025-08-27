package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestCollectionGuaranteeID_Malleability confirms that the CollectionGuarantee struct, which implements
// the [flow.IDEntity] interface, is resistant to tampering.
func TestCollectionGuaranteeID_Malleability(t *testing.T) {
	unittest.RequireEntityNonMalleable(t, unittest.CollectionGuaranteeFixture())
}

func TestNewCollectionGuarantee(t *testing.T) {
	t.Run("valid guarantee", func(t *testing.T) {
		ug := flow.UntrustedCollectionGuarantee{
			CollectionID:     unittest.IdentifierFixture(),
			ReferenceBlockID: unittest.IdentifierFixture(),
			ClusterChainID:   flow.Testnet,
			SignerIndices:    []byte{0, 1, 2},
			Signature:        unittest.SignatureFixture(),
		}

		guarantee, err := flow.NewCollectionGuarantee(ug)
		assert.NoError(t, err)
		assert.NotNil(t, guarantee)
	})

	t.Run("missing collection ID", func(t *testing.T) {
		ug := flow.UntrustedCollectionGuarantee{
			CollectionID:     flow.ZeroID,
			ReferenceBlockID: unittest.IdentifierFixture(),
			SignerIndices:    []byte{1},
		}

		guarantee, err := flow.NewCollectionGuarantee(ug)
		assert.Error(t, err)
		assert.Nil(t, guarantee)
		assert.Contains(t, err.Error(), "CollectionID")
	})

	t.Run("missing reference block ID", func(t *testing.T) {
		ug := flow.UntrustedCollectionGuarantee{
			CollectionID:     unittest.IdentifierFixture(),
			ReferenceBlockID: flow.ZeroID,
			SignerIndices:    []byte{1},
		}

		guarantee, err := flow.NewCollectionGuarantee(ug)
		assert.Error(t, err)
		assert.Nil(t, guarantee)
		assert.Contains(t, err.Error(), "ReferenceBlockID")
	})

	t.Run("missing signer indices", func(t *testing.T) {
		ug := flow.UntrustedCollectionGuarantee{
			CollectionID:     unittest.IdentifierFixture(),
			ReferenceBlockID: unittest.IdentifierFixture(),
			SignerIndices:    []byte{},
		}

		guarantee, err := flow.NewCollectionGuarantee(ug)
		assert.Error(t, err)
		assert.Nil(t, guarantee)
		assert.Contains(t, err.Error(), "SignerIndices")
	})
}

// BenchmarkBlockID benchmarks the time required to compute the ID of a reasonably sizable flow.Block
// as we could encounter it on Flow mainnet after the launch of Forte (Oct 2025).
// The block contains a payload with:
// - 50 CollectionGuarantees, produced by clusters with 40 nodes each
// - 2 Seals
// - 6 Receipts
// - 1 ExecutionResult
func BenchmarkBlockID(b *testing.B) {
	// Create 50 collection guarantees with 8-byte SignerIndices (instead of default 16-byte)
	guarantees := make([]*flow.CollectionGuarantee, 0, 50)
	for i := 0; i < len(guarantees); i++ {
		guarantees = append(guarantees, unittest.CollectionGuaranteeFixture(func(guarantee *flow.CollectionGuarantee) {
			// A collection guarantee is produced by a cluster of roughly 40 nodes.
			// In general, we need to encode, which of the n nodes signed the guarantee. In its most efficient form,
			// this can be represented by a bit string of n bits.
			guarantee.SignerIndices = unittest.RandomBytes(5) // 5 bytes -- represents the signer participation cluster of 40 nodes
		}))
	}

	// Create 2 seals
	seals := unittest.Seal.Fixtures(2)

	// Create 6 receipts
	receipts := make([]*flow.ExecutionReceipt, 0, 6)
	for i := 0; i < len(receipts); i++ {
		receipts = append(receipts, unittest.ExecutionReceiptFixture())
	}

	// Create 1 execution result
	results := []*flow.ExecutionResult{unittest.ExecutionResultFixture()}

	// Create payload with all components
	payload := unittest.PayloadFixture(
		unittest.WithSeals(seals...),
		unittest.WithGuarantees(guarantees...),
		unittest.WithReceipts(receipts...),
		unittest.WithExecutionResults(results...),
	)

	// Create block with the payload
	blockB := unittest.BlockFixture(unittest.Block.WithPayload(payload))

	// Reset timer to exclude setup time from benchmark
	b.ResetTimer()

	// Run the benchmark
	for i := 0; i < b.N; i++ {
		_ = blockB.ID()
	}
}
