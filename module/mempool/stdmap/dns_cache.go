package stdmap

import (
	"net"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module/mempool"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata/herocache"
	"github.com/onflow/flow-go/module/mempool/stdmap/backdata/herocache/heropool"
)

var _ mempool.DNSCache = &DNSCache{}

type DNSCache struct {
	ipCache  *Backend
	txtCache *Backend
}

func NewDNSCache(sizeLimit uint32, logger zerolog.Logger) *DNSCache {
	return &DNSCache{
		txtCache: NewBackendWithBackData(
			herocache.NewCache(sizeLimit, 8, heropool.LRUEjection, logger)),
		ipCache: NewBackendWithBackData(
			herocache.NewCache(sizeLimit, 8, heropool.LRUEjection, logger)),
	}
}

func (d *DNSCache) PutIpDomain(domain string, addresses []net.IPAddr) error {
	return nil
}

// PutTxtDomain adds the given txt domain into the cache.
func (d *DNSCache) PutTxtDomain(domain string, addresses []string) error {
	return nil
}

// GetIpDomain returns the ip domain if exists in the cache.
// The second return value determines the timestamp of adding the
// domain to the cache.
// The boolean return value determines if domain exists in the cache.
func (d *DNSCache) GetIpDomain(domain string) ([]net.IPAddr, int64, bool) {
	return nil, 0, false
}

// GetTxtDomain returns the txt domain if exists in the cache.
// The second return value determines the timestamp of adding the
// domain to the cache.
// The boolean return value determines if domain exists in the cache.
func (d *DNSCache) GetTxtDomain(domain string) ([]string, int64, bool) {
	return nil, 0, false
}

// RemoveIp removes an ip domain from cache.
func (d *DNSCache) RemoveIp(domain string) bool {
	return false
}

// RemoveTxt removes a txt domain from cache.
func (d *DNSCache) RemoveTxt(domain string) bool {
	return false
}

// Size returns total domains maintained into this cache.
// The first returned value determines number of ip domains.
// The second returned value determines number of txt domains.
func (d DNSCache) Size() (uint, uint) {
	return 0, 0
}
