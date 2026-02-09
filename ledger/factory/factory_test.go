package factory

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
	"github.com/onflow/flow-go/ledger/remote"
	"github.com/onflow/flow-go/module/metrics"
)

// TestRemoteLedgerClient creates a local ledger and a remote ledger client,
// and tests that they behave identically for various operations.
func TestRemoteLedgerClient(t *testing.T) {
	withLedgerPair(t, func(localLedger, remoteLedger ledger.Ledger) {

		t.Run("InitialState", func(t *testing.T) {
			localState := localLedger.InitialState()
			remoteState := remoteLedger.InitialState()

			// Both should return the same initial state
			assert.Equal(t, localState, remoteState, "InitialState should be the same for local and remote ledger")
			assert.NotEqual(t, ledger.DummyState, localState)
			assert.NotEqual(t, ledger.DummyState, remoteState)
		})

		t.Run("HasState", func(t *testing.T) {
			localInitialState := localLedger.InitialState()
			remoteInitialState := remoteLedger.InitialState()

			// Both should have the same initial state
			assert.Equal(t, localInitialState, remoteInitialState)

			localHasState := localLedger.HasState(localInitialState)
			remoteHasState := remoteLedger.HasState(remoteInitialState)
			assert.Equal(t, localHasState, remoteHasState, "HasState should return the same result for local and remote ledger")
			assert.True(t, localHasState)

			// Test with non-existent state
			dummyState := ledger.DummyState
			localHasState = localLedger.HasState(dummyState)
			remoteHasState = remoteLedger.HasState(dummyState)
			assert.Equal(t, localHasState, remoteHasState, "HasState for non-existent state should return the same result")
			assert.False(t, localHasState)
		})

		t.Run("GetSingleValue", func(t *testing.T) {
			localInitialState := localLedger.InitialState()
			remoteInitialState := remoteLedger.InitialState()
			assert.Equal(t, localInitialState, remoteInitialState)

			// Create a test key
			key := ledger.NewKey([]ledger.KeyPart{
				ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner")),
				ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key")),
			})

			localQuery, err := ledger.NewQuerySingleValue(localInitialState, key)
			require.NoError(t, err)
			remoteQuery, err := ledger.NewQuerySingleValue(remoteInitialState, key)
			require.NoError(t, err)

			localValue, err := localLedger.GetSingleValue(localQuery)
			require.NoError(t, err)
			remoteValue, err := remoteLedger.GetSingleValue(remoteQuery)
			require.NoError(t, err)

			// Both should return the same value
			assert.Equal(t, localValue, remoteValue, "GetSingleValue should return the same value for local and remote ledger")
			assert.Equal(t, ledger.Value([]byte{}), localValue)
			assert.Equal(t, 0, len(localValue))
		})

		t.Run("Get", func(t *testing.T) {
			localInitialState := localLedger.InitialState()
			remoteInitialState := remoteLedger.InitialState()
			assert.Equal(t, localInitialState, remoteInitialState)

			// Create test keys
			keys := []ledger.Key{
				ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner1")),
					ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key1")),
				}),
				ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner2")),
					ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key2")),
				}),
			}

			localQuery, err := ledger.NewQuery(localInitialState, keys)
			require.NoError(t, err)
			remoteQuery, err := ledger.NewQuery(remoteInitialState, keys)
			require.NoError(t, err)

			localValues, err := localLedger.Get(localQuery)
			require.NoError(t, err)
			remoteValues, err := remoteLedger.Get(remoteQuery)
			require.NoError(t, err)

			// Both should return the same values
			require.Len(t, localValues, 2)
			require.Len(t, remoteValues, 2)
			assert.Equal(t, localValues, remoteValues, "Get should return the same values for local and remote ledger")
			assert.Equal(t, ledger.Value([]byte{}), localValues[0])
			assert.Equal(t, 0, len(localValues[0]))
		})

		t.Run("Set", func(t *testing.T) {
			localInitialState := localLedger.InitialState()
			remoteInitialState := remoteLedger.InitialState()
			assert.Equal(t, localInitialState, remoteInitialState)

			// Create test keys and values
			keys := []ledger.Key{
				ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner")),
					ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key-non-empty")),
				}),
				ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner-1")),
					ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key-empty-slice")),
				}),
				ledger.NewKey([]ledger.KeyPart{
					ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner-2")),
					ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key-nil")),
				}),
			}
			values := []ledger.Value{
				ledger.Value("test-value"),
				ledger.Value([]byte{}),
				ledger.Value(nil),
			}

			localUpdate, err := ledger.NewUpdate(localInitialState, keys, values)
			require.NoError(t, err)
			remoteUpdate, err := ledger.NewUpdate(remoteInitialState, keys, values)
			require.NoError(t, err)

			localNewState, localTrieUpdate, err := localLedger.Set(localUpdate)
			require.NoError(t, err)
			remoteNewState, remoteTrieUpdate, err := remoteLedger.Set(remoteUpdate)
			require.NoError(t, err)

			// Both should return the same new state
			assert.Equal(t, localNewState, remoteNewState, "Set should return the same new state for local and remote ledger")
			assert.NotEqual(t, ledger.DummyState, localNewState)
			assert.NotEqual(t, localInitialState, localNewState)

			// Both should return non-nil trie updates
			assert.NotNil(t, localTrieUpdate)
			assert.NotNil(t, remoteTrieUpdate)

			// Verify that both trie updates produce identical CBOR encodings
			// This ensures that nil vs empty slice distinction is preserved correctly
			// through the remote ledger's protobuf encoding/decoding cycle
			localTrieUpdateCBOR := ledger.EncodeTrieUpdateCBOR(localTrieUpdate)
			remoteTrieUpdateCBOR := ledger.EncodeTrieUpdateCBOR(remoteTrieUpdate)
			assert.Equal(t, localTrieUpdateCBOR, remoteTrieUpdateCBOR,
				"Trie updates must produce identical CBOR encodings. "+
					"Local CBOR length: %d, Remote CBOR length: %d",
				len(localTrieUpdateCBOR), len(remoteTrieUpdateCBOR))

			// Verify we can read back the value from both
			localQuery, err := ledger.NewQuerySingleValue(localNewState, keys[0])
			require.NoError(t, err)
			remoteQuery, err := ledger.NewQuerySingleValue(remoteNewState, keys[0])
			require.NoError(t, err)

			localValue, err := localLedger.GetSingleValue(localQuery)
			require.NoError(t, err)
			remoteValue, err := remoteLedger.GetSingleValue(remoteQuery)
			require.NoError(t, err)

			// Both should return the same value
			assert.Equal(t, localValue, remoteValue, "GetSingleValue after Set should return the same value")
			assert.Equal(t, ledger.Value("test-value"), localValue)
		})

		t.Run("Prove", func(t *testing.T) {
			localInitialState := localLedger.InitialState()
			remoteInitialState := remoteLedger.InitialState()
			assert.Equal(t, localInitialState, remoteInitialState)

			// Create test key
			key := ledger.NewKey([]ledger.KeyPart{
				ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner")),
				ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key")),
			})

			localQuery, err := ledger.NewQuery(localInitialState, []ledger.Key{key})
			require.NoError(t, err)
			remoteQuery, err := ledger.NewQuery(remoteInitialState, []ledger.Key{key})
			require.NoError(t, err)

			localProof, err := localLedger.Prove(localQuery)
			require.NoError(t, err)
			remoteProof, err := remoteLedger.Prove(remoteQuery)
			require.NoError(t, err)

			// Both should return proofs of the same length
			assert.NotNil(t, localProof)
			assert.NotNil(t, remoteProof)
			assert.Equal(t, len(localProof), len(remoteProof), "Prove should return proofs of the same length")
			assert.Greater(t, len(localProof), 0)
		})
	})
}

