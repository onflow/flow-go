package connection

import (
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/onflow/crypto"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"

	"github.com/onflow/flow-go/module"
)

// CachedClient represents a gRPC client connection that is cached for reuse.
type CachedClient struct {
	conn    *grpc.ClientConn
	address string
	timeout time.Duration

	cache          *Cache
	closeRequested bool
	closed         bool

	wg sync.WaitGroup
	mu sync.RWMutex
}

// ClientConn returns the underlying gRPC client connection.
func (cc *CachedClient) ClientConn() *grpc.ClientConn {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.conn
}

// Address returns the address of the remote server.
func (cc *CachedClient) Address() string {
	return cc.address
}

// CloseRequested returns true if the CachedClient has been marked for closure.
func (cc *CachedClient) CloseRequested() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.closeRequested
}

// Closed returns true if the CachedClient completed closing its connection.
func (cc *CachedClient) Closed() bool {
	cc.mu.RLock()
	defer cc.mu.RUnlock()
	return cc.closed
}

// TryAddRequest attempts to add a request to the CachedClient.
// If the client is marked for closure, the request is not added and false is returned.
// Otherwise, it increments the in-flight request counter for the CachedClient and returns a function
// that MUST be called when the request completes to decrement the counter.
// Failure to call the callback will prevent the client from closing and will result in a goroutine leak!
func (cc *CachedClient) TryAddRequest() (func(), bool) {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	if cc.closeRequested {
		return func() {}, false
	}

	cc.wg.Add(1)
	return cc.wg.Done, true
}

// Close closes the CachedClient connection.
// It marks the client for closure preventing new requests, then waits asynchronously for outstanding
// requests to complete before closing the connection.
// Returns immediately after marking the client for closure.
func (cc *CachedClient) Close() {
	conn, ok := cc.initiateClose()
	if !ok {
		return
	}

	go func() {
		// If there are ongoing requests, wait for them to complete asynchronously
		// this avoids tearing down the connection while requests are in-flight resulting in errors
		cc.wg.Wait()

		// Close the connection
		conn.Close()

		cc.mu.Lock()
		defer cc.mu.Unlock()

		cc.closed = true
	}()
}

// initiateClose marks the client for closure and returns the connection if it is available.
// Returns false if the client is already marked for closure or if the connection is not available.
func (cc *CachedClient) initiateClose() (*grpc.ClientConn, bool) {
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

// Cache represents a cache of CachedClient instances with a given maximum size.
type Cache struct {
	cache   *lru.Cache[string, *CachedClient]
	maxSize int

	logger  zerolog.Logger
	metrics module.GRPCConnectionPoolMetrics
}

// NewCache creates a new Cache with the specified maximum size and the underlying LRU cache.
func NewCache(
	log zerolog.Logger,
	metrics module.GRPCConnectionPoolMetrics,
	maxSize int,
) (*Cache, error) {
	cache, err := lru.NewWithEvict(maxSize, func(_ string, client *CachedClient) {
		client.Close()

		log.Debug().Str("grpc_conn_evicted", client.address).Msg("closing grpc connection evicted from pool")
		metrics.ConnectionFromPoolEvicted()
	})

	if err != nil {
		return nil, fmt.Errorf("could not initialize connection pool cache: %w", err)
	}

	return &Cache{
		cache:   cache,
		maxSize: maxSize,
		logger:  log,
		metrics: metrics,
	}, nil
}

// GetConnected returns a CachedClient for the given address that has an active connection.
// If the address is not in the cache, it creates a new entry and connects.
func (c *Cache) GetConnected(
	address string,
	timeout time.Duration,
	networkPubKey crypto.PublicKey,
	connectFn func(string, time.Duration, crypto.PublicKey, *CachedClient) (*grpc.ClientConn, error),
) (*CachedClient, error) {
	client := &CachedClient{
		address: address,
		timeout: timeout,
		cache:   c,
	}

	existingClient, existed, _ := c.cache.PeekOrAdd(address, client)
	if existed {
		// Note: PeekOrAdd does not "visit" the existing entry, so we need to call Get explicitly
		// to mark the entry as "visited" and update the LRU order. Unfortunately, the lru library
		// doesn't have a GetOrAdd method, so this is the simplest way to achieve a semi-atomic
		// get-or-add operation.
		_, _ = c.cache.Get(address)
		client = existingClient
		c.metrics.ConnectionFromPoolReused()
	} else {
		c.metrics.ConnectionAddedToPool()
	}

	client.mu.Lock()
	defer client.mu.Unlock()

	// after getting the lock, check if the connection is still active
	if client.conn != nil && client.conn.GetState() != connectivity.Shutdown {
		return client, nil
	}

	// if the connection is not setup yet or closed, create a new connection and cache it
	conn, err := connectFn(client.address, client.timeout, networkPubKey, client)
	if err != nil {
		return nil, err
	}
	client.conn = conn

	c.metrics.NewConnectionEstablished()
	c.metrics.TotalConnectionsInPool(uint(c.Len()), uint(c.MaxSize()))

	return client, nil
}

// invalidate removes the CachedClient entry from the cache with the given address, and shuts
// down the connection.
func (c *Cache) Invalidate(client *CachedClient) {
	if c.cache.Remove(client.Address()) {
		c.logger.Debug().Str("cached_client_invalidated", client.Address()).Msg("invalidating cached client")
		c.metrics.ConnectionFromPoolInvalidated()
	}
	client.Close()
}

// Len returns the number of CachedClient entries in the cache.
func (c *Cache) Len() int {
	return c.cache.Len()
}

// MaxSize returns the maximum size of the cache.
func (c *Cache) MaxSize() int {
	return c.maxSize
}
