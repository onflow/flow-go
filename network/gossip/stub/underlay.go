package stub

import (
	"errors"
	"fmt"

	"github.com/dapperlabs/flow-go/network/gossip"
)

// Underlay is a stub to be used for testing overlay's behavior
type Underlay struct {
	running    bool
	callback   gossip.OnReceiveCallback
	whitePeers map[string]bool
}

// Handle stores the callback for onReceive, can only be set once
func (u *Underlay) Handle(onReceive gossip.OnReceiveCallback) error {
	if u.callback != nil {
		return errors.New("callback can only be set once")
	}
	u.callback = onReceive
	return nil
}

func (u *Underlay) Start(address string, port string) error {
	if u.running {
		return errors.New("already running")
	}
	u.running = true
	return nil
}

func (u *Underlay) Stop() error {
	if !u.running {
		return errors.New("not running")
	}
	u.running = false
	return nil
}

// Dial will return a connection if the peer is whitelisted,
// otherwise will return an error
func (u *Underlay) Dial(address string) (*Connection, error) {
	_, ok := u.whitePeers[address]
	if !ok {
		return nil, fmt.Errorf("fail to connect to an non-whitelisted peer %v", address)
	}
	return NewConnection(address, u), nil
}

// mark a certain address as whitelist peer
func (u *Underlay) whitelistPeer(address string) {
	u.whitePeers[address] = true
}

func (u *Underlay) receive(sender string, msg []byte) {
	u.callback(sender, msg)
}
