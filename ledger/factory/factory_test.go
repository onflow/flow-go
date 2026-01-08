package factory

import (
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
	_ "google.golang.org/grpc/encoding/gzip" // required for gRPC compression

	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/deflate" // required for gRPC compression
	_ "github.com/onflow/flow-go/engine/common/grpc/compressor/snappy"  // required for gRPC compression
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/pathfinder"
	"github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal"
	ledgerpb "github.com/onflow/flow-go/ledger/protobuf"
	"github.com/onflow/flow-go/ledger/remote"
	"github.com/onflow/flow-go/module/metrics"
)

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
		metricsCollector,
		logger,
		complete.DefaultPathFinderVersion,
	)

	// Create ledger instance
	ledgerStorage, err := factory.NewLedger()
	require.NoError(t, err)

	// Wait for ledger to be ready (WAL replay)
	<-ledgerStorage.Ready()

	// Create gRPC server
	grpcServer := grpc.NewServer()

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

// TestRemoteLedgerClient creates a local ledger and a remote ledger client,
// and tests that they behave identically for various operations.
func TestRemoteLedgerClient(t *testing.T) {
	// Create temporary directories for WALs
	tempDir := t.TempDir()
	remoteWalDir := filepath.Join(tempDir, "remote_wal")
	localWalDir := filepath.Join(tempDir, "local_wal")

	err := os.MkdirAll(remoteWalDir, 0755)
	require.NoError(t, err)
	err = os.MkdirAll(localWalDir, 0755)
	require.NoError(t, err)

	// Start ledger server
	serverAddr, cleanup := startLedgerServer(t, remoteWalDir)
	defer cleanup()

	logger := zerolog.Nop()
	metricsCollector := &metrics.NoopCollector{}

	// Create local ledger using factory
	localResult, err := NewLedger(Config{
		Triedir:                              localWalDir,
		MTrieCacheSize:                       100,
		CheckpointDistance:                   1000,
		CheckpointsToKeep:                    10,
		TriggerCheckpointOnNextSegmentFinish: atomic.NewBool(false),
		MetricsRegisterer:                    nil,
		WALMetrics:                           metricsCollector,
		LedgerMetrics:                        metricsCollector,
		Logger:                               logger,
	})
	require.NoError(t, err)
	require.NotNil(t, localResult)
	localLedger := localResult.Ledger
	require.NotNil(t, localLedger)
	defer func() {
		if localResult.WAL != nil {
			<-localResult.WAL.Done()
		}
		<-localLedger.Done()
	}()

	// Create remote client using factory
	remoteResult, err := NewLedger(Config{
		LedgerServiceAddr: serverAddr,
		Logger:            logger,
	})
	require.NoError(t, err)
	require.NotNil(t, remoteResult)
	remoteLedger := remoteResult.Ledger
	require.NotNil(t, remoteLedger)
	defer func() {
		<-remoteLedger.Done()
	}()

	// Wait for both to be ready
	<-localLedger.Ready()
	<-remoteLedger.Ready()

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
				ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key")),
			}),
		}
		values := []ledger.Value{
			ledger.Value("test-value"),
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
}
