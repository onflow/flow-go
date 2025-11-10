package systemcollection

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/systemcontracts"
	"github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest/fixtures"
)

const (
	// WARNING: these height should never change, if tests are failing, the solution
	// is not to change these heights.
	// TODO: set the actual height when it is determined.

	// testMainnetV1Height is the height at which Mainnet transitions to Version1.
	testMainnetV1Height = 200
	// testTestnetV1Height is the height at which Testnet transitions to Version1.
	testTestnetV1Height = 200
)

func TestDefault(t *testing.T) {
	t.Run("returns versioned system collection for valid chain", func(t *testing.T) {
		versioned := Default(flow.Mainnet)
		require.NotNil(t, versioned)

		builder := versioned.ByHeight(0)
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

func TestVersioned_SearchAll(t *testing.T) {
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

	t.Run("returns V0 builder for height < testMainnetVersion1Height on Mainnet", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		versionedBuilder := Default(flow.Mainnet)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		builder := versioned.ByHeight(testMainnetV1Height - 100)
		require.NotNil(t, builder)

		// Verify it's V0 by checking the type
		_, isV0 := builder.(*builderV0)
		assert.True(t, isV0, "should return V0 builder for height < testMainnetVersion1Height")
	})

	t.Run("returns V1 builder for height >= testMainnetVersion1Height on Mainnet", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		versionedBuilder := Default(flow.Mainnet)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		builder := versioned.ByHeight(testMainnetV1Height)
		require.NotNil(t, builder)

		// Verify it's V1 by checking the type
		_, isV1 := builder.(*builderV1)
		assert.True(t, isV1, "should return V1 builder for height >= testMainnetVersion1Height")
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

		// Both should use V1 at their respective version boundaries
		assert.Equal(t, Version1, mainnetMapper.GetVersion(testMainnetV1Height))
		assert.Equal(t, Version1, testnetMapper.GetVersion(testTestnetV1Height))
	})

	t.Run("test transient networks use LatestBoundary", func(t *testing.T) {
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

		// Test V1 at testMainnetVersion1Height
		builderV1 := versioned.ByHeight(testMainnetV1Height)
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

// TestVersioned_SystemCollection tests that the latest builder version produces the expected system
// collection, and handles different success and failure scenarios.
// Note: we only test the latest, because the previous versions all passed this test at one point,
// and we statically check that the produced collection matches an expected ID. The intention is that
// past versions are never updated, so ongoing testing is not necessary.
func TestVersioned_SystemCollection(t *testing.T) {
	t.Parallel()

	chain := flow.Mainnet.Chain()
	g := fixtures.NewGeneratorSuite(fixtures.WithChainID(chain.ChainID()))

	// use the latest system collection builder
	systemCollections, err := NewVersioned(chain, Default(chain.ChainID()))
	require.NoError(t, err)
	latestBuilder := systemCollections.ByHeight(math.MaxUint64)

	tests := []struct {
		name            string
		events          []flow.Event
		expectedTxCount int
		errorMessage    string
	}{
		{
			name:            "no events",
			events:          []flow.Event{},
			expectedTxCount: 2, // process + system chunk
		},
		{
			name:            "single valid callback event",
			events:          []flow.Event{g.PendingExecutionEvents().Fixture()},
			expectedTxCount: 3, // process + execute + system chunk
		},
		{
			name: "multiple valid callback events",
			events: []flow.Event{
				g.PendingExecutionEvents().Fixture(),
				g.PendingExecutionEvents().Fixture(),
				g.PendingExecutionEvents().Fixture(),
			},
			expectedTxCount: 5, // process + 3 executes + system chunk
		},
		{
			name: "mixed events - valid callbacks and invalid types",
			events: []flow.Event{
				g.PendingExecutionEvents().Fixture(),
				createInvalidTypeEvent(g),
				g.PendingExecutionEvents().Fixture(),
				createInvalidTypeEvent(g),
			},
			expectedTxCount: 4, // process + 2 executes + system chunk
		},
		{
			name: "only invalid event types",
			events: []flow.Event{
				createInvalidTypeEvent(g),
				createInvalidTypeEvent(g),
			},
			expectedTxCount: 2, // process + system chunk
		},
		{
			name:         "invalid CCF payload in callback event",
			events:       []flow.Event{createInvalidPayloadEvent(g)},
			errorMessage: "failed to construct execute callbacks transactions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := systemcontracts.SystemContractsForChain(chain.ChainID()).ScheduledTransactionExecutor.Address

			collection, err := latestBuilder.SystemCollection(chain, access.StaticEventProvider(tt.events))
			if tt.errorMessage != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMessage)
				assert.Nil(t, collection)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, collection)

			transactions := collection.Transactions
			assert.Len(t, transactions, tt.expectedTxCount)

			if tt.expectedTxCount > 0 {
				// First transaction should always be the process transaction
				processTx := transactions[0]
				assert.NotNil(t, processTx)
				assert.NotEmpty(t, processTx.Script)
				assert.Equal(t, uint64(flow.DefaultMaxTransactionGasLimit), processTx.GasLimit)
				assert.Equal(t, []flow.Address{chain.ServiceAddress()}, processTx.Authorizers)
				assert.Empty(t, processTx.Arguments)

				// Last transaction should always be the system chunk transaction
				systemChunkTx := transactions[len(transactions)-1]
				assert.NotNil(t, systemChunkTx)
				assert.NotEmpty(t, systemChunkTx.Script)
				assert.Equal(t, []flow.Address{chain.ServiceAddress()}, systemChunkTx.Authorizers)

				// Middle transactions should be execute callback transactions
				executeCount := tt.expectedTxCount - 2 // subtract process and system chunk
				if executeCount > 0 {
					for i := 1; i < len(transactions)-1; i++ {
						executeTx := transactions[i]
						assert.NotNil(t, executeTx)
						assert.NotEmpty(t, executeTx.Script)
						assert.Equal(t, []flow.Address{executor}, executeTx.Authorizers)
						assert.Len(t, executeTx.Arguments, 1)
						assert.NotEmpty(t, executeTx.Arguments[0])
					}
				}
			}

			// Verify collection properties
			assert.NotEmpty(t, collection.ID())
			assert.Equal(t, len(transactions), len(collection.Transactions))
		})
	}
}

func createInvalidTypeEvent(g *fixtures.GeneratorSuite) flow.Event {
	event := g.PendingExecutionEvents().Fixture()
	event.Type = g.EventTypes().Fixture()
	return event
}

func createInvalidPayloadEvent(g *fixtures.GeneratorSuite) flow.Event {
	event := g.PendingExecutionEvents().Fixture()
	event.Payload = []byte("not valid ccf")
	return event
}
