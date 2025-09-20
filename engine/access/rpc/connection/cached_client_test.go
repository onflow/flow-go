package connection

import (
	crand "crypto/rand"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/onflow/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
)

// TestRun tests the different code paths for the Run function when executed in isolation.
func TestRun(t *testing.T) {
	// test the happy path where the callback is executed successfully.
	t.Run("callback is executed", func(t *testing.T) {
		client := &cachedClient{
			address: clientAddress,
		}

		cb := newSuccessCallbackMock()
		err := client.Run(cb.Execute)

		assert.NoError(t, err)
		assert.Equal(t, 1, cb.Calls())
	})

	// test the case where the callback returns an error.
	t.Run("callback returns error", func(t *testing.T) {
		client := &cachedClient{
			address: clientAddress,
		}
		expectedErr := errors.New("callback error")

		cb := newErrorCallbackMock(expectedErr)
		err := client.Run(cb.Execute)

		assert.ErrorIs(t, err, expectedErr)
		assert.Equal(t, 1, cb.Calls())
	})

	// test the case where the callback panics to ensure that the wait group is decremented avoiding
	// a leak if the caller recovers from the panic.
	t.Run("callback panics", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			mockConn := newMockGrpcConn()
			client := &cachedClient{
				address: clientAddress,
				conn:    mockConn,
			}

			expectedErr := errors.New("callback error")

			cb := newCallbackMock(func() error {
				panic(expectedErr)
			})

			// recover from the panic then call Close(). It should close the connection without
			// blocking since there are no outstanding requests.
			defer func() {
				r := recover()

				if r != nil {
					client.Close()

					assert.True(t, client.isCloseRequested())
					assert.Equal(t, 1, cb.Calls())

					// wait for the Close() goroutine to finish
					synctest.Wait()

					assert.True(t, mockConn.isClosed())
				} else {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			// callback panics, so we don't expect a return
			_ = client.Run(cb.Execute)
		})
	})

	// test the case where the client is already shutting down, and the callback is not executed.
	t.Run("returns ErrClientShuttingDown when shutting down", func(t *testing.T) {
		client := &cachedClient{
			address:        clientAddress,
			closeRequested: true,
		}

		cb := newSuccessCallbackMock()
		err := client.Run(cb.Execute)

		assert.ErrorIs(t, err, ErrClientShuttingDown)
		assert.Equal(t, 0, cb.Calls())
	})

	// test that concurrent calls to Run to not interfere with each other
	t.Run("handles concurrent calls", func(t *testing.T) {
		client := &cachedClient{
			address: clientAddress,
		}

		var wg sync.WaitGroup
		for range 1000 {
			wg.Go(func() {
				cb := newSuccessCallbackMock()
				err := client.Run(cb.Execute)

				assert.NoError(t, err)
				assert.Equal(t, 1, cb.Calls())
			})
		}
		wg.Wait()
	})
}

// TestClose tests the different code paths for the Close function when run in isolation.
func TestClose(t *testing.T) {
	t.Run("initiates shutdown", func(t *testing.T) {
		mockConn := newMockGrpcConn()
		client := &cachedClient{
			address: clientAddress,
			conn:    mockConn,
		}

		client.Close()

		assert.True(t, client.isCloseRequested())
		assert.Eventually(t, func() bool {
			return mockConn.isClosed()
		}, 100*time.Millisecond, 5*time.Millisecond, "client timed out closing connection")
	})

	// test that concurrent calls to Close do not interfere with each other
	t.Run("handles concurrent calls", func(t *testing.T) {
		mockConn := newMockGrpcConn()
		client := &cachedClient{
			address: clientAddress,
			conn:    mockConn,
		}

		synctest.Test(t, func(t *testing.T) {
			defer func() {
				// mockConn.Close() will return an error if called multiple times, which will
				// cause client.Close() to panic. This shouldn't happen here.
				if r := recover(); r != nil {
					t.Errorf("unexpected panic: %v", r)
				}
			}()

			for range 100 {
				go func() {
					client.Close()
					assert.True(t, client.isCloseRequested())
				}()
			}

			// wait for all goroutines to finish, at which point the connection should be closed
			// all test goroutines should run and exit. The single goroutine started by Close() will
			// not block since there are no outstanding requests, so it too will run and exit.
			synctest.Wait()
			assert.True(t, mockConn.isClosed())
		})
	})

	// test the gorountine started by Close() will wait for all requests to complete before closing
	// the connection, and that the connection is closed after the request completes.
	t.Run("waits for requests to complete before closing the connection", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			mockConn := newMockGrpcConn()
			client := &cachedClient{
				address: clientAddress,
				conn:    mockConn,
			}

			// run a request async that will block until requestCanComplete is closed. this allows us
			// to test close will wait for the request to complete.
			requestCanComplete := make(chan struct{})

			started := make(chan struct{})
			go func() {
				err := client.Run(func() error {
					close(started)
					<-requestCanComplete
					return nil
				})
				assert.NoError(t, err)
			}()

			// wait until the Run callback is executed. this ensures that the request has started
			// before Close() is called.
			<-started

			// Start closing the client
			client.Close()

			// Wait for the close goroutine to block waiting for the in-flight request
			synctest.Wait()

			// At this point, both goroutines should be blocked, and the connection should NOT be
			// closed yet
			assert.False(t, mockConn.isClosed(), "connection should not be closed while request is in-flight")

			// Allow the request to complete
			close(requestCanComplete)

			// Wait for all goroutines to finish
			synctest.Wait()

			// Now the connection should be closed
			assert.True(t, mockConn.isClosed())
		})
	})
}

