package dns

import (
	"sync"

	"github.com/multiformats/go-multiaddr"
)

type hostnameMa string
type ipMa string

func hmaStr(ma multiaddr.Multiaddr) hostnameMa {
	return hostnameMa(ma.String())
}

func ipmaStr(ma multiaddr.Multiaddr) ipMa {
	return ipMa(ma.String())
}


var DefaultDnsMap = DnsMap{
	hostnameIPMap: make(map[hostnameMa]multiaddr.Multiaddr),
	ipHostnameMap: make(map[ipMa]multiaddr.Multiaddr),
}

type DnsMap struct {
	m sync.Mutex
	hostnameIPMap map[hostnameMa]multiaddr.Multiaddr
	ipHostnameMap map[ipMa]multiaddr.Multiaddr
}

func (d *DnsMap) update(hostname, ip multiaddr.Multiaddr) {
	hma := hmaStr(hostname)
	ipma := ipmaStr(ip)
	d.m.Lock()
	defer d.m.Unlock()
	d.hostnameIPMap[hma] = ip
	d.ipHostnameMap[ipma] = hostname
}

func (d *DnsMap) LookUp(hostname multiaddr.Multiaddr) multiaddr.Multiaddr{
	hma := hmaStr(hostname)
	d.m.Lock()
	defer d.m.Unlock()
	if ip, ok := d.hostnameIPMap[hma]; ok {
		return ip
	}
	return nil
}

func (d *DnsMap) ReverseLookUp(ip multiaddr.Multiaddr) multiaddr.Multiaddr{
	ipma := ipmaStr(ip)
	d.m.Lock()
	defer d.m.Unlock()
	if h, ok := d.ipHostnameMap[ipma]; ok {
		return h
	}
	return nil
}
