package dns

import (
	"net"
)

// ipCacheEntry is a dns cache entry for ip records.
type ipCacheEntry struct {
	addresses []net.IPAddr
	timestamp int64
}

// txtCacheEntry is a dns cache entry for txt records.
type txtCacheEntry struct {
	addresses []string
	timestamp int64
}
