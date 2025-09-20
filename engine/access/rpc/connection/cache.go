package connection

import (
	"fmt"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/simplelru"
	"github.com/onflow/crypto"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/onflow/flow-go/module"
)

// ConnectClientFn is callback function that creates a new gRPC client connection.
type ConnectClientFn func(
	address string,
	cfg Config,
	networkPubKey crypto.PublicKey,
	client *cachedClient,
) (grpcClientConn, error)

// <component_spec>
//
// Cache provides access to a pool of cached gRPC client connections as part of the client connection
// pool. It has the following objectives:
//   - Allow reuse of a single gRPC client connection per address, enabling efficient remote calls
//     during Access API handling.
//   - Provide synchronization for getting and creating gRPC client connections.
//   - Allow invalidating entries when connections are determined to be unhealthy.
//   - Ensure only healthy connections are cached, to avoid reusing known unhealthy connections.
//
// The following are required for safe operation:
//   - Only a single connection exists per address.
//   - Invalidated or evicted clients are closed and removed from the cache.
//   - Clients with known failed connections are removed from the cache.
//
// </component_spec>
type Cache struct {
	logger  zerolog.Logger
	metrics module.GRPCConnectionPoolMetrics

	// cache is the underlying LRU cache used to store the client connections.
	// we are using simplelru.LRU which does not provide synchronization. this allow us to implement
	// the atomic GetOrAdd functionality that's missing from the standard lru.Cache implementation,
	// which is required to guarantee that only a single connection exists per address.
	cache   *simplelru.LRU[string, *cachedClient]
	maxSize uint

	// mu protects access to `cache`
	mu sync.RWMutex
}

// NewCache creates a new Cache with the specified maximum size and the underlying LRU cache.
//
// No errors are expected during normal operation.
func NewCache(
	log zerolog.Logger,
	metrics module.GRPCConnectionPoolMetrics,
	maxSize uint,
) (*Cache, error) {
	c := &Cache{
		logger:  log,
		metrics: metrics,
		maxSize: maxSize,
	}

	cache, err := simplelru.NewLRU(int(maxSize), c.onEvict)
	if err != nil {
		return nil, fmt.Errorf("could not initialize connection pool cache: %w", err)
	}
	c.cache = cache

	return c, nil
}

// onEvict is called when a client is evicted from the cache.
func (c *Cache) onEvict(_ string, client *cachedClient) {
	client.Close()

	c.metrics.ConnectionFromPoolEvicted()
	c.logger.Debug().
		Str("grpc_conn_evicted", client.address).
		Msg("closing grpc connection evicted from pool")
}

// GetConnection returns a grpc connection for the given address.
// If the address is not in the cache, it creates a new entry and establishes a connection.
//
// All returned errors are benign and side-effect free for the node. They indicate issues connecting
// with an external node.
func (c *Cache) GetConnection(
	address string,
	cfg Config,
	networkPubKey crypto.PublicKey,
	connectFn ConnectClientFn,
) (grpc.ClientConnInterface, error) {
	client, added := c.getOrAdd(address, cfg.Timeout)

	if added {
		c.metrics.ConnectionAddedToPool()
	} else {
		c.metrics.ConnectionFromPoolReused()
	}

	conn, err := client.clientConnection(cfg, networkPubKey, connectFn)
	if err != nil {
		// remove the failed client from the cache to ensure a fresh connection is created on the
		// next attempt. the client guarantees that only a single connection attempt is made per
		// client object, so all goroutines with a reference to this client will fail with an error.
		// therefore, it's safe to discard the client after the first error.
		_ = c.remove(address)
		return nil, err
	}

	c.metrics.NewConnectionEstablished()
	c.metrics.TotalConnectionsInPool(uint(c.Len()), c.maxSize)

	return conn, nil
}

// getOrAdd atomically gets an existing client from the cache or adds a new one if one doesn't exist.
// this ensures there is only a single client in the cache per address.
// Returns true if a new client was added to the cache.
func (c *Cache) getOrAdd(address string, timeout time.Duration) (*cachedClient, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if client, ok := c.cache.Get(address); ok {
		return client, false
	}

	client := &cachedClient{
		address: address,
		timeout: timeout,
	}

	_ = c.cache.Add(address, client)

	return client, true
}

// Invalidate removes the client entry from the cache and closes the connection.
func (c *Cache) Invalidate(client *cachedClient) {
	if c.remove(client.Address()) {
		c.logger.Debug().Str("cached_client_invalidated", client.Address()).Msg("invalidating cached client")
		c.metrics.ConnectionFromPoolInvalidated()
	}

	client.Close()
}

// remove removes the client entry from the cache.
func (c *Cache) remove(address string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cache.Remove(address)
}

// Len returns the number of entries in the cache.
func (c *Cache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Len()
}
