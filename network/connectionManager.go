package network

import (
	"github.com/libp2p/go-libp2p-core/connmgr"
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
)

type Observer interface {
	libp2pnet.Notifiee
}

type Observable interface {
	// indicates that the observer is ready to receive notifications from the Observable
	Subscribe(observer Observer)
	// indicates that the observer no longer wants to receive notifications from the Observable
	Unsubscribe(observer Observer)
}

type ConnectionManager interface {
	connmgr.ConnManager
	libp2pnet.Notifiee
	Observable
}
