package connection

import (
	"errors"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/utils/unittest"
)

const clientAddress = "test-address"

func clientConf(t *testing.T) (string, Config, crypto.PublicKey) {
	return clientAddress,
		Config{Timeout: 10 * time.Second},
		generateRandomPublicKey(t)
}

func TestGetConnection(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	clientAddress, cfg, networkPubKey := clientConf(t)

	mockConn := newMockGrpcConn()

	// tests that GetConnection() creates a new connection if one doesn't exist, and reuses it on
	// subsequent calls.
	t.Run("creates a new connection if one doesn't exist", func(t *testing.T) {
		cache, err := NewCache(logger, metrics, 100)
		require.NoError(t, err)

		connectFn := assertConnectFn(t, clientAddress, cfg, networkPubKey, mockConn, nil)

		// first call creates a new connection
		conn, err := cache.GetConnection(clientAddress, cfg, networkPubKey, connectFn)
		require.NoError(t, err)
		assert.Same(t, mockConn, conn)

		// subsequent calls reuse the existing connection
		conn, err = cache.GetConnection(clientAddress, cfg, networkPubKey, neverConnectClientFn(t))
		require.NoError(t, err)
		assert.Same(t, mockConn, conn)

		conn, err = cache.GetConnection(clientAddress, cfg, networkPubKey, neverConnectClientFn(t))
		require.NoError(t, err)
		assert.Same(t, mockConn, conn)
	})

	// tests that concurrent calls block and all receive the same connection.
	t.Run("concurrent calls block and all receive the same connection", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			cache, err := NewCache(logger, metrics, 100)
			require.NoError(t, err)

			callCount := atomic.NewInt32(0)
			connectFn := countedConnectClientFn(callCount, mockConn, nil)

			attemptCount := 50
			started := make(chan struct{})
			for range attemptCount {
				go func() {
					<-started
					conn, err := cache.GetConnection(clientAddress, cfg, networkPubKey, connectFn)
					require.NoError(t, err)
					assert.Same(t, mockConn, conn)
				}()
			}
			synctest.Wait() // wait until all goroutines are blocked on the started channel
			close(started)

			synctest.Wait() // wait until all goroutines have finished
			assert.Equal(t, 1, cache.Len())
			assert.Equal(t, int32(1), callCount.Load())
		})
	})

	// tests that GetConnection() returns an error if the connection fails to be established and the
	// client entry is removed from the cache.
	t.Run("returns an error if the connection fails to be established", func(t *testing.T) {
		cache, err := NewCache(logger, metrics, 100)
		require.NoError(t, err)

		expectedErr := errors.New("test error: conn failed to connect")
		connectFn := assertConnectFn(t, clientAddress, cfg, networkPubKey, nil, expectedErr)

		// first call creates a new connection
		conn, err := cache.GetConnection(clientAddress, cfg, networkPubKey, connectFn)
		require.ErrorIs(t, err, expectedErr)
		assert.Nil(t, conn)
		assert.Equal(t, 0, cache.Len())
	})

	// tests that concurrent calls block and all receive the same error. This requires that we
	// synchronize all goroutines to block waiting for the clientConnection() mutex. Otherwise,
	// if calls come after the first connection attempt returns, the cache may create and add a new
	// client entry to the cache.
	//
	// Note: we can't use synctest here because synctest.Wait() does not consider mutexes durably
	// blocked (https://go.dev/blog/synctest#mutexes). This means that we can't wait for all goroutines
	// to block on the clientConnection() mutex. Instead, we use the started and blocked channels to
	// synchronize all goroutines starting at the same time, then a small sleep to give them time to
	// block on the mutex.
	t.Run("concurrent calls block and all receive the same error", func(t *testing.T) {
		cache, err := NewCache(logger, metrics, 100)
		require.NoError(t, err)

		expectedErr := errors.New("test error: conn failed to connect")

		callCount := atomic.NewInt32(0)
		finishConnection := make(chan struct{})
		connectFn := func(string, Config, crypto.PublicKey, *cachedClient) (grpcClientConn, error) {
			if callCount.CompareAndSwap(0, 1) {
				<-finishConnection
				return nil, expectedErr
			}
			return nil, errors.New("test error: only one connection attempt should be made")
		}

		var wg sync.WaitGroup

		attemptCount := 10
		started := make(chan struct{}, attemptCount)
		block := make(chan struct{}, attemptCount)

		gotExpectedErr := atomic.NewBool(false)
		for range attemptCount {
			wg.Go(func() {
				started <- struct{}{}
				<-block
				conn, err := cache.GetConnection(clientAddress, cfg, networkPubKey, connectFn)
				assert.Nil(t, conn)

				// if some of the goroutines take longer to get rescheduled, they may get to the
				// clientConnection() mutex after the connection is closed. Allow either error to be
				// to avoid flakiness.
				switch {
				case errors.Is(err, expectedErr):
					gotExpectedErr.Store(true)
				case errors.Is(err, ErrClientShuttingDown):
				default:
					t.Errorf("unexpected error: %v", err)
				}
			})
		}

		// wait for all goroutines to block on the started channel, then release them
		for range attemptCount {
			<-started
		}
		close(block)

		// give goroutines time to block on the clientConnection() mutex.
		time.Sleep(100 * time.Millisecond)

		// allow the initial connection callback to return, which will release each of the waiting
		// goroutines.
		close(finishConnection)

		// wait for all goroutines to finish. they each run assertions for the error.
		wg.Wait()

		assert.Equal(t, 0, cache.Len())

		// make sure we got at least one of the expected errors
		assert.True(t, gotExpectedErr.Load())
	})
}

