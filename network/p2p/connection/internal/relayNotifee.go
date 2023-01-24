package internal

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
)

type RelayNotifee struct {
	n []network.Notifiee
}

func (r RelayNotifee) Listen(n network.Network, multiaddr multiaddr.Multiaddr) {
	//TODO implement me
	panic("implement me")
}

func (r RelayNotifee) ListenClose(n network.Network, multiaddr multiaddr.Multiaddr) {
	//TODO implement me
	panic("implement me")
}

func (r RelayNotifee) Connected(n network.Network, conn network.Conn) {
	//TODO implement me
	panic("implement me")
}

func (r RelayNotifee) Disconnected(n network.Network, conn network.Conn) {
	//TODO implement me
	panic("implement me")
}

var _ network.Notifiee = (*RelayNotifee)(nil)