func TestClientConnection(t *testing.T) {
	clientAddress, cfg, networkPubKey := clientConf(t)

	// test that clientConnection calls connectFn on the first request, and returns the existing
	// connection on subsequent requests.
	t.Run("creates new connection once", func(t *testing.T) {
		client := &cachedClient{
			address: clientAddress,
			timeout: cfg.Timeout,
		}

		rawConn := &grpc.ClientConn{}

		// first call creates the connection
		conn, err := client.clientConnection(cfg, networkPubKey, assertConnectClientFn(t, client, cfg, networkPubKey, rawConn, nil))
		assert.NoError(t, err)
		assert.Same(t, rawConn, conn)
		assert.Same(t, rawConn, client.conn)

		// subsequent calls return the existing connection and do not call the connectFn
		conn, err = client.clientConnection(cfg, networkPubKey, neverConnectClientFn(t))
		assert.NoError(t, err)
		assert.Same(t, rawConn, conn)

		conn, err = client.clientConnection(cfg, networkPubKey, neverConnectClientFn(t))
		assert.NoError(t, err)
		assert.Same(t, rawConn, conn)
	})

	// test that clientConnection returns ErrClientShuttingDown when the client is already
	// shutting down.
	t.Run("returns ErrClientShuttingDown when client is shutting down", func(t *testing.T) {
		client := &cachedClient{
			address:        clientAddress,
			closeRequested: true,
		}

		conn, err := client.clientConnection(cfg, networkPubKey, neverConnectClientFn(t))
		assert.ErrorIs(t, err, ErrClientShuttingDown)
		assert.Nil(t, conn)
	})

	// test that clientConnection propagates any error returned from connectFn, and that the same
	// error is returned on subsequent calls.
	t.Run("returns error from first attempt", func(t *testing.T) {
		client := &cachedClient{
			address: clientAddress,
			timeout: cfg.Timeout,
		}

		expectedErr := errors.New("connectFn error")

		// first call fails to create the connection
		conn, err := client.clientConnection(cfg, networkPubKey, assertConnectClientFn(t, client, cfg, networkPubKey, nil, expectedErr))
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, conn)

		// subsequent calls return the existing connection and do not call the connectFn
		conn, err = client.clientConnection(cfg, networkPubKey, neverConnectClientFn(t))
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, conn)

		conn, err = client.clientConnection(cfg, networkPubKey, neverConnectClientFn(t))
		assert.ErrorIs(t, err, expectedErr)
		assert.Nil(t, conn)
	})

	// test making multiple concurrent calls to clientConnection results in exactly one call to
	// connectFn, and that all attempts receive the same connection object.
	t.Run("handles concurrent calls correctly", func(t *testing.T) {
		for range 1000 {
			synctest.Test(t, func(t *testing.T) {
				client := &cachedClient{
					address: clientAddress,
					timeout: cfg.Timeout,
				}

				callCount := atomic.NewInt32(0)
				rawConn := &grpc.ClientConn{}

				connectAttempts := 50
				conns := make(chan grpc.ClientConnInterface, connectAttempts)

				for range connectAttempts {
					go func() {
						conn, err := client.clientConnection(cfg, networkPubKey, countedConnectClientFn(callCount, rawConn, nil))
						assert.NoError(t, err)
						conns <- conn
					}()
				}

				// wait for all goroutines to finish
				synctest.Wait()

				// close the channel to signal that all goroutines have finished
				close(conns)

				assert.Equal(t, int32(1), callCount.Load())
				assert.Equal(t, connectAttempts, len(conns))
				for conn := range conns {
					assert.Same(t, rawConn, conn)
				}
			})
		}
	})
}

