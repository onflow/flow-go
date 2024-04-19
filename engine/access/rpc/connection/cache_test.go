package connection

import (
	"net"
	"sync"
	"testing"
	"time"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestCachedClientShutdown(t *testing.T) {
	// Test that a completely uninitialized client can be closed without panics
	t.Run("uninitialized client", func(t *testing.T) {
		client := &CachedClient{
			closeRequested: atomic.NewBool(false),
		}
		client.Close()
		assert.True(t, client.closeRequested.Load())
	})

	// Test closing a client with no outstanding requests
	// Close() should return quickly
	t.Run("with no outstanding requests", func(t *testing.T) {
		client := &CachedClient{
			closeRequested: atomic.NewBool(false),
			conn:           setupGRPCServer(t),
		}

		unittest.RequireReturnsBefore(t, func() {
			client.Close()
		}, 100*time.Millisecond, "client timed out closing connection")

		assert.True(t, client.closeRequested.Load())
	})

	// Test closing a client with outstanding requests waits for requests to complete
	// Close() should block until the request completes
	t.Run("with some outstanding requests", func(t *testing.T) {
		client := &CachedClient{
			closeRequested: atomic.NewBool(false),
			conn:           setupGRPCServer(t),
		}
		done := client.AddRequest()

		doneCalled := atomic.NewBool(false)
		go func() {
			defer done()
			time.Sleep(50 * time.Millisecond)
			doneCalled.Store(true)
		}()

		unittest.RequireReturnsBefore(t, func() {
			client.Close()
		}, 100*time.Millisecond, "client timed out closing connection")

		assert.True(t, client.closeRequested.Load())
		assert.True(t, doneCalled.Load())
	})

	// Test closing a client that is already closing does not block
	// Close() should return immediately
	t.Run("already closing", func(t *testing.T) {
		client := &CachedClient{
			closeRequested: atomic.NewBool(true), // close already requested
			conn:           setupGRPCServer(t),
		}
		done := client.AddRequest()

		doneCalled := atomic.NewBool(false)
		go func() {
			defer done()

			// use a long delay and require Close() to complete faster
			time.Sleep(5 * time.Second)
			doneCalled.Store(true)
		}()

		// should return immediately
		unittest.RequireReturnsBefore(t, func() {
			client.Close()
		}, 10*time.Millisecond, "client timed out closing connection")

		assert.True(t, client.closeRequested.Load())
		assert.False(t, doneCalled.Load())
	})

	// Test closing a client that is locked during connection setup
	// Close() should wait for the lock before shutting down
	t.Run("connection setting up", func(t *testing.T) {
		client := &CachedClient{
			closeRequested: atomic.NewBool(false),
		}

		// simulate an in-progress connection setup
		client.mu.Lock()

		go func() {
			// unlock after setting up the connection
			defer client.mu.Unlock()

			// pause before setting the connection to cause client.Close() to block
			time.Sleep(100 * time.Millisecond)
			client.conn = setupGRPCServer(t)
		}()

		// should wait at least 100 milliseconds before returning
		unittest.RequireReturnsBefore(t, func() {
			client.Close()
		}, 500*time.Millisecond, "client timed out closing connection")

		assert.True(t, client.closeRequested.Load())
		assert.NotNil(t, client.conn)
	})
}

// Test that rapid connections and disconnects do not cause a panic.
func TestConcurrentConnectionsAndDisconnects(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	cache, err := NewCache(logger, metrics, 1)
	require.NoError(t, err)

	connectionCount := 100_000
	conn := setupGRPCServer(t)

	t.Run("test concurrent connections", func(t *testing.T) {
		wg := sync.WaitGroup{}
		wg.Add(connectionCount)
		callCount := atomic.NewInt32(0)
		for i := 0; i < connectionCount; i++ {
			go func() {
				defer wg.Done()
				cachedConn, err := cache.GetConnected("foo", DefaultClientTimeout, nil, func(string, time.Duration, crypto.PublicKey, *CachedClient) (*grpc.ClientConn, error) {
					callCount.Inc()
					return conn, nil
				})
				require.NoError(t, err)

				done := cachedConn.AddRequest()
				time.Sleep(1 * time.Millisecond)
				done()
			}()
		}
		unittest.RequireReturnsBefore(t, wg.Wait, time.Second, "timed out waiting for connections to finish")

		// the client should be cached, so only a single connection is created
		assert.Equal(t, int32(1), callCount.Load())
	})

	t.Run("test rapid connections and invalidations", func(t *testing.T) {
		wg := sync.WaitGroup{}
		wg.Add(connectionCount)
		callCount := atomic.NewInt32(0)
		for i := 0; i < connectionCount; i++ {
			go func() {
				defer wg.Done()
				cachedConn, err := cache.GetConnected("foo", DefaultClientTimeout, nil, func(string, time.Duration, crypto.PublicKey, *CachedClient) (*grpc.ClientConn, error) {
					callCount.Inc()
					return conn, nil
				})
				require.NoError(t, err)

				done := cachedConn.AddRequest()
				time.Sleep(1 * time.Millisecond)
				cachedConn.Invalidate()
				done()
			}()
		}
		wg.Wait()

		// since all connections are invalidated, the cache should be empty at the end
		require.Eventually(t, func() bool {
			return cache.Len() == 0
		}, time.Second, 20*time.Millisecond, "cache should be empty")

		// Many connections should be created, but some will be shared
		assert.Greater(t, callCount.Load(), int32(1))
		assert.LessOrEqual(t, callCount.Load(), int32(connectionCount))
	})
}

// setupGRPCServer starts a dummy grpc server for connection tests
func setupGRPCServer(t *testing.T) *grpc.ClientConn {
	l, err := net.Listen("tcp", net.JoinHostPort("localhost", "0"))
	require.NoError(t, err)

	server := grpc.NewServer()

	t.Cleanup(func() {
		server.Stop()
	})

	go func() {
		err = server.Serve(l)
		require.NoError(t, err)
	}()

	conn, err := grpc.Dial(l.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)

	return conn
}
