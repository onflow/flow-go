// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package gossip

import (
	"context"
	"github.com/dapperlabs/flow-go/proto/gossip/messages"
	"net"
)

// ServePlacer is an interface that represents the protocol
// layer used as a connection medium for gossip
type ServePlacer interface {
	// Serve starts serving a new connection
	Serve(net.Listener)
	// Place places a message for sending according to its gossip mode
	Place(context.Context, string, *messages.GossipMessage, Mode) (*messages.GossipReply, error)
}
