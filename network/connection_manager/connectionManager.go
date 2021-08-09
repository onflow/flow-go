package connection_manager

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/connmgr"
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/multiformats/go-multiaddr"

	"github.com/onflow/flow-go/network"
)

var _ network.ConnectionManager = &ConnectionManager{}

type ConnectionManager struct {
	sync.RWMutex
	observers           map[network.Observer]struct{}
	connmgr.NullConnMgr // a null conn mgr provided by libp2p to allow implementing only the functions needed
}

func (c *ConnectionManager) Listen(n libp2pnet.Network, multiaddr multiaddr.Multiaddr) {
	c.notifyObservers(func(observer network.Observer) {
		observer.Listen(n, multiaddr)
	})
}

func (c *ConnectionManager) ListenClose(n libp2pnet.Network, multiaddr multiaddr.Multiaddr) {
	c.notifyObservers(func(observer network.Observer) {
		observer.ListenClose(n, multiaddr)
	})
}

func (c *ConnectionManager) Connected(n libp2pnet.Network, conn libp2pnet.Conn) {
	c.notifyObservers(func(observer network.Observer) {
		observer.Connected(n, conn)
	})
}

func (c *ConnectionManager) Disconnected(n libp2pnet.Network, conn libp2pnet.Conn) {
	c.notifyObservers(func(observer network.Observer) {
		observer.Disconnected(n, conn)
	})
}

func (c *ConnectionManager) OpenedStream(n libp2pnet.Network, stream libp2pnet.Stream) {
	c.notifyObservers(func(observer network.Observer) {
		observer.OpenedStream(n, stream)
	})
}

func (c *ConnectionManager) ClosedStream(n libp2pnet.Network, stream libp2pnet.Stream) {
	c.notifyObservers(func(observer network.Observer) {
		observer.ClosedStream(n, stream)
	})
}

func (c *ConnectionManager) notifyObservers(observerFun func(observer network.Observer)) {
	c.RLock()
	defer c.RUnlock()
	for o := range c.observers {
		observerFun(o)
	}
}

func (c *ConnectionManager) Subscribe(observer network.Observer) {
	c.Lock()
	defer c.Unlock()
	c.observers[observer] = struct{}{}
}

func (c *ConnectionManager) Unsubscribe(observer network.Observer) {
	c.Lock()
	defer c.Unlock()
	delete(c.observers, observer)
}
