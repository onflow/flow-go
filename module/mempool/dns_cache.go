package mempool

import (
	"net"
)

// DNSCache provides an in-memory cache for storing dns entries.
type DNSCache interface {
	// PutIpDomain adds the given ip domain into cache.
	// The uint64 argument is the timestamp associated with the domain.
	PutIpDomain(string, int64, []net.IPAddr) bool

	// PutTxtDomain adds the given txt domain into the cache.
	// The uint64 argument is the timestamp associated with the domain.
	PutTxtDomain(string, int64, []string) bool

	// GetIpDomain returns the ip domain if exists in the cache.
	// The second return value determines the timestamp of adding the
	// domain to the cache.
	// The boolean return value determines if domain exists in the cache.
	GetIpDomain(string) ([]net.IPAddr, int64, bool)

	// GetTxtDomain returns the txt domain if exists in the cache.
	// The second return value determines the timestamp of adding the
	// domain to the cache.
	// The boolean return value determines if domain exists in the cache.
	GetTxtDomain(string) ([]string, int64, bool)

	// RemoveIp removes an ip domain from cache.
	RemoveIp(string) bool

	// RemoveTxt removes a txt domain from cache.
	RemoveTxt(string) bool

	// Size returns total domains maintained into this cache.
	// The first returned value determines number of ip domains.
	// The second returned value determines number of txt domains.
	Size() (uint, uint)
}
