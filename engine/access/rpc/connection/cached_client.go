package connection

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/crypto"
	"google.golang.org/grpc"
)

// ErrClientShuttingDown is returned when the operation was not successful because the client is
// shutting down.
var ErrClientShuttingDown = errors.New("client is shutting down")

type grpcClientConn interface {
	grpc.ClientConnInterface
	Close() error
}

// <component_spec>
//
// cachedClient provides concurrency safe access to a cached gRPC client connection as part of the
// client connection pool. It has the following objectives:
//   - Allow reuse of a single gRPC client connection per address, enabling efficient remote calls
//     during Access API handling.
//   - Enable graceful shutdown of the connection, allowing in-flight requests to complete before
//     closing the connection.
//   - Ensure all interactions are safe for concurrent access by the many goroutines serving the
//     Access API.
//
// The following are required for safe operation:
//   - Only one connection can be created per client. Concurrent calls before the connection is
//     first established should block until the connection is ready.
//   - A client must not be reused after Close() is called.
//   - The grpc.ClientConn is closed only after all outstanding requests have completed.
//   - The grpc.ClientConn must be closed only once.
//   - All operations using the client's connection should be performed using `Run()`.
//
// </component_spec>
//
// CAUTION: All operations using the client's grpc connection should be performed using `Run()` to
// ensure that the connection is not closed while actively in use. Failing to do so may result in
// the connection closing mid-request if the client is evicted from the cache.
//
// All methods are safe for concurrent access.
type cachedClient struct {
	address string
	timeout time.Duration

	// conn is the underlying gRPC client connection.
	// this is the primary object managed by the cachedClient.
	conn grpcClientConn

	// connectErr is set when the initial connection attempt fails. this is needed to avoid making
	// rapid repeated connection attempts when multiple gorountines attempt to connect to the same
	// address concurrently. By caching the error, the higher level business logic can safely discard
	// a cachedClient object if any connection attempt fails since all subsequent calls will also fail.
	connectErr error

	// How our implementation is structured to provide correct concurrency handling:
	//   - All accesses to mutable state are protected by a read-write lock (`mu`).
	//   - All concurrent calls to `Run()` are tracked by a wait group (`wg`).
	//   - `closeRequested` is set when `Close()` is first called, and checked before performing any
	//     operation.
	//   - The client is not closed until `wg` is empty.
	//   - No operation is allowed after `closeRequested` is set. This ensures that only a single
	//     connection is created for the client, and that the connection is closed only once.

	// wg tracks in progress calls to Run, which represent all in-flight requests utilizing the
	// client's connection.
	wg sync.WaitGroup

	// my protects access to conn, connectErr, closeRequested, and wg.Add().
	mu sync.RWMutex

	// closeRequested is set when a client is shutting down. once set, all calls to `Run` or attempts
	// to access/set the client connection should fail.
	closeRequested bool
}

// Address returns the address of the remote server.
func (cc *cachedClient) Address() string {
	return cc.address
}

// Run executes the provided callback and propagates any error returned.
// If `Close()` has been called and the client is shutting down, the callback is not executed and
// [ErrClientShuttingDown] is returned.
//
// The method tracks in-flight requests which the client uses to allow for graceful shutdown of the
// underlying connection.
//
// IMPORTANT: All operations using the client's grpc connection should be performed using `Run()` to
// ensure that the connection is not closed while it's actively in used. Failing to do so may result
// in the connection closing mid-request if the client is evicted from the cache.
//
// Expected error returns during normal operation:
//   - [ErrClientShuttingDown] if the client is shutting down
//
// No assertions are made about the errors returned by the callback. The caller must be aware of the
// expected benign and exception cases, and handle them accordingly.
func (cc *cachedClient) Run(callback func() error) error {
	// atomically check if `closeRequested` is set then add the new request to the wait group.
	// this guarantees that no new requests are allowed after `closeRequested` is set.
	cc.mu.Lock()
	if cc.closeRequested {
		cc.mu.Unlock()
		return ErrClientShuttingDown
	}
	cc.wg.Add(1)
	cc.mu.Unlock()

	// `wg.Done()` must be called in a defer to guarantee that the wait group is decremented even if
	// the callback panics.
	defer cc.wg.Done()

	return callback()
}