// TestInvalidate tests various scenarios for the Invalidate() function.
func TestInvalidate(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	clientAddress, cfg, networkPubKey := clientConf(t)

	// tests that Invalidate() removes the client from the cache and closes its connection.
	t.Run("removes the client from the cache and closes the connection", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			mockConn := newMockGrpcConn()
			connectFn := assertConnectFn(t, clientAddress, cfg, networkPubKey, mockConn, nil)

			cache, err := NewCache(logger, metrics, 100)
			require.NoError(t, err)

			// add the client to the cache
			conn, err := cache.GetConnection(clientAddress, cfg, networkPubKey, connectFn)
			require.NoError(t, err)
			assert.Same(t, mockConn, conn)
			assert.Equal(t, 1, cache.Len())

			client, ok := cache.cache.Get(clientAddress)
			require.True(t, ok)

			// invalidate the client. the cached entry should be removed immediately.
			cache.Invalidate(client)
			assert.Equal(t, 0, cache.Len())

			// wait for the client.Close() goroutine to finish
			synctest.Wait()
			assert.True(t, mockConn.isClosed())
		})
	})

	// tests that Invalidate() closes the connection even if it did not exist in the cache.
	t.Run("closes the connection even if it did not exist in the cache", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			mockConn := newMockGrpcConn()
			client := &cachedClient{
				address: clientAddress,
				conn:    mockConn,
			}

			cache, err := NewCache(logger, metrics, 100)
			require.NoError(t, err)

			// invalidate the client which does not exist in the cache
			assert.Equal(t, 0, cache.Len())
			cache.Invalidate(client)

			// wait for the client.Close() goroutine to finish
			synctest.Wait()
			assert.True(t, mockConn.isClosed())
		})
	})

	// tests that concurrent calls to Invalidate() still remove the client entry, close its connection,
	// and do not panic or deadlock.
	t.Run("concurrent calls behave as expected", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			mockConn := newMockGrpcConn()
			connectFn := assertConnectFn(t, clientAddress, cfg, networkPubKey, mockConn, nil)

			cache, err := NewCache(logger, metrics, 100)
			require.NoError(t, err)

			// add the client to the cache
			conn, err := cache.GetConnection(clientAddress, cfg, networkPubKey, connectFn)
			require.NoError(t, err)
			assert.Same(t, mockConn, conn)
			assert.Equal(t, 1, cache.Len())

			client, ok := cache.cache.Get(clientAddress)
			require.True(t, ok)

			for range 100 {
				go func() {
					cache.Invalidate(client)
				}()
			}

			// wait for the client.Close() goroutine to finish
			synctest.Wait()
			assert.Equal(t, 0, cache.Len())
			assert.True(t, mockConn.isClosed())
		})
	})
}

func TestEvict(t *testing.T) {
	logger := unittest.Logger()
	metrics := metrics.NewNoopCollector()

	clientAddress1 := "test-address-1"
	clientAddress2 := "test-address-2"
	_, cfg, networkPubKey := clientConf(t)

	synctest.Test(t, func(t *testing.T) {
		mockConn1 := newMockGrpcConn()
		mockConn2 := newMockGrpcConn()
		connectFn1 := assertConnectFn(t, clientAddress1, cfg, networkPubKey, mockConn1, nil)
		connectFn2 := assertConnectFn(t, clientAddress2, cfg, networkPubKey, mockConn2, nil)

		cache, err := NewCache(logger, metrics, 1)
		require.NoError(t, err)
		assert.Equal(t, 0, cache.Len())

		// add the first client to the cache
		conn1, err := cache.GetConnection(clientAddress1, cfg, networkPubKey, connectFn1)
		require.NoError(t, err)
		assert.Same(t, mockConn1, conn1)
		assert.Equal(t, 1, cache.Len())

		// confirm the first client is now in the cache
		client1, ok := cache.cache.Get(clientAddress1)
		require.True(t, ok)
		require.Same(t, client1.conn, conn1)

		// add the second client to the cache
		conn2, err := cache.GetConnection(clientAddress2, cfg, networkPubKey, connectFn2)
		require.NoError(t, err)
		assert.Same(t, mockConn2, conn2)
		assert.Equal(t, 1, cache.Len())

		// this should evict the first client. wait for Close() to finish and check that the
		// connection is closed.
		synctest.Wait()
		assert.True(t, mockConn1.isClosed())

		// confirm the second client is now in the cache
		client2, ok := cache.cache.Get(clientAddress2)
		require.True(t, ok)
		require.Same(t, client2.conn, conn2)

		// confirm the first client is no longer in the cache
		client1, ok = cache.cache.Get(clientAddress1)
		require.False(t, ok)
		require.Nil(t, client1)
	})
}

// assertConnectClientFn returns a ConnectClientFn that asserts the correct parameters are passed to
// the connectFn.
func assertConnectFn(
	t *testing.T,
	expectedAddress string,
	expectedCfg Config,
	expectedNetworkPubKey crypto.PublicKey,
	rawConn grpcClientConn,
	err error,
) ConnectClientFn {
	return func(address string, cfg Config, networkPubKey crypto.PublicKey, client *cachedClient) (grpcClientConn, error) {
		assert.Equal(t, expectedAddress, address)
		assert.Equal(t, expectedCfg, cfg)
		if expectedNetworkPubKey != nil {
			assert.Same(t, expectedNetworkPubKey, networkPubKey)
		} else {
			assert.Nil(t, networkPubKey)
		}

		return rawConn, err
	}
}
