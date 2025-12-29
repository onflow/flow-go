package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

// TestValidateAddress tests the Address validation function.
func TestValidateAddress(t *testing.T) {
	t.Parallel()

	// Use Testnet chain which is what AddressFixture uses
	chain := flow.Testnet.Chain()

	t.Run("valid address", func(t *testing.T) {
		t.Parallel()

		validAddr := unittest.AddressFixture()
		rawAddr := validAddr.Bytes()

		addr, err := convert.Address(rawAddr, chain)
		require.NoError(t, err)
		assert.Equal(t, validAddr, addr)
	})

	t.Run("empty address", func(t *testing.T) {
		t.Parallel()

		_, err := convert.Address([]byte{}, chain)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "address cannot be empty")
	})

	t.Run("invalid address for chain", func(t *testing.T) {
		t.Parallel()

		// Use an address from a different chain
		mainnetAddr := unittest.RandomAddressFixtureForChain(flow.Mainnet)
		rawAddr := mainnetAddr.Bytes()

		// This address should be invalid for Testnet chain
		_, err := convert.Address(rawAddr, chain)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "is invalid for chain")
	})
}

// TestValidateHexToAddress tests the HexToAddress validation function.
func TestValidateHexToAddress(t *testing.T) {
	t.Parallel()

	chain := flow.Testnet.Chain()

	t.Run("valid hex address", func(t *testing.T) {
		t.Parallel()

		validAddr := unittest.AddressFixture()
		hexAddr := validAddr.Hex()

		addr, err := convert.HexToAddress(hexAddr, chain)
		require.NoError(t, err)
		assert.Equal(t, validAddr, addr)
	})

	t.Run("empty hex address", func(t *testing.T) {
		t.Parallel()

		_, err := convert.HexToAddress("", chain)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "address cannot be empty")
	})

	t.Run("invalid hex address for chain", func(t *testing.T) {
		t.Parallel()

		// Use an address from a different chain
		mainnetAddr := unittest.RandomAddressFixtureForChain(flow.Mainnet)
		hexAddr := mainnetAddr.Hex()

		// This address should be invalid for Testnet chain
		_, err := convert.HexToAddress(hexAddr, chain)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "is invalid for chain")
	})
}

// TestValidateBlockID tests the BlockID validation function.
func TestValidateBlockID(t *testing.T) {
	t.Parallel()

	t.Run("valid block ID", func(t *testing.T) {
		t.Parallel()

		expectedID := unittest.IdentifierFixture()
		rawID := expectedID[:]

		blockID, err := convert.BlockID(rawID)
		require.NoError(t, err)
		assert.Equal(t, expectedID, blockID)
	})

	t.Run("invalid block ID length - too short", func(t *testing.T) {
		t.Parallel()

		invalidID := []byte{0x01, 0x02, 0x03}

		_, err := convert.BlockID(invalidID)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "invalid block id")
	})

	t.Run("invalid block ID length - too long", func(t *testing.T) {
		t.Parallel()

		invalidID := make([]byte, flow.IdentifierLen+1)

		_, err := convert.BlockID(invalidID)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "invalid block id")
	})

	t.Run("empty block ID", func(t *testing.T) {
		t.Parallel()

		_, err := convert.BlockID([]byte{})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

// TestValidateBlockIDs tests the BlockIDs validation function.
func TestValidateBlockIDs(t *testing.T) {
	t.Parallel()

	g := fixtures.NewGeneratorSuite()

	t.Run("valid random block IDs lists", func(t *testing.T) {
		t.Parallel()

		expectedIDs := g.Identifiers().List(g.Random().IntInRange(3, 8))
		rawIDs := make([][]byte, len(expectedIDs))
		for j, id := range expectedIDs {
			rawIDs[j] = id[:]
		}

		blockIDs, err := convert.BlockIDs(rawIDs)
		require.NoError(t, err)
		assert.Equal(t, []flow.Identifier(expectedIDs), blockIDs)
	})

	t.Run("empty block IDs list", func(t *testing.T) {
		t.Parallel()

		_, err := convert.BlockIDs([][]byte{})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "empty block ids")
	})

	t.Run("one invalid block ID in list", func(t *testing.T) {
		t.Parallel()

		validID := g.Identifiers().Fixture()
		invalidID := []byte{0x01, 0x02}

		rawIDs := [][]byte{validID[:], invalidID}

		_, err := convert.BlockIDs(rawIDs)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

// TestValidateCollectionID tests the CollectionID validation function.
func TestValidateCollectionID(t *testing.T) {
	t.Parallel()

	t.Run("valid collection ID", func(t *testing.T) {
		t.Parallel()

		expectedID := unittest.IdentifierFixture()
		rawID := expectedID[:]

		collectionID, err := convert.CollectionID(rawID)
		require.NoError(t, err)
		assert.Equal(t, expectedID, collectionID)
	})

	t.Run("empty collection ID", func(t *testing.T) {
		t.Parallel()

		_, err := convert.CollectionID([]byte{})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "invalid collection id")
	})

	t.Run("collection ID with different length", func(t *testing.T) {
		t.Parallel()

		shortID := []byte{0x01, 0x02}

		_, err := convert.CollectionID(shortID)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "invalid collection id")
	})
}

// TestValidateEventType tests the EventType validation function.
func TestValidateEventType(t *testing.T) {
	t.Parallel()

	t.Run("valid event type", func(t *testing.T) {
		t.Parallel()

		eventType := "A.0x1.MyContract.MyEvent"

		result, err := convert.EventType(eventType)
		require.NoError(t, err)
		assert.Equal(t, eventType, result)
	})

	t.Run("empty event type", func(t *testing.T) {
		t.Parallel()

		_, err := convert.EventType("")
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "invalid event type")
	})

	t.Run("whitespace only event type", func(t *testing.T) {
		t.Parallel()

		_, err := convert.EventType("   ")
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "invalid event type")
	})

	t.Run("event type with leading/trailing whitespace", func(t *testing.T) {
		t.Parallel()

		eventType := "  A.0x1.MyContract.MyEvent  "

		result, err := convert.EventType(eventType)
		require.NoError(t, err)
		// The function returns the original string, not trimmed
		assert.Equal(t, eventType, result)
	})
}

// TestValidateTransactionID tests the TransactionID validation function.
func TestValidateTransactionID(t *testing.T) {
	t.Parallel()

	t.Run("valid transaction ID", func(t *testing.T) {
		t.Parallel()

		expectedID := unittest.IdentifierFixture()
		rawID := expectedID[:]

		txID, err := convert.TransactionID(rawID)
		require.NoError(t, err)
		assert.Equal(t, expectedID, txID)
	})

	t.Run("empty transaction ID", func(t *testing.T) {
		t.Parallel()

		_, err := convert.TransactionID([]byte{})
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "invalid transaction id")
	})

	t.Run("transaction ID with different length", func(t *testing.T) {
		t.Parallel()

		shortID := []byte{0x01, 0x02}

		_, err := convert.TransactionID(shortID)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
		assert.Contains(t, err.Error(), "invalid transaction id")
	})
}