// TestTrieUpdatePayloadValueEquivalence verifies that trie updates with three different
// payload value representations (non-empty, empty slice, nil) produce the same result ID
// for both remote ledger and local ledger.
//
// This test ensures that:
// 1. State commitments match between local and remote ledgers for all three value types
// 2. Execution data IDs are identical across all three scenarios
// 3. The nil vs empty slice distinction is properly preserved through protobuf encoding
func TestTrieUpdatePayloadValueEquivalence(t *testing.T) {
	withLedgerPair(t, func(localLedger, remoteLedger ledger.Ledger) {

		// Get initial state (should be the same for both)
		localInitialState := localLedger.InitialState()
		remoteInitialState := remoteLedger.InitialState()
		require.Equal(t, localInitialState, remoteInitialState, "Initial states must match")

		// Create three test keys for the three different payload value types
		keys := []ledger.Key{
			ledger.NewKey([]ledger.KeyPart{
				ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner")),
				ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key-non-empty")),
			}),
			ledger.NewKey([]ledger.KeyPart{
				ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner")),
				ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key-empty-slice")),
			}),
			ledger.NewKey([]ledger.KeyPart{
				ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner")),
				ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key-nil")),
			}),
		}

		// Create a single update with three different payload value representations:
		// 1. Non-empty payload value
		// 2. Empty slice payload value
		// 3. Nil payload value
		values := []ledger.Value{
			ledger.Value([]byte{1, 2, 3}), // non-empty
			ledger.Value([]byte{}),        // empty slice
			ledger.Value(nil),             // nil
		}

		t.Logf("Creating single trie update with three payloads: non-empty, empty slice, and nil")

		// Create updates for both ledgers with all three values in a single update
		localUpdate, err := ledger.NewUpdate(localInitialState, keys, values)
		require.NoError(t, err, "Failed to create local update")

		remoteUpdate, err := ledger.NewUpdate(remoteInitialState, keys, values)
		require.NoError(t, err, "Failed to create remote update")

		// Apply the single update to both ledgers
		localNewState, localTrieUpdate, err := localLedger.Set(localUpdate)
		require.NoError(t, err, "Failed to apply local update")

		remoteNewState, remoteTrieUpdate, err := remoteLedger.Set(remoteUpdate)
		require.NoError(t, err, "Failed to apply remote update")

		// Verify state commitments match
		assert.Equal(t, localNewState, remoteNewState,
			"State commitments must match between local and remote ledger")
		assert.NotEqual(t, ledger.DummyState, localNewState,
			"State should not be dummy state")

		// Verify trie updates are not nil
		require.NotNil(t, localTrieUpdate, "Local trie update should not be nil")
		require.NotNil(t, remoteTrieUpdate, "Remote trie update should not be nil")

		// Verify both trie updates have the same number of payloads
		require.Equal(t, len(localTrieUpdate.Payloads), len(remoteTrieUpdate.Payloads),
			"Local and remote trie updates should have the same number of payloads")
		require.Equal(t, 3, len(localTrieUpdate.Payloads),
			"Trie update should contain exactly 3 payloads")

		t.Logf("Trie update contains %d payloads", len(localTrieUpdate.Payloads))
		t.Logf("Payload 0 (non-empty) value length: %d", len(localTrieUpdate.Payloads[0].Value()))
		t.Logf("Payload 1 (empty slice) value length: %d, is nil: %v", len(localTrieUpdate.Payloads[1].Value()), localTrieUpdate.Payloads[1].Value() == nil)
		t.Logf("Payload 2 (nil) value length: %d, is nil: %v", len(localTrieUpdate.Payloads[2].Value()), localTrieUpdate.Payloads[2].Value() == nil)

		// Create ChunkExecutionData from the local trie update
		collection := unittest.CollectionFixture(1)
		localChunkExecutionData := &execution_data.ChunkExecutionData{
			Collection:         &collection,
			Events:             flow.EventsList{},
			TrieUpdate:         localTrieUpdate,
			TransactionResults: []flow.LightTransactionResult{},
		}

		// Create ChunkExecutionData from the remote trie update
		remoteChunkExecutionData := &execution_data.ChunkExecutionData{
			Collection:         &collection,
			Events:             flow.EventsList{},
			TrieUpdate:         remoteTrieUpdate,
			TransactionResults: []flow.LightTransactionResult{},
		}

		// Create BlockExecutionData for both
		blockID := unittest.IdentifierFixture()
		localBlockExecutionData := &execution_data.BlockExecutionData{
			BlockID:             blockID,
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{localChunkExecutionData},
		}

		remoteBlockExecutionData := &execution_data.BlockExecutionData{
			BlockID:             blockID,
			ChunkExecutionDatas: []*execution_data.ChunkExecutionData{remoteChunkExecutionData},
		}

		// Calculate execution data IDs using the default serializer
		serializer := execution_data.DefaultSerializer
		ctx := context.Background()

		localExecutionDataID, err := execution_data.CalculateID(ctx, localBlockExecutionData, serializer)
		require.NoError(t, err, "Failed to calculate local execution data ID")

		remoteExecutionDataID, err := execution_data.CalculateID(ctx, remoteBlockExecutionData, serializer)
		require.NoError(t, err, "Failed to calculate remote execution data ID")

		// The key assertion: local and remote execution data IDs must match
		// This verifies that the remote ledger properly preserves the nil vs empty slice
		// distinction through protobuf encoding, ensuring deterministic CBOR serialization
		assert.Equal(t, localExecutionDataID, remoteExecutionDataID,
			"Execution data IDs must match between local and remote ledger. "+
				"Local ID: %s, Remote ID: %s",
			localExecutionDataID, remoteExecutionDataID)

		t.Logf("Test completed successfully.")
		t.Logf("State commitment: %s", localNewState)
		t.Logf("Execution data ID (local and remote match): %s", localExecutionDataID)
	})
}