// Close asynchronously closes the client's connection after all outstanding requests have completed.
// Additional calls to Close are no-ops.
//
// The method sets an internal flag indicating that the client is shutting down. Any subsequent calls
// to `Run()` or get the client's connection will fail with [ErrClientShuttingDown].
func (cc *cachedClient) Close() {
	conn, ok := cc.initiateClose()
	if !ok {
		// close is already requested or the connection is not set. this is a no-op.
		return
	}

	go func() {
		// If there are ongoing requests, wait for them to complete asynchronously
		// this avoids tearing down the connection while requests are in progress resulting in errors,
		// which is especially problematic when there are cache evictions.
		cc.wg.Wait()

		if err := conn.Close(); err != nil {
			// this indicates that the connection is already closing. the only way this can happen
			// is if there is a bug in the concurrency handling and multiple calls to Close were
			// able to get to this point.
			panic(fmt.Sprintf("unexpected error closing connection: %v", err))
		}
	}()
}

// initiateClose sets closeRequested and returns the connection.
// Returns false if the client is already shutting down or if the connection is not set.
func (cc *cachedClient) initiateClose() (grpcClientConn, bool) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closeRequested {
		return nil, false
	}
	cc.closeRequested = true

	// conn will be nil if the initial connection attempt failed, or if `Close()` was called before
	// the connection was established.
	return cc.conn, (cc.conn != nil)
}

// clientConnection returns the underlying gRPC client connection. If this is the first call, a new
// connection is created using the provided `connectFn`. Any error returned from `connectFn` is returned
// to the caller. If the client is already shutting down, [ErrClientShuttingDown] is returned.
//
// If the initial connection attempt failed, the error is stored and returned on subsequent calls.
// This avoids rapid, repeated connection attempts when concurrent calls are made to clientConnection,
// and allows the cache to safely discard a `cachedClient` entry if the initial connection attempt fails.
// The cache should ensure that this entry is removed so a fresh connection can eventually be
// established.
//
// Expected error returns during normal operation:
//   - [ErrClientShuttingDown] if the client is shutting down.
//
// No assertions are made about the errors returned by `connectFn` since only the caller has sufficient
// context to determine if they are exceptions or benign.
func (cc *cachedClient) clientConnection(
	cfg Config,
	networkPubKey crypto.PublicKey,
	connectFn ConnectClientFn,
) (grpc.ClientConnInterface, error) {
	// during normal operation, the common case is that client connections are already setup and can
	// be reused. use a read lock here to optimize for this case.
	existingConn, err := cc.clientConn()
	if err != nil {
		return nil, err
	}
	if existingConn != nil {
		return existingConn, nil
	}

	// at this point, there is no connection so create a new one. take the write lock to ensure that
	// only a single goroutine is able to create the connection.
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// perform some sanity checks to ensure another goroutine didn't change the client state between
	// releasing the read lock and taking the write lock. If any fail, return early.

	// if `Close()` was called, abort early. skipping this check would result in leaked connections
	// since close can only be called once.
	if cc.closeRequested {
		return nil, ErrClientShuttingDown
	}

	// if another goroutine already attempted to create the connection, return the result.
	if cc.conn != nil || cc.connectErr != nil {
		return cc.conn, cc.connectErr
	}

	// otherwise, this is the first attempt to create the connection.
	conn, err := connectFn(cc.address, cfg, networkPubKey, cc)
	if err != nil {
		cc.connectErr = err
		return nil, err
	}
	cc.conn = conn

	return conn, nil
}

// clientConn returns the underlying gRPC client connection.
//
// Expected error returns during normal operation:
//   - [ErrClientShuttingDown] if the client is shutting down.
func (cc *cachedClient) clientConn() (grpc.ClientConnInterface, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	if cc.closeRequested {
		return nil, ErrClientShuttingDown
	}
	return cc.conn, cc.connectErr
}

// isCloseRequested returns true if the client is shutting down.
// Helper used for testing.
func (cc *cachedClient) isCloseRequested() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.closeRequested
}
