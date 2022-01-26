package herocache

import (
	"net"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

type DNSCache struct {
	ipCache  *stdmap.Backend
	txtCache *stdmap.Backend
}

func NewDNSCache(sizeLimit uint32, logger zerolog.Logger) *DNSCache {
	return &DNSCache{
		txtCache: stdmap.NewBackendWithBackData(
			herocache.NewCache(sizeLimit, 8, heropool.LRUEjection, logger)),
		ipCache: stdmap.NewBackendWithBackData(
			herocache.NewCache(sizeLimit, 8, heropool.LRUEjection, logger)),
	}
}

// PutIpDomain adds the given ip domain into the cache.
func (d *DNSCache) PutIpDomain(domain string, timestamp int64, addresses []net.IPAddr) bool {
	i := ipEntity{
		id:        domainToIdentifier(domain),
		domain:    domain,
		addresses: addresses,
		timestamp: timestamp,
	}

	return d.ipCache.Add(i)
}

// PutTxtDomain adds the given txt domain into the cache.
func (d *DNSCache) PutTxtDomain(domain string, timestamp int64, addresses []string) bool {
	t := txtEntity{
		id:        domainToIdentifier(domain),
		domain:    domain,
		addresses: addresses,
		timestamp: timestamp,
	}

	return d.txtCache.Add(t)
}

// GetIpDomain returns the ip domain if exists in the cache.
// The second return value determines the timestamp of adding the
// domain to the cache.
// The boolean return value determines if domain exists in the cache.
func (d *DNSCache) GetIpDomain(domain string) ([]net.IPAddr, int64, bool) {
	entity, ok := d.ipCache.ByID(domainToIdentifier(domain))
	if !ok {
		return nil, 0, false
	}

	i, ok := entity.(ipEntity)
	if !ok {
		return nil, 0, false
	}

	return i.addresses, i.timestamp, true
}

// GetTxtDomain returns the txt domain if exists in the cache.
// The second return value determines the timestamp of adding the
// domain to the cache.
// The boolean return value determines if domain exists in the cache.
func (d *DNSCache) GetTxtDomain(domain string) ([]string, int64, bool) {
	entity, ok := d.txtCache.ByID(domainToIdentifier(domain))
	if !ok {
		return nil, 0, false
	}

	t, ok := entity.(txtEntity)
	if !ok {
		return nil, 0, false
	}

	return t.addresses, t.timestamp, true
}

// RemoveIp removes an ip domain from cache.
func (d *DNSCache) RemoveIp(domain string) bool {
	return d.ipCache.Rem(domainToIdentifier(domain))
}

// RemoveTxt removes a txt domain from cache.
func (d *DNSCache) RemoveTxt(domain string) bool {
	return d.txtCache.Rem(domainToIdentifier(domain))
}

// Size returns total domains maintained into this cache.
// The first returned value determines number of ip domains.
// The second returned value determines number of txt domains.
func (d DNSCache) Size() (uint, uint) {
	return d.ipCache.Size(), d.txtCache.Size()
}

// ipEntity is a dns cache entry for ip records.
type ipEntity struct {
	// caching identifier to avoid cpu overhead
	// per query.
	id        flow.Identifier
	domain    string
	addresses []net.IPAddr
	timestamp int64
}

func (i ipEntity) ID() flow.Identifier {
	return i.id
}

func (i ipEntity) Checksum() flow.Identifier {
	return domainToIdentifier(i.domain)
}

// txtEntity is a dns cache entry for txt records.
type txtEntity struct {
	// caching identifier to avoid cpu overhead
	// per query.
	id        flow.Identifier
	domain    string
	addresses []string
	timestamp int64
}

func (t txtEntity) ID() flow.Identifier {
	return t.id
}

func (t txtEntity) Checksum() flow.Identifier {
	return domainToIdentifier(t.domain)
}

func domainToIdentifier(domain string) flow.Identifier {
	return flow.MakeID(domain)
}