// startLedgerServer starts a ledger server on a random port and returns the address and cleanup function.
func startLedgerServer(t *testing.T, walDir string) (string, func()) {
	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	require.NoError(t, err)
	addr := listener.Addr().String()
	listener.Close()

	logger := unittest.Logger()

	// Create WAL
	metricsCollector := &metrics.NoopCollector{}
	diskWal, err := wal.NewDiskWAL(
		logger,
		nil,
		metricsCollector,
		walDir,
		100,
		pathfinder.PathByteSize,
		wal.SegmentSize,
	)
	require.NoError(t, err)

	// Create compactor config
	compactorConfig := ledger.DefaultCompactorConfig(metricsCollector)

	// Create ledger factory
	factory := complete.NewLocalLedgerFactory(
		diskWal,
		100,
		compactorConfig,
		atomic.NewBool(false), // trigger checkpoint signal
		metricsCollector,
		logger,
		complete.DefaultPathFinderVersion,
	)

	// Create ledger instance
	ledgerStorage, err := factory.NewLedger()
	require.NoError(t, err)

	// Wait for ledger to be ready (WAL replay)
	<-ledgerStorage.Ready()

	// Create gRPC server with max message size configuration
	// Use large limits to match production defaults (1 GiB for both)
	grpcServer := grpc.NewServer(
		grpc.MaxRecvMsgSize(1<<30), // 1 GiB for requests
		grpc.MaxSendMsgSize(1<<30), // 1 GiB for responses
	)

	// Create and register ledger service
	ledgerService := remote.NewService(ledgerStorage, logger)
	ledgerpb.RegisterLedgerServiceServer(grpcServer, ledgerService)

	// Start gRPC server
	lis, err := net.Listen("tcp", addr)
	require.NoError(t, err)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			serverErr <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	// Cleanup function
	cleanup := func() {
		grpcServer.GracefulStop()
		<-ledgerStorage.Done()
	}

	return addr, cleanup
}