// TestClientActions tests various scenarios of concurrent calls to clientConnection, Run, and Close.
func TestClientActions(t *testing.T) {
	clientAddress, cfg, networkPubKey := clientConf(t)

	// tests the happy path where the connection is created, the callback is executed, and the
	// connection is closed in sequence.
	t.Run("serialized connect-run-close", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
			cb := newSuccessCallbackMock()
			mockConn := newMockGrpcConn()
			client := &cachedClient{
				address: clientAddress,
			}

			_, err := client.clientConnection(cfg, networkPubKey, assertConnectClientFn(t, client, cfg, networkPubKey, mockConn, nil))
			require.NoError(t, err)

			err = client.Run(cb.Execute)
			require.NoError(t, err)

			client.Close()

			// wait until all goroutines finish
			synctest.Wait()

			assert.NotNil(t, client.conn)       // connection successfully created
			assert.Equal(t, 1, cb.Calls())      // Run() callback executed
			assert.True(t, mockConn.isClosed()) // connection closed
		})
	})

	// test that if Close() is called during connect, connect finished, then close completes
	t.Run("close during connect", func(t *testing.T) {
		mockConn := newMockGrpcConn()
		client := &cachedClient{
			address: clientAddress,
		}

		synctest.Test(t, func(t *testing.T) {
			_, err := client.clientConnection(cfg, networkPubKey, func(string, Config, crypto.PublicKey, *cachedClient) (grpcClientConn, error) {
				// run Close() within a goroutine since it requires the lock, otherwise this test
				// will deadlock
				started := make(chan struct{})
				go func() {
					close(started)
					client.Close()
				}()
				<-started // wait until goroutine is running

				return mockConn, nil
			})
			require.NoError(t, err)

			// wait until gorountine started by Close() returns
			synctest.Wait()
			assert.True(t, mockConn.isClosed())
		})
	})

	// test that if Close() is called during Run(), the callback finishes, then close completes
	t.Run("close during run", func(t *testing.T) {
		mockConn := newMockGrpcConn()
		client := &cachedClient{
			address: clientAddress,
			conn:    mockConn,
		}

		synctest.Test(t, func(t *testing.T) {
			cb := newCallbackMock(func() error {
				// run Close() within a goroutine since it requires the lock, otherwise this test
				// will deadlock
				started := make(chan struct{})
				go func() {
					close(started)
					client.Close()
				}()
				<-started // wait until goroutine is running

				return nil
			})

			err := client.Run(cb.Execute)
			require.NoError(t, err)

			// wait until gorountine started by Close() returns
			synctest.Wait()
			assert.True(t, mockConn.isClosed())
			assert.Equal(t, 1, cb.Calls())
		})
	})

	// tests that calling Close() concurrently with Run() and clientConnection() eventually results in
	// the connection being closed. this test is designed to exercise different timings of when Close()
	// is called relative to the other functions.
	t.Run("concurrent close eventually closes", func(t *testing.T) {
		mockConn := newMockGrpcConn()
		client := &cachedClient{
			address: clientAddress,
		}

		for range 1000 {
			// test random scenarios of concurrent calls to connect, run, and close.
			// all scenarios should end with the connection being closed.
			synctest.Test(t, func(t *testing.T) {
				cb := newSuccessCallbackMock()

				// synchronize all goroutines on the started channel to create variability in the
				// execution order of the actual logic.
				started := make(chan struct{})

				// randomize the order the functions are started, because their execution order after
				// started is closed is biased by which function is called first. (Discovered empirically)
				functions := []func(){
					func() {
						<-started
						_, err := client.clientConnection(cfg, networkPubKey, func(string, Config, crypto.PublicKey, *cachedClient) (grpcClientConn, error) {
							return mockConn, nil
						})
						if err != nil {
							require.ErrorIs(t, err, ErrClientShuttingDown)
						}
					},
					func() {
						<-started
						err := client.Run(cb.Execute)
						if err != nil {
							require.ErrorIs(t, err, ErrClientShuttingDown)
						}
					},
					func() {
						<-started
						client.Close()
					},
				}

				rand.Shuffle(len(functions), func(i, j int) {
					functions[i], functions[j] = functions[j], functions[i]
				})

				for _, fn := range functions {
					go fn()
				}

				// wait for all goroutines to block on the started channel
				synctest.Wait()

				// allow all goroutines to continue concurrently
				close(started)

				// wait for all goroutines to complete
				synctest.Wait()

				// if Close() was executed first, the clientConnection() call will abort before
				// creating the connection, so it will not be closed.
				if client.conn != nil {
					assert.True(t, mockConn.isClosed())
				}
			})
		}
	})
}

