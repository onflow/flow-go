package internal

import (
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/multiformats/go-multiaddr"
)

// RelayNotifee is a notifiee that relays notifications to a list of notifiees.
// A network.Notifiee is a function that is called when a network event occurs, such as a new connection.
type RelayNotifee struct {
	n []network.Notifiee
}

var _ network.Notifiee = (*RelayNotifee)(nil)

func NewRelayNotifee(notifiees ...network.Notifiee) *RelayNotifee {
	return &RelayNotifee{notifiees}
}

func (r *RelayNotifee) Listen(n network.Network, multiaddr multiaddr.Multiaddr) {
	for _, notifiee := range r.n {
		notifiee.Listen(n, multiaddr)
	}
}

func (r *RelayNotifee) ListenClose(n network.Network, multiaddr multiaddr.Multiaddr) {
	for _, notifiee := range r.n {
		notifiee.ListenClose(n, multiaddr)
	}
}

func (r *RelayNotifee) Connected(n network.Network, conn network.Conn) {
	for _, notifiee := range r.n {
		notifiee.Connected(n, conn)
	}
}

func (r *RelayNotifee) Disconnected(n network.Network, conn network.Conn) {
	for _, notifiee := range r.n {
		notifiee.Disconnected(n, conn)
	}
}
