package systemcollection

import (
	"fmt"
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
	// WARNING: these heights should never change, if tests are failing, the solution
	// is not to change these heights.

	// testTestnetV1Height is the height at which Testnet transitions to Version1.
	testTestnetV1Height = 290050888
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
	// Define network version configurations
	// - For networks with explicit version boundaries: add heightTests map with height -> expected version
	// - For networks that always use latest: leave heightTests nil (empty)
	type networkConfig struct {
		chainID     flow.ChainID
		heightTests map[uint64]access.Version // height -> expected version (nil/empty = always latest)
	}

	networkConfigs := []networkConfig{
		// Testnet has explicit version boundaries
		{
			chainID: flow.Testnet,
			heightTests: map[uint64]access.Version{
				0:                         Version0,
				testTestnetV1Height:       Version1,
				testTestnetV1Height + 100: Version1,
			},
		},
		// Uncomment and configure when Mainnet boundaries are defined:
		// {
		// 	chainID: flow.Mainnet,
		// 	heightTests: map[uint64]access.Version{
		// 		0:                          Version0,
		// 		testMainnetV1Height - 100:  Version0,
		// 		testMainnetV1Height:        Version1,
		// 		testMainnetV1Height + 100:  Version1,
		// 	},
		// },

		// Networks that always use the latest version (nil heightTests = always latest)
		{chainID: flow.Mainnet},
		{chainID: flow.Emulator},
		{chainID: flow.Sandboxnet},
		{chainID: flow.Previewnet},
		{chainID: flow.Benchnet},
		{chainID: flow.Localnet},
		{chainID: flow.BftTestnet},
		{chainID: flow.MonotonicEmulator},
	}

	for _, config := range networkConfigs {
		t.Run(config.chainID.String(), func(t *testing.T) {
			chain := config.chainID.Chain()
			versionedBuilder := Default(config.chainID)

			versioned, err := NewVersioned(chain, versionedBuilder)
			require.NoError(t, err)

			// If heightTests is nil/empty, test that it always uses latest version
			if len(config.heightTests) == 0 {
				testHeights := []uint64{0, 1000}
				for _, height := range testHeights {
					t.Run(fmt.Sprintf("height_%d_returns_latest", height), func(t *testing.T) {
						builder := versioned.ByHeight(height)
						require.NotNil(t, builder)
						_, isV1 := builder.(*builderV1)
						assert.True(t, isV1, "should return latest builder (V1) at height %d", height)
					})
				}
				return
			}

			// Otherwise, test specific height boundaries
			for height, expectedVersion := range config.heightTests {
				t.Run(fmt.Sprintf("height_%d_returns_v%d", height, expectedVersion), func(t *testing.T) {
					builder := versioned.ByHeight(height)
					require.NotNil(t, builder)

					switch expectedVersion {
					case Version0:
						_, isV0 := builder.(*builderV0)
						assert.True(t, isV0, "should return V0 builder at height %d", height)
					case Version1:
						_, isV1 := builder.(*builderV1)
						assert.True(t, isV1, "should return V1 builder at height %d", height)
					default:
						t.Fatalf("unknown expected version: %d", expectedVersion)
					}
				})
			}
		})
	}
}

func TestChainHeightVersions(t *testing.T) {
	t.Run("Testnet has explicit version mapping", func(t *testing.T) {
		mapper, exists := ChainHeightVersions[flow.Testnet]
		assert.True(t, exists, "Testnet should have version mapping")
		assert.NotNil(t, mapper, "Testnet mapper should not be nil")
	})

	t.Run("Testnet uses explicit version boundaries", func(t *testing.T) {
		testnetMapper := ChainHeightVersions[flow.Testnet]

		// Testnet should use V0 at height 0
		assert.Equal(t, Version0, testnetMapper.GetVersion(0))

		// Testnet should use V1 at testTestnetV1Height
		assert.Equal(t, Version1, testnetMapper.GetVersion(testTestnetV1Height))
	})

	t.Run("chains without explicit mappings use LatestBoundary via Default", func(t *testing.T) {
		// These chains don't have explicit entries in ChainHeightVersions,
		// so Default() will give them LatestBoundary
		testNetworks := []flow.ChainID{
			flow.Mainnet,
			flow.Sandboxnet,
			flow.Previewnet,
			flow.Benchnet,
			flow.Localnet,
			flow.Emulator,
			flow.BftTestnet,
			flow.MonotonicEmulator,
		}

		for _, chainID := range testNetworks {
			versioned := Default(chainID)
			builder := versioned.ByHeight(0)
			_, isV1 := builder.(*builderV1)
			assert.True(t, isV1, "chain %s should use latest version (V1) at height 0", chainID)
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
	t.Run("can construct system collections for different heights on Testnet", func(t *testing.T) {
		chain := flow.Testnet.Chain()
		versionedBuilder := Default(flow.Testnet)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		// Test V0 at height 0
		builderV0 := versioned.ByHeight(0)
		collectionV0, err := builderV0.SystemCollection(chain, nil)
		require.NoError(t, err)
		require.NotNil(t, collectionV0)
		assert.Equal(t, len(collectionV0.Transactions), 2, "V0 should have 2 transactions")

		// Test V1 at testTestnetV1Height
		builderV1 := versioned.ByHeight(testTestnetV1Height)
		collectionV1, err := builderV1.SystemCollection(chain, nil)
		require.NoError(t, err)
		require.NotNil(t, collectionV1)
		assert.Equal(t, len(collectionV1.Transactions), 2, "V1 should have 2 transactions")
	})

	t.Run("Mainnet uses latest version for all heights", func(t *testing.T) {
		chain := flow.Mainnet.Chain()
		versionedBuilder := Default(flow.Mainnet)

		versioned, err := NewVersioned(chain, versionedBuilder)
		require.NoError(t, err)

		// Mainnet should use V1 (latest) at all heights
		builder := versioned.ByHeight(0)
		collection, err := builder.SystemCollection(chain, nil)
		require.NoError(t, err)
		require.NotNil(t, collection)
		assert.Equal(t, len(collection.Transactions), 2, "V1 should have 2 transactions")
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