// Mocks and helpers

type callbackMock struct {
	calls *atomic.Int32
	fn    func() error
}

func newSuccessCallbackMock() *callbackMock {
	return newCallbackMock(nil)
}

func newErrorCallbackMock(err error) *callbackMock {
	fn := func() error { return err }
	return newCallbackMock(fn)
}

func newCallbackMock(fn func() error) *callbackMock {
	if fn == nil {
		fn = func() error { return nil }
	}
	return &callbackMock{
		calls: atomic.NewInt32(0),
		fn:    fn,
	}
}

func (c *callbackMock) Execute() error {
	c.calls.Inc()
	return c.fn()
}

func (c *callbackMock) Calls() int {
	return int(c.calls.Load())
}

type mockGrpcConn struct {
	grpc.ClientConnInterface
	closed  *atomic.Bool
	closeFn func() error
}

func newMockGrpcConn() *mockGrpcConn {
	return &mockGrpcConn{
		closed: atomic.NewBool(false),
		closeFn: func() error {
			return nil
		},
	}
}

func (m *mockGrpcConn) Close() error {
	if m.closed.CompareAndSwap(false, true) {
		return m.closeFn()
	}
	return errors.New("connection already closed")
}

func (m *mockGrpcConn) isClosed() bool {
	return m.closed.Load()
}

// assertConnectClientFn returns a ConnectClientFn that asserts the correct parameters are passed to
// the connectFn.
func assertConnectClientFn(t *testing.T, expectedClient *cachedClient, expectedCfg Config, expectedNetworkPubKey crypto.PublicKey, rawConn grpcClientConn, err error) ConnectClientFn {
	return func(address string, cfg Config, networkPubKey crypto.PublicKey, client *cachedClient) (grpcClientConn, error) {
		assert.Equal(t, expectedClient.address, address)
		assert.Equal(t, expectedCfg, cfg)
		assert.Same(t, expectedClient, client)
		if expectedNetworkPubKey != nil {
			assert.Same(t, expectedNetworkPubKey, networkPubKey)
		} else {
			assert.Nil(t, networkPubKey)
		}

		return rawConn, err
	}
}

// neverConnectClientFn returns a ConnectClientFn that fails the test if connectFn is called.
func neverConnectClientFn(t *testing.T) ConnectClientFn {
	return func(string, Config, crypto.PublicKey, *cachedClient) (grpcClientConn, error) {
		t.Errorf("connectFn should never be called")
		return nil, fmt.Errorf("should never be called")
	}
}

// countedConnectClientFn returns a ConnectClientFn that increments the call counter when called.
func countedConnectClientFn(callCount *atomic.Int32, rawConn grpcClientConn, err error) ConnectClientFn {
	return func(string, Config, crypto.PublicKey, *cachedClient) (grpcClientConn, error) {
		callCount.Inc()
		return rawConn, err
	}
}

// generateRandomPublicKey generates a random public key for testing.
func generateRandomPublicKey(t *testing.T) crypto.PublicKey {
	seed := make([]byte, crypto.KeyGenSeedMinLen)
	_, err := crand.Read(seed)
	require.NoError(t, err)

	pk, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
	require.NoError(t, err)
	return pk.PublicKey()
}
