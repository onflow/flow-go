package herocache

import (
	"fmt"
	"net"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool"
	herocache "github.com/onflow/flow-go/module/mempool/herocache/backdata"
	"github.com/onflow/flow-go/module/mempool/herocache/backdata/heropool"
	"github.com/onflow/flow-go/module/mempool/stdmap"
)

// DNSCache provides a caching mechanism for DNS IP and TXT records.
//
// The cache stores IP records in ipCache and TXT records in txtCache, using the domain's hashed
// flow.Identifier as the key.
type DNSCache struct {
	ipCache  *stdmap.Backend[flow.Identifier, *mempool.IpRecord]
	txtCache *stdmap.Backend[flow.Identifier, *mempool.TxtRecord]
}

// NewDNSCache creates and returns a new instance of DNSCache.
// It initializes both the IP and TXT record caches with the provided size limit, logger,
// and cache metrics collectors for IP and TXT records.
//
// Parameters:
// - sizeLimit: The maximum number of records the cache can hold before eviction.
// - logger: The logger instance used to log cache operations, such as evictions.
// - ipCollector: Metrics collector for IP records in the cache.
// - txtCollector: Metrics collector for TXT records in the cache.
func NewDNSCache(sizeLimit uint32, logger zerolog.Logger, ipCollector module.HeroCacheMetrics, txtCollector module.HeroCacheMetrics,
) *DNSCache {
	return &DNSCache{
		txtCache: stdmap.NewBackend(
			stdmap.WithMutableBackData[flow.Identifier, *mempool.TxtRecord](
				herocache.NewCache[*mempool.TxtRecord](
					sizeLimit,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("mempool", "dns-txt-cache").Logger(),
					txtCollector,
				),
			),
		),
		ipCache: stdmap.NewBackend(
			stdmap.WithMutableBackData[flow.Identifier, *mempool.IpRecord](
				herocache.NewCache[*mempool.IpRecord](
					sizeLimit,
					herocache.DefaultOversizeFactor,
					heropool.LRUEjection,
					logger.With().Str("mempool", "dns-ip-cache").Logger(),
					ipCollector,
				),
			),
		),
	}
}

// PutIpDomain adds the given ip domain into the cache.
func (d *DNSCache) PutIpDomain(domain string, addresses []net.IPAddr, timestamp int64) bool {
	ipRecord := &mempool.IpRecord{
		Domain:    domain,
		Addresses: addresses,
		Timestamp: timestamp,
		Locked:    false,
	}

	return d.ipCache.Add(domainToIdentifier(domain), ipRecord)
}

// PutTxtRecord adds the given txt record into the cache.
func (d *DNSCache) PutTxtRecord(domain string, record []string, timestamp int64) bool {
	txtRecord := &mempool.TxtRecord{
		Txt:       domain,
		Records:   record,
		Timestamp: timestamp,
		Locked:    false,
	}

	return d.txtCache.Add(domainToIdentifier(domain), txtRecord)
}

// GetDomainIp returns the ip domain if exists in the cache.
// The boolean return value determines if domain exists in the cache.
func (d *DNSCache) GetDomainIp(domain string) (*mempool.IpRecord, bool) {
	return d.ipCache.Get(domainToIdentifier(domain))
}

// GetTxtRecord returns the txt record if exists in the cache.
// The boolean return value determines if record exists in the cache.
func (d *DNSCache) GetTxtRecord(domain string) (*mempool.TxtRecord, bool) {
	return d.txtCache.Get(domainToIdentifier(domain))
}

// RemoveIp removes an ip domain from cache.
func (d *DNSCache) RemoveIp(domain string) bool {
	return d.ipCache.Remove(domainToIdentifier(domain))
}

// RemoveTxt removes a txt record from cache.
func (d *DNSCache) RemoveTxt(domain string) bool {
	return d.txtCache.Remove(domainToIdentifier(domain))
}

