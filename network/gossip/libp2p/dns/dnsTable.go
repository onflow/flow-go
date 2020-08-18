package dns

import (
	"sync"

	"github.com/multiformats/go-multiaddr"
)

type hostnameMa multiaddr.Multiaddr
type ipMa multiaddr.Multiaddr


var dnsMap = dnsTable{
	hostnameIPMap: make(map[hostnameMa]ipMa),
	ipHostnameMap: make(map[ipMa]hostnameMa),
}

type dnsTable struct {
	sync.Mutex
	hostnameIPMap map[hostnameMa]ipMa
	ipHostnameMap map[ipMa]hostnameMa
}

func (d *dnsTable) update(hma hostnameMa, ipma ipMa) {
	d.Lock()
	defer d.Unlock()
	d.hostnameIPMap[hma] = ipma
	d.ipHostnameMap[ipma] = hma
}

func (d *dnsTable) lookUp(hma hostnameMa) ipMa{
	d.Lock()
	defer d.Unlock()
	return d.hostnameIPMap[hma]
}

func (d *dnsTable) reverseLookUp(ip ipMa) hostnameMa{
	d.Lock()
	defer d.Unlock()
	return d.ipHostnameMap[ip]
}