// withLedgerPair creates both a local and remote ledger instance, handles Ready/Done,
// and automatically cleans up resources after the test function completes.
// The temp directory is automatically cleaned up by t.TempDir().
func withLedgerPair(t *testing.T, fn func(localLedger, remoteLedger ledger.Ledger)) {
	// Create temporary directories for WALs
	tempDir := t.TempDir()
	remoteWalDir := filepath.Join(tempDir, "remote_wal")
	localWalDir := filepath.Join(tempDir, "local_wal")

	err := os.MkdirAll(remoteWalDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(localWalDir, 0755)
	require.NoError(t, err)

	// Start ledger server
	serverAddr, serverCleanup := startLedgerServer(t, remoteWalDir)

	logger := zerolog.Nop()
	metricsCollector := &metrics.NoopCollector{}

	// Create local ledger using factory
	localLedger, err := NewLedger(Config{
		Triedir:            localWalDir,
		MTrieCacheSize:     100,
		CheckpointDistance: 1000,
		CheckpointsToKeep:  10,
		MetricsRegisterer:  nil,
		WALMetrics:         metricsCollector,
		LedgerMetrics:      metricsCollector,
		Logger:             logger,
	}, atomic.NewBool(false))
	require.NoError(t, err)
	require.NotNil(t, localLedger)

	// Create remote client using factory
	remoteLedger, err := NewLedger(Config{
		LedgerServiceAddr: serverAddr,
		Logger:            logger,
	}, nil)
	require.NoError(t, err)
	require.NotNil(t, remoteLedger)

	// Wait for both to be ready
	<-localLedger.Ready()
	<-remoteLedger.Ready()

	// Ensure cleanup happens even if the test function panics
	defer func() {
		// Stop remote ledger
		<-remoteLedger.Done()

		// Stop local ledger (WAL cleanup is handled internally by the ledger)
		<-localLedger.Done()

		// Stop server
		serverCleanup()
	}()

	// Execute the test function with the ledgers
	fn(localLedger, remoteLedger)
}
