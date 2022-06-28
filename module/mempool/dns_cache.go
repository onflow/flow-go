package mempool

import (
	"net"
)

// DNSCache provides an in-memory cache for storing dns entries.
type DNSCache interface {
	// PutDomainIp adds the given ip domain into cache.
	// The int64 argument is the timestamp associated with the domain.
	PutDomainIp(string, []net.IPAddr, int64) bool

	// PutTxtRecord adds the given txt record into the cache.
	// The int64 argument is the timestamp associated with the domain.
	PutTxtRecord(string, []string, int64) bool

	// GetDomainIp returns the ip domain if exists in the cache.
	// The boolean return value determines if domain exists in the cache.
	GetDomainIp(string) (*IpRecord, bool)

	// GetTxtRecord returns the txt record if exists in the cache.
	// The boolean return value determines if record exists in the cache.
	GetTxtRecord(string) (*TxtRecord, bool)

	// RemoveIp removes an ip domain from cache.
	RemoveIp(string) bool

	// RemoveTxt removes a txt record from cache.
	RemoveTxt(string) bool

	// Size returns total domains maintained into this cache.
	// The first returned value determines number of ip domains.
	// The second returned value determines number of txt records.
	Size() (uint, uint)
}

type TxtRecord struct {
	Txt       string
	Record    []string
	Timestamp int64
	Locked    bool
}

type IpRecord struct {
	Domain    string
	Addresses []net.IPAddr
	Timestamp int64
	Locked    bool
}
