package connection

import (
	"fmt"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/onflow/crypto"
	"github.com/rs/zerolog"
	"go.uber.org/atomic"
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
	closeRequested *atomic.Bool
	wg             sync.WaitGroup
	mu             sync.RWMutex
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
	return cc.closeRequested.Load()
}

// AddRequest increments the in-flight request counter for the CachedClient.
// It returns a function that should be called when the request completes to decrement the counter
func (cc *CachedClient) AddRequest() func() {
	cc.wg.Add(1)
	return cc.wg.Done
}

// Invalidate removes the CachedClient from the cache and closes the connection.
func (cc *CachedClient) Invalidate() {
	cc.cache.invalidate(cc.address)

	// Close the connection asynchronously to avoid blocking requests
	go cc.Close()
}

// Close closes the CachedClient connection. It marks the connection for closure and waits asynchronously for ongoing
// requests to complete before closing the connection.
func (cc *CachedClient) Close() {
	// Mark the connection for closure
	if !cc.closeRequested.CompareAndSwap(false, true) {
		return
	}

	// Obtain the lock to ensure that any connection attempts have completed
	cc.mu.RLock()
	conn := cc.conn
	cc.mu.RUnlock()

	// If the initial connection attempt failed, conn will be nil
	if conn == nil {
		return
	}

	// If there are ongoing requests, wait for them to complete asynchronously
	// this avoids tearing down the connection while requests are in-flight resulting in errors
	cc.wg.Wait()

	// Close the connection
	conn.Close()
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
		go client.Close() // close is blocking, so run in a goroutine

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
		address:        address,
		timeout:        timeout,
		closeRequested: atomic.NewBool(false),
		cache:          c,
	}

	// Note: PeekOrAdd does not "visit" the existing entry, so we need to call Get explicitly
	// to mark the entry as "visited" and update the LRU order. Unfortunately, the lru library
	// doesn't have a GetOrAdd method, so this is the simplest way to achieve atomic get-or-add
	val, existed, _ := c.cache.PeekOrAdd(address, client)
	if existed {
		client = val
		_, _ = c.cache.Get(address)
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

	c.metrics.NewConnectionEstablished()
	c.metrics.TotalConnectionsInPool(uint(c.Len()), uint(c.MaxSize()))

	client.conn = conn
	return client, nil
}

// invalidate removes the CachedClient entry from the cache with the given address, and shuts
// down the connection.
func (c *Cache) invalidate(address string) {
	if !c.cache.Remove(address) {
		return
	}

	c.logger.Debug().Str("cached_client_invalidated", address).Msg("invalidating cached client")
	c.metrics.ConnectionFromPoolInvalidated()
}

// Len returns the number of CachedClient entries in the cache.
func (c *Cache) Len() int {
	return c.cache.Len()
}

// MaxSize returns the maximum size of the cache.
func (c *Cache) MaxSize() int {
	return c.maxSize
}
