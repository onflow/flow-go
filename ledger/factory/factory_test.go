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
	compactorConfig := &ledger.CompactorConfig{
		CheckpointCapacity:                   100,
		CheckpointDistance:                   100,
		CheckpointsToKeep:                    3,
		TriggerCheckpointOnNextSegmentFinish: atomic.NewBool(false),
		Metrics:                              metricsCollector,
	}

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

func TestRemoteLedgerClient(t *testing.T) {

	// Create temporary directory for WAL
	walDir := filepath.Join(t.TempDir(), "wal")
	err := os.MkdirAll(walDir, 0755)
	require.NoError(t, err)

	// Start ledger server
	serverAddr, cleanup := startLedgerServer(t, walDir)
	defer cleanup()

	// Create remote client using factory
	logger := zerolog.Nop()
	result, err := NewLedger(Config{
		LedgerServiceAddr: serverAddr,
		Logger:            logger,
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	remoteLedger := result.Ledger
	require.NotNil(t, remoteLedger)

	// Wait for client to be ready
	<-remoteLedger.Ready()

	t.Run("InitialState", func(t *testing.T) {
		state := remoteLedger.InitialState()
		assert.NotEqual(t, ledger.DummyState, state)
	})

	t.Run("HasState", func(t *testing.T) {
		initialState := remoteLedger.InitialState()
		hasState := remoteLedger.HasState(initialState)
		assert.True(t, hasState)

		// Test with non-existent state
		dummyState := ledger.DummyState
		hasState = remoteLedger.HasState(dummyState)
		assert.False(t, hasState)
	})

	t.Run("GetSingleValue", func(t *testing.T) {
		initialState := remoteLedger.InitialState()

		// Create a test key
		key := ledger.NewKey([]ledger.KeyPart{
			ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner")),
			ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key")),
		})

		query, err := ledger.NewQuerySingleValue(initialState, key)
		require.NoError(t, err)

		value, err := remoteLedger.GetSingleValue(query)
		require.NoError(t, err)
		// Empty ledger should return empty value
		assert.Equal(t, ledger.Value(nil), value)
	})

	t.Run("Get", func(t *testing.T) {
		initialState := remoteLedger.InitialState()

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

		query, err := ledger.NewQuery(initialState, keys)
		require.NoError(t, err)

		values, err := remoteLedger.Get(query)
		require.NoError(t, err)
		require.Len(t, values, 2)
		// Empty ledger should return empty values
		assert.Equal(t, ledger.Value(nil), values[0])
		assert.Equal(t, ledger.Value(nil), values[1])
	})

	t.Run("Set", func(t *testing.T) {
		initialState := remoteLedger.InitialState()

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

		update, err := ledger.NewUpdate(initialState, keys, values)
		require.NoError(t, err)

		newState, trieUpdate, err := remoteLedger.Set(update)
		require.NoError(t, err)
		assert.NotEqual(t, ledger.DummyState, newState)
		assert.NotEqual(t, initialState, newState)
		assert.NotNil(t, trieUpdate)

		// Verify we can read back the value
		query, err := ledger.NewQuerySingleValue(newState, keys[0])
		require.NoError(t, err)

		value, err := remoteLedger.GetSingleValue(query)
		require.NoError(t, err)
		assert.Equal(t, ledger.Value("test-value"), value)
	})

	t.Run("Prove", func(t *testing.T) {
		initialState := remoteLedger.InitialState()

		// Create test key
		key := ledger.NewKey([]ledger.KeyPart{
			ledger.NewKeyPart(ledger.KeyPartOwner, []byte("test-owner")),
			ledger.NewKeyPart(ledger.KeyPartKey, []byte("test-key")),
		})

		query, err := ledger.NewQuery(initialState, []ledger.Key{key})
		require.NoError(t, err)

		proof, err := remoteLedger.Prove(query)
		require.NoError(t, err)
		assert.NotNil(t, proof)
		assert.Greater(t, len(proof), 0)
	})

	// Cleanup client - Done() is called automatically by defer cleanup
	// but we can test it explicitly if needed
	<-remoteLedger.Done()
}
