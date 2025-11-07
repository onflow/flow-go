package systemcollection

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

func TestDefault(t *testing.T) {
	t.Run("returns versioned system collection for valid chain", func(t *testing.T) {
		versioned := Default(flow.Mainnet)
		require.NotNil(t, versioned)

		// Should be able to get builders by height
		builder := versioned.Get(0)
		require.NotNil(t, builder)
	})

	t.Run("returns versioned system collection for all supported chains", func(t *testing.T) {
		chains := []flow.ChainID{
			flow.Mainnet,
			flow.Testnet,
			flow.Sandboxnet,
			flow.Previewnet,
			flow.Benchnet,
			flow.Localnet,
			flow.Emulator,
			flow.BftTestnet,
			flow.MonotonicEmulator,
		}

		for _, chainID := range chains {
			versioned := Default(chainID)
			require.NotNil(t, versioned, "should return versioned for chain %s", chainID)
		}
	})
}

func TestNewVersioned(t *testing.T) {
	t.Run("successfully creates Versioned with valid inputs", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		versionedBuilder := Default(flow.Emulator)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)
		require.NotNil(t, versioned)

		assert.Equal(t, chain, versioned.chain)
		assert.NotNil(t, versioned.transactions)
		assert.NotNil(t, versioned.versioned)
	})

	t.Run("caches all system transactions from all versions", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		versionedBuilder := Default(flow.Emulator)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		// Should have cached transactions from all versions
		assert.NotEmpty(t, versioned.transactions, "should cache system transactions")

		// Each version should contribute transactions
		// V0 and V1 both have process + system chunk = at least 2 transactions per version
		assert.GreaterOrEqual(t, len(versioned.transactions), 2, "should have at least 2 transactions cached")
	})

	t.Run("all cached transactions have unique IDs", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		versionedBuilder := Default(flow.Emulator)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		// Verify all IDs are unique (map keys guarantee this, but we verify content)
		seenIDs := make(map[flow.Identifier]bool)
		for id := range versioned.transactions {
			assert.False(t, seenIDs[id], "transaction ID %s should be unique", id)
			seenIDs[id] = true
		}
	})
}

func TestVersioned_SearchAll(t *testing.T) {
	t.Run("finds transaction by ID from any version", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		versionedBuilder := Default(flow.Emulator)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		// Get a transaction ID that should exist
		require.NotEmpty(t, versioned.transactions, "should have cached transactions")

		var testID flow.Identifier
		var expectedTx *flow.TransactionBody
		for id, tx := range versioned.transactions {
			testID = id
			expectedTx = tx
			break
		}

		// Search for the transaction
		foundTx, ok := versioned.SearchAll(testID)
		assert.True(t, ok, "should find transaction by ID")
		assert.Equal(t, expectedTx, foundTx, "should return correct transaction")
	})

	t.Run("returns false for non-existent transaction ID", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		versionedBuilder := Default(flow.Emulator)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		// Use a random ID that shouldn't exist
		nonExistentID := flow.Identifier{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

		foundTx, ok := versioned.SearchAll(nonExistentID)
		assert.False(t, ok, "should not find non-existent transaction")
		assert.Nil(t, foundTx, "should return nil for non-existent transaction")
	})

	t.Run("finds system chunk transaction", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		versionedBuilder := Default(flow.Emulator)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		// Get the system chunk transaction from V0
		builder := &builderV0{}
		systemChunkTx, err := builder.SystemChunkTransaction(chain)
		require.NoError(t, err)

		// Should be able to find it in the versioned cache
		foundTx, ok := versioned.SearchAll(systemChunkTx.ID())
		assert.True(t, ok, "should find system chunk transaction")
		assert.Equal(t, systemChunkTx.ID(), foundTx.ID(), "should return correct system chunk transaction")
	})

	t.Run("finds process callbacks transaction", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		versionedBuilder := Default(flow.Emulator)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		// Get the process callbacks transaction from V0
		builder := &builderV0{}
		processTx, err := builder.ProcessCallbacksTransaction(chain)
		require.NoError(t, err)

		// Should be able to find it in the versioned cache
		foundTx, ok := versioned.SearchAll(processTx.ID())
		assert.True(t, ok, "should find process callbacks transaction")
		assert.Equal(t, processTx.ID(), foundTx.ID(), "should return correct process callbacks transaction")
	})
}

func TestVersioned_ByHeight(t *testing.T) {
	t.Run("returns V0 builder for height 0 on Mainnet", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		versionedBuilder := Default(flow.Mainnet)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		builder := versioned.ByHeight(0)
		require.NotNil(t, builder)

		// Verify it's V0 by checking the type
		_, isV0 := builder.(*builderV0)
		assert.True(t, isV0, "should return V0 builder for height 0")
	})

	t.Run("returns V0 builder for height < 200 on Mainnet", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		versionedBuilder := Default(flow.Mainnet)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		builder := versioned.ByHeight(100)
		require.NotNil(t, builder)

		// Verify it's V0 by checking the type
		_, isV0 := builder.(*builderV0)
		assert.True(t, isV0, "should return V0 builder for height < 200")
	})

	t.Run("returns V1 builder for height >= 200 on Mainnet", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		versionedBuilder := Default(flow.Mainnet)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		builder := versioned.ByHeight(200)
		require.NotNil(t, builder)

		// Verify it's V1 by checking the type
		_, isV1 := builder.(*builderV1)
		assert.True(t, isV1, "should return V1 builder for height >= 200")
	})

	t.Run("returns latest version for Emulator at any height", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		versionedBuilder := Default(flow.Emulator)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		// Emulator uses LatestBoundary, so all heights should use the latest version
		builder := versioned.ByHeight(0)
		require.NotNil(t, builder)

		builder = versioned.ByHeight(1000)
		require.NotNil(t, builder)
	})

	t.Run("returns consistent builder for same height", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		versionedBuilder := Default(flow.Mainnet)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		builder1 := versioned.ByHeight(100)
		builder2 := versioned.ByHeight(100)

		// Should return the same builder instance
		assert.Equal(t, builder1, builder2, "should return consistent builder for same height")
	})
}

