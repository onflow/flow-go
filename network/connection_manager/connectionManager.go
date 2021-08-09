package connection_manager

import (
	"sync"

	"github.com/libp2p/go-libp2p-core/connmgr"
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/network"
)

var _ network.ConnectionManager = &ConnectionManager{}

type ConnectionManager struct {
	sync.RWMutex
	observers           map[network.Observer]struct{}
	connmgr.NullConnMgr // a null conn mgr provided by libp2p to allow implementing only the functions needed
}

type Option func(c *ConnectionManager)

func WithConnectionLogger(log zerolog.Logger) Option {
	return func(cm *ConnectionManager) {
		cl := NewConnectionLogger(log)
		cm.Subscribe(cl)
	}
}

func WithConnectionMetrics(metrics module.NetworkMetrics) Option {
	return func(cm *ConnectionManager) {
		cl := NewConnectionMetrics(metrics)
		cm.Subscribe(cl)
	}
}

func NewConnectionManager(options ...Option) *ConnectionManager {
	cm := &ConnectionManager{
		observers:   make(map[network.Observer]struct{}),
		NullConnMgr: connmgr.NullConnMgr{},
	}
	for _, o := range options {
		o(cm)
	}
	return cm
}

func (cm *ConnectionManager) Listen(n libp2pnet.Network, multiaddr multiaddr.Multiaddr) {
	cm.notifyObservers(func(observer network.Observer) {
		observer.Listen(n, multiaddr)
	})
}

func (cm *ConnectionManager) ListenClose(n libp2pnet.Network, multiaddr multiaddr.Multiaddr) {
	cm.notifyObservers(func(observer network.Observer) {
		observer.ListenClose(n, multiaddr)
	})
}

func (cm *ConnectionManager) Connected(n libp2pnet.Network, conn libp2pnet.Conn) {
	cm.notifyObservers(func(observer network.Observer) {
		observer.Connected(n, conn)
	})
}

func (cm *ConnectionManager) Disconnected(n libp2pnet.Network, conn libp2pnet.Conn) {
	cm.notifyObservers(func(observer network.Observer) {
		observer.Disconnected(n, conn)
	})
}

func (cm *ConnectionManager) OpenedStream(n libp2pnet.Network, stream libp2pnet.Stream) {
	cm.notifyObservers(func(observer network.Observer) {
		observer.OpenedStream(n, stream)
	})
}

func (cm *ConnectionManager) ClosedStream(n libp2pnet.Network, stream libp2pnet.Stream) {
	cm.notifyObservers(func(observer network.Observer) {
		observer.ClosedStream(n, stream)
	})
}

func (cm *ConnectionManager) notifyObservers(observerFun func(observer network.Observer)) {
	cm.RLock()
	defer cm.RUnlock()
	for o := range cm.observers {
		observerFun(o)
	}
}

func (cm *ConnectionManager) Subscribe(observer network.Observer) {
	cm.Lock()
	defer cm.Unlock()
	cm.observers[observer] = struct{}{}
}

func (cm *ConnectionManager) Unsubscribe(observer network.Observer) {
	cm.Lock()
	defer cm.Unlock()
	delete(cm.observers, observer)
}
