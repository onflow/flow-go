package mempool

import (
	"net"
)

// DNSCache provides an in-memory cache for storing dns entries.
type DNSCache interface {
	// PutIpDomain adds the given ip domain into cache.
	// The int64 argument is the timestamp associated with the domain.
	PutIpDomain(string, []net.IPAddr, int64) bool

	// PutTxtRecord adds the given txt record into the cache.
	// The int64 argument is the timestamp associated with the domain.
	PutTxtRecord(string, []string, int64) bool

	// GetDomainIp returns the ip domain if exists in the cache.
	// The boolean return value determines if domain exists in the cache.
	GetDomainIp(string) (*IpRecord, bool)

	// GetTxtRecord returns the txt record if exists in the cache.
	// The boolean return value determines if record exists in the cache.
	GetTxtRecord(string) (*TxtRecord, bool)

	// LockIPDomain locks an ip address dns record if exists in the cache.
	// The boolean return value determines whether attempt on locking was successful.
	//
	// A locking attempt is successful when the domain record exists in the cache and has not
	// been locked before.
	// Once a domain record gets locked the only way to unlock it is through updating that record.
	//
	// The locking process is defined to record that a resolving attempt is ongoing for an expired domain.
	// So the locking happens to avoid any other parallel resolving.
	LockIPDomain(string) (bool, error)

	// LockTxtRecord locks a txt address dns record if exists in the cache.
	// The boolean return value determines whether attempt on locking was successful.
	//
	// A locking attempt is successful when the domain record exists in the cache and has not
	// been locked before.
	// Once a domain record gets locked the only way to unlock it is through updating that record.
	//
	// The locking process is defined to record that a resolving attempt is ongoing for an expired domain.
	// So the locking happens to avoid any other parallel resolving.
	LockTxtRecord(string) (bool, error)

	// RemoveIp removes an ip domain from cache.
	RemoveIp(string) bool

	// RemoveTxt removes a txt record from cache.
	RemoveTxt(string) bool

	// UpdateTxtRecord updates the dns record for the given txt domain with the new address and timestamp values.
	UpdateTxtRecord(string, []string, int64) error

	// UpdateIPDomain updates the dns record for the given ip domain with the new address and timestamp values.
	UpdateIPDomain(string, []net.IPAddr, int64) error

	// Size returns total domains maintained into this cache.
	// The first returned value determines number of ip domains.
	// The second returned value determines number of txt records.
	Size() (uint, uint)
}

// TxtRecord represents the data model for maintaining a txt dns record in cache.
type TxtRecord struct {
	Txt       string
	Records   []string
	Timestamp int64

	// A TxtRecord is locked when it is expired and a resolving is ongoing for it.
	// A locked TxtRecord is unlocked by updating it through the cache.
	Locked bool
}

// IpRecord represents the data model for maintaining an ip dns record in cache.
type IpRecord struct {
	Domain    string
	Addresses []net.IPAddr
	Timestamp int64

	// An IpRecord is locked when it is expired and a resolving is ongoing for it.
	// A locked IpRecord is unlocked by updating it through the cache.
	Locked bool
}
