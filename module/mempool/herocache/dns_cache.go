package herocache

import (
	"net"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

type DNSCache struct {
	ipCache  *stdmap.Backend
	txtCache *stdmap.Backend
}

func NewDNSCache(sizeLimit uint32, logger zerolog.Logger, ipCollector module.HeroCacheMetrics, txtCollector module.HeroCacheMetrics) *DNSCache {
	return &DNSCache{
		txtCache: stdmap.NewBackend(
			stdmap.WithBackData(
				herocache.NewCache(
					sizeLimit,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("mempool", "dns-txt-cache").Logger(),
					txtCollector))),
		ipCache: stdmap.NewBackend(
			stdmap.WithBackData(
				herocache.NewCache(
					sizeLimit,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("mempool", "dns-ip-cache").Logger(),
					ipCollector))),
	}
}

// PutDomainIp adds the given ip domain into the cache.
func (d *DNSCache) PutDomainIp(domain string, addresses []net.IPAddr, timestamp int64) bool {
	i := ipEntity{
		IpRecord: mempool.IpRecord{
			Domain:    domain,
			Addresses: addresses,
			Timestamp: timestamp,
		},
		id: domainToIdentifier(domain),
	}

	return d.ipCache.Add(i)
}

// PutTxtRecord adds the given txt record into the cache.
func (d *DNSCache) PutTxtRecord(domain string, record []string, timestamp int64) bool {
	t := txtEntity{
		TxtRecord: mempool.TxtRecord{
			Txt:       domain,
			Record:    record,
			Timestamp: timestamp,
		},
		id: domainToIdentifier(domain),
	}

	return d.txtCache.Add(t)
}

// GetIpDomain returns the ip domain if exists in the cache.
// The second return value determines the timestamp of adding the
// domain to the cache.
// The boolean return value determines if domain exists in the cache.
func (d *DNSCache) GetDomainIp(domain string) (*mempool.IpRecord, bool) {
	entity, ok := d.ipCache.ByID(domainToIdentifier(domain))
	if !ok {
		return nil, false
	}

	i, ok := entity.(ipEntity)
	if !ok {
		return nil, false
	}
	ipRecord := i.IpRecord

	return &ipRecord, true
}

// GetTxtRecord returns the txt record if exists in the cache.
// The second return value determines the timestamp of adding the
// record to the cache.
// The boolean return value determines if record exists in the cache.
func (d *DNSCache) GetTxtRecord(domain string) (*mempool.TxtRecord, bool) {
	entity, ok := d.txtCache.ByID(domainToIdentifier(domain))
	if !ok {
		return nil, false
	}

	t, ok := entity.(txtEntity)
	if !ok {
		return nil, false
	}
	txtRecord := t.TxtRecord

	return &txtRecord, true
}

// RemoveIp removes an ip domain from cache.
func (d *DNSCache) RemoveIp(domain string) bool {
	return d.ipCache.Rem(domainToIdentifier(domain))
}

// RemoveTxt removes a txt record from cache.
func (d *DNSCache) RemoveTxt(domain string) bool {
	return d.txtCache.Rem(domainToIdentifier(domain))
}

// Size returns total domains maintained into this cache.
// The first returned value determines number of ip domains.
// The second returned value determines number of txt records.
func (d DNSCache) Size() (uint, uint) {
	return d.ipCache.Size(), d.txtCache.Size()
}

// ipEntity is a dns cache entry for ip records.
type ipEntity struct {
	mempool.IpRecord
	// caching identifier to avoid cpu overhead
	// per query.
	id flow.Identifier
}

func (i ipEntity) ID() flow.Identifier {
	return i.id
}

func (i ipEntity) Checksum() flow.Identifier {
	return domainToIdentifier(i.IpRecord.Domain)
}

// txtEntity is a dns cache entry for txt records.
type txtEntity struct {
	mempool.TxtRecord
	// caching identifier to avoid cpu overhead
	// per query.
	id flow.Identifier
}

func (t txtEntity) ID() flow.Identifier {
	return t.id
}

func (t txtEntity) Checksum() flow.Identifier {
	return domainToIdentifier(t.TxtRecord.Txt)
}

func domainToIdentifier(domain string) flow.Identifier {
	return flow.MakeID(domain)
}
