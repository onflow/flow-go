package connection

import (
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"go.uber.org/atomic"
	"google.golang.org/grpc"
)

// CachedClient represents a gRPC client connection that is cached for reuse.
type CachedClient struct {
	ClientConn     *grpc.ClientConn
	Address        string
	timeout        time.Duration
	closeRequested *atomic.Bool
	wg             sync.WaitGroup
	mu             sync.Mutex
}

// Close closes the CachedClient connection. It marks the connection for closure and waits asynchronously for ongoing
// requests to complete before closing the connection.
func (cc *CachedClient) Close() {
	// Mark the connection for closure
	if !cc.closeRequested.CompareAndSwap(false, true) {
		return
	}

	// Obtain the lock to ensure that any connection attempts have completed
	cc.mu.Lock()
	conn := cc.ClientConn
	cc.mu.Unlock()

	// If the initial connection attempt failed, ClientConn will be nil
	if conn == nil {
		return
	}

	// If there are ongoing requests, wait for them to complete asynchronously
	cc.wg.Wait()

	// Close the connection
	conn.Close()
}

// Cache represents a cache of CachedClient instances with a given maximum size.
type Cache struct {
	cache *lru.Cache[string, *CachedClient]
	size  int
}

// NewCache creates a new Cache with the specified maximum size and the underlying LRU cache.
func NewCache(cache *lru.Cache[string, *CachedClient], size int) *Cache {
	return &Cache{
		cache: cache,
		size:  size,
	}
}

// Get retrieves the CachedClient for the given address from the cache.
// It returns the CachedClient and a boolean indicating whether the entry exists in the cache.
func (c *Cache) Get(address string) (*CachedClient, bool) {
	val, ok := c.cache.Get(address)
	if !ok {
		return nil, false
	}
	return val, true
}

// GetOrAdd atomically gets the CachedClient for the given address from the cache, or adds a new one
// if none existed.
// New entries are added to the cache with their mutex locked. This ensures that the caller gets
// priority when working with the new client, allowing it to create the underlying connection.
// Clients retrieved from the cache are returned without modifying their lock.
func (c *Cache) GetOrAdd(address string, timeout time.Duration) (*CachedClient, bool) {
	client := &CachedClient{
		Address:        address,
		timeout:        timeout,
		closeRequested: atomic.NewBool(false),
	}
	client.mu.Lock()

	val, existed, _ := c.cache.PeekOrAdd(address, client)
	if existed {
		return val, true
	}

	return client, false
}

// Add adds a CachedClient to the cache with the given address.
// It returns a boolean indicating whether an existing entry was evicted.
func (c *Cache) Add(address string, client *CachedClient) (evicted bool) {
	return c.cache.Add(address, client)
}

// Remove removes the CachedClient entry from the cache with the given address.
// It returns a boolean indicating whether the entry was present and removed.
func (c *Cache) Remove(address string) (present bool) {
	return c.cache.Remove(address)
}

// Len returns the number of CachedClient entries in the cache.
func (c *Cache) Len() int {
	return c.cache.Len()
}

// MaxSize returns the maximum size of the cache.
func (c *Cache) MaxSize() int {
	return c.size
}

// Contains checks if the cache contains an entry with the given address.
// It returns a boolean indicating whether the address is present in the cache.
func (c *Cache) Contains(address string) (containKey bool) {
	return c.cache.Contains(address)
}