func TestChainHeightVersions(t *testing.T) {
	t.Run("all chains have version mappings", func(t *testing.T) {
		chains := []flow.ChainID{
			flow.Mainnet,
			flow.Testnet,
			flow.Sandboxnet,
			flow.Previewnet,
			flow.Benchnet,
			flow.Localnet,
			flow.Emulator,
			flow.BftTestnet,
			flow.MonotonicEmulator,
		}

		for _, chainID := range chains {
			mapper, exists := ChainHeightVersions[chainID]
			assert.True(t, exists, "chain %s should have version mapping", chainID)
			assert.NotNil(t, mapper, "chain %s mapper should not be nil", chainID)
		}
	})

	t.Run("Mainnet and Testnet use explicit version boundaries", func(t *testing.T) {
		mainnetMapper := ChainHeightVersions[flow.Mainnet]
		testnetMapper := ChainHeightVersions[flow.Testnet]

		// Both should use V0 at height 0
		assert.Equal(t, Version0, mainnetMapper.GetVersion(0))
		assert.Equal(t, Version0, testnetMapper.GetVersion(0))

		// Both should use V1 at height 200
		assert.Equal(t, Version1, mainnetMapper.GetVersion(200))
		assert.Equal(t, Version1, testnetMapper.GetVersion(200))
	})

	t.Run("test networks use LatestBoundary", func(t *testing.T) {
		testNetworks := []flow.ChainID{
			flow.Sandboxnet,
			flow.Previewnet,
			flow.Benchnet,
			flow.Localnet,
			flow.Emulator,
			flow.BftTestnet,
			flow.MonotonicEmulator,
		}

		for _, chainID := range testNetworks {
			mapper := ChainHeightVersions[chainID]
			// LatestBoundary maps height 0 to VersionLatest
			version := mapper.GetVersion(0)
			assert.Equal(t, access.VersionLatest, version, "chain %s should use VersionLatest", chainID)
		}
	})
}

func TestVersionBuilder(t *testing.T) {
	t.Run("has builders for all versions", func(t *testing.T) {
		expectedVersions := []access.Version{
			Version0,
			Version1,
			access.VersionLatest,
		}

		for _, version := range expectedVersions {
			builder, exists := versionBuilder[version]
			assert.True(t, exists, "should have builder for version %d", version)
			assert.NotNil(t, builder, "builder for version %d should not be nil", version)
		}
	})

	t.Run("Version0 maps to builderV0", func(t *testing.T) {
		builder := versionBuilder[Version0]
		_, isV0 := builder.(*builderV0)
		assert.True(t, isV0, "Version0 should map to builderV0")
	})

	t.Run("Version1 maps to builderV1", func(t *testing.T) {
		builder := versionBuilder[Version1]
		_, isV1 := builder.(*builderV1)
		assert.True(t, isV1, "Version1 should map to builderV1")
	})

	t.Run("VersionLatest maps to builderV1", func(t *testing.T) {
		builder := versionBuilder[access.VersionLatest]
		_, isV1 := builder.(*builderV1)
		assert.True(t, isV1, "VersionLatest should map to builderV1")
	})
}

func TestVersioned_Integration(t *testing.T) {
	t.Run("can construct system collections for different heights", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		versionedBuilder := Default(flow.Mainnet)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		// Test V0 at height 0
		builderV0 := versioned.ByHeight(0)
		collectionV0, err := builderV0.SystemCollection(chain, nil)
		require.NoError(t, err)
		require.NotNil(t, collectionV0)
		assert.NotEmpty(t, collectionV0.Transactions, "V0 should have transactions")

		// Test V1 at height 200
		builderV1 := versioned.ByHeight(200)
		collectionV1, err := builderV1.SystemCollection(chain, nil)
		require.NoError(t, err)
		require.NotNil(t, collectionV1)
		assert.NotEmpty(t, collectionV1.Transactions, "V1 should have transactions")
	})

	t.Run("system collections have consistent structure", func(t *testing.T) {
		chain := flow.Emulator.Chain()
		versionedBuilder := Default(flow.Emulator)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		builder := versioned.ByHeight(0)
		collection, err := builder.SystemCollection(chain, nil)
		require.NoError(t, err)

		// System collection should have at least 2 transactions (process + system chunk)
		assert.GreaterOrEqual(t, len(collection.Transactions), 2,
			"system collection should have at least process and system chunk transactions")

		// All transactions should have valid IDs
		for i, tx := range collection.Transactions {
			assert.NotEqual(t, flow.ZeroID, tx.ID(),
				"transaction %d should have non-zero ID", i)
		}
	})
}
