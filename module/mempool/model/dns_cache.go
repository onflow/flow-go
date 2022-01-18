package model

import (
	"net"
)

// IpCacheEntryDNS is a dns cache entry for ip records.
type IpCacheEntryDNS struct {
	Addresses []net.IPAddr
	Timestamp int64
}

// TxtCacheEntryDNS is a dns cache entry for txt records.
type TxtCacheEntryDNS struct {
	Addresses []string
	Timestamp int64
}