// LockIPDomain locks an ip address dns record if exists in the cache.
// The boolean return value determines whether attempt on locking was successful.
//
// A locking attempt is successful when the domain record exists in the cache and has not
// been locked before.
// Once a domain record gets locked the only way to unlock it is through updating that record.
//
// The locking process is defined to record that a resolving attempt is ongoing for an expired domain.
// So the locking happens to avoid any other parallel resolving
func (d *DNSCache) LockIPDomain(domain string) (bool, error) {
	locked := false
	err := d.ipCache.Run(func(backData mempool.BackData[flow.Identifier, *mempool.IpRecord]) error {
		key := domainToIdentifier(domain)

		ipRecord, ok := backData.Get(key)
		if !ok {
			return fmt.Errorf("ip record does not exist in cache for locking: %s", domain)
		}

		if ipRecord.Locked {
			return nil // record has already been locked
		}

		ipRecord.Locked = true

		if _, removed := backData.Remove(key); !removed {
			return fmt.Errorf("ip record could not be removed from backdata")
		}

		if added := backData.Add(key, ipRecord); !added {
			return fmt.Errorf("updated ip record could not be added to back data")
		}

		locked = ipRecord.Locked
		return nil
	})

	return locked, err
}

// UpdateIPDomain updates the dns record for the given ip domain with the new address and timestamp values.
func (d *DNSCache) UpdateIPDomain(domain string, addresses []net.IPAddr, timestamp int64) error {
	return d.ipCache.Run(func(backData mempool.BackData[flow.Identifier, *mempool.IpRecord]) error {
		key := domainToIdentifier(domain)

		// removes old entry if exists.
		backData.Remove(key)

		ipRecord := &mempool.IpRecord{
			Domain:    domain,
			Addresses: addresses,
			Timestamp: timestamp,
			Locked:    false, // by default an ip record is unlocked.
		}

		if added := backData.Add(key, ipRecord); !added {
			return fmt.Errorf("updated ip record could not be added to backdata")
		}

		return nil
	})
}

// UpdateTxtRecord updates the dns record for the given txt domain with the new address and timestamp values.
func (d *DNSCache) UpdateTxtRecord(txt string, records []string, timestamp int64) error {
	return d.txtCache.Run(func(backData mempool.BackData[flow.Identifier, *mempool.TxtRecord]) error {
		key := domainToIdentifier(txt)

		// removes old entry if exists.
		backData.Remove(key)

		txtRecord := &mempool.TxtRecord{
			Txt:       txt,
			Records:   records,
			Timestamp: timestamp,
			Locked:    false, // by default a txt record is unlocked.
		}

		if added := backData.Add(key, txtRecord); !added {
			return fmt.Errorf("updated txt record could not be added to backdata")
		}

		return nil
	})
}

// LockTxtRecord locks a txt address dns record if exists in the cache.
// The boolean return value determines whether attempt on locking was successful.
//
// A locking attempt is successful when the domain record exists in the cache and has not
// been locked before.
// Once a domain record gets locked the only way to unlock it is through updating that record.
//
// The locking process is defined to record that a resolving attempt is ongoing for an expired domain.
// So the locking happens to avoid any other parallel resolving.
func (d *DNSCache) LockTxtRecord(txt string) (bool, error) {
	locked := false
	err := d.txtCache.Run(func(backData mempool.BackData[flow.Identifier, *mempool.TxtRecord]) error {
		key := domainToIdentifier(txt)

		txtRecord, ok := backData.Get(key)
		if !ok {
			return fmt.Errorf("txt record does not exist in cache for locking: %s", txt)
		}

		if txtRecord.Locked {
			return nil // record has already been locked
		}

		txtRecord.Locked = true

		if _, removed := backData.Remove(key); !removed {
			return fmt.Errorf("txt record could not be removed from backdata")
		}

		if added := backData.Add(key, txtRecord); !added {
			return fmt.Errorf("updated txt record could not be added to back data")
		}

		locked = txtRecord.Locked
		return nil
	})

	return locked, err
}

// Size returns total domains maintained into this cache.
// The first returned value determines number of ip domains.
// The second returned value determines number of txt records.
func (d *DNSCache) Size() (uint, uint) {
	return d.ipCache.Size(), d.txtCache.Size()
}

// domainToIdentifier is a helper function for creating the key for DNSCache by hashing the domain.
// Returns:
// - the hash of the domain as a flow.Identifier.
func domainToIdentifier(domain string) flow.Identifier {
	return flow.MakeID(domain)
}
