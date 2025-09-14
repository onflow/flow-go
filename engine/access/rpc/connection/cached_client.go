package connection

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/onflow/crypto"
	"google.golang.org/grpc"
)

// ErrClientMarkedForClosure is returned when the operation was not successful because the client is
// marked for closure
var ErrClientMarkedForClosure = errors.New("client is marked for closure")

// ConnectClientFn is callback function that creates a new gRPC client connection.
type ConnectClientFn func(
	address string,
	timeout time.Duration,
	networkPubKey crypto.PublicKey,
	client *cachedClient,
) (grpcClientConn, error)

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
//   - A client must not be reused after Close() is called.
//   - Only one connection can be created per client. Concurrent calls before the connection is
//     first established should block until the connection is ready.
//   - Only one goroutine can close the grpc.ClientConn.
//   - The grpc.ClientConn is closed only after all outstanding requests have completed.
//
// </component_spec>
type cachedClient struct {
	conn    grpcClientConn
	address string
	timeout time.Duration

	// connectErr is set when the initial connection attempt fails. this is needed to avoid making
	// rapid repeated connection attempts when multiple gorountines attempt to connect to the same
	// address concurrently. By caching the error, the higher level business logic can safely discard
	// a cachedClient object if any connection attempt fails since all subsequent calls will also fail.
	connectErr error

	// TODO: add overview comment about concurrency handling.

	// wg tracks in progress calls to Run, which are all in-flight requests utilizing the client
	// connection.
	wg sync.WaitGroup

	// my protects access to conn, connectErr, and closeRequested.
	mu sync.RWMutex

	// closeRequested is set when a client is shutting down. once set, all calls to Run or attempts
	// to access client connection should fail.
	closeRequested bool
}

// Address returns the address of the remote server.
func (cc *cachedClient) Address() string {
	return cc.address
}

// Run executes the provided callback and returns any error returned.
// If the client is marked for closure, the callback is not executed and ErrClientMarkedForClosure
// is returned.
//
// This tracks in-flight requests which the client uses to allow for graceful shutdown of the
// underlying connection.
//
// Expected error returns during normal operation:
//   - ErrClientMarkedForClosure if the client is marked for closure
//
// No assertions are made about the errors returned by the callback since only the caller has
// sufficient context to determine if they are exceptions or benign.
func (cc *cachedClient) Run(callback func() error) error {
	// we need to hold the lock to both check that cc.closeRequested is not set and increment the
	// wait group. this guarantees the no new requests are allowed after closeRequested is set.
	cc.mu.Lock()
	if cc.closeRequested {
		cc.mu.Unlock()
		return ErrClientMarkedForClosure
	}
	cc.wg.Add(1)
	cc.mu.Unlock()

	err := callback()
	cc.wg.Done()

	return err
}

// Close asynchronously closes the client's connection after all outstanding requests have completed.
// Additional calls to Close are no-ops.
//
// This sets an internal flag indicating that the client is shutting down. Any subsequent calls to
// Run or get the client's connection will fail with ErrClientMarkedForClosure.
func (cc *cachedClient) Close() {
	conn, ok := cc.initiateClose()
	if !ok {
		// close is already requested or the connection is not set. this is a no-op.
		return
	}

	go func() {
		// If there are ongoing requests, wait for them to complete asynchronously
		// this avoids tearing down the connection while requests are in-flight resulting in errors
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

	// If the initial connection attempt failed, conn will be nil
	if cc.conn == nil {
		return nil, false
	}

	return cc.conn, true
}

// clientConnection returns the underlying gRPC client connection. If this is the first call, a new
// connection is created using the provided connectFn and networkPubKey. Any error returned from
// connectFn is returned to the caller.
//
// If the initial connection attempt failed, the error is stored and returned on subsequent calls.
// This avoids rapid repeated connection attempts when concurrent calls are made to clientConnection.
// The cache should ensure that this entry is removed so a fresh connection can eventually be
// established.
//
// If the client is already shutting down, ErrClientMarkedForClosure is returned.
//
// Expected error returns during normal operation:
//   - ErrClientMarkedForClosure if the client is marked for closure
//
// No assertions are made about the errors returned by connectFn since only the caller has sufficient
// context to determine if they are exceptions or benign.
func (cc *cachedClient) clientConnection(
	connectFn ConnectClientFn,
	networkPubKey crypto.PublicKey,
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

	// there is no connection so create a new one. take the write lock to ensure that only a single
	// goroutine is able to create the connection.
	cc.mu.Lock()
	defer cc.mu.Unlock()

	// perform some sanity checks to ensure another goroutine didn't change the client state between
	// releasing the read lock and taking the write lock. If any fail, return early.

	// 1. the client is marked for closure. this is unlikely to happen, but would result in a
	// leaked connection if close was called before we set the conn.
	if cc.closeRequested {
		return nil, ErrClientMarkedForClosure
	}

	// 2. the initial connection attempt failed. this avoids repeated failed connection attempts when
	// concurrent calls are made.
	if cc.connectErr != nil {
		return nil, cc.connectErr
	}

	// 3. the connection is already successfully setup.
	if cc.conn != nil {
		return cc.conn, nil
	}

	// otherwise, this is the first goroutine to create the connection.
	conn, err := connectFn(cc.address, cc.timeout, networkPubKey, cc)
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
//   - ErrClientMarkedForClosure if the client is marked for closure
func (cc *cachedClient) clientConn() (grpc.ClientConnInterface, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	if cc.closeRequested {
		return nil, ErrClientMarkedForClosure
	}
	return cc.conn, cc.connectErr
}

// isCloseRequested returns true if the client is marked for closure.
// Helper used for testing.
func (cc *cachedClient) isCloseRequested() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.closeRequested
}
