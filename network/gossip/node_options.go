package gossip

import (
	"github.com/dapperlabs/flow-go/network"
	"github.com/dapperlabs/flow-go/network/gossip/order"
	"github.com/dapperlabs/flow-go/network/gossip/registry"
	"github.com/dapperlabs/flow-go/network/gossip/storage"
	"github.com/dapperlabs/flow-go/network/gossip/util"
	"github.com/rs/zerolog"
)

// Option is a configuration function for a node.
type Option func(*Node)

// WithProtocol specifies the protocol to be used with a node
func WithProtocol(sv ServePlacer) Option {
	return func(n *Node) { n.server = sv }
}

// WithLogger specifies the logger to be used with a node
func WithLogger(logger zerolog.Logger) Option {
	return func(n *Node) { n.logger = logger }
}

// WithCodec specifies the codec to be used with a node
func WithCodec(codec network.Codec) Option {
	return func(n *Node) { n.codec = codec }
}

// WithPeerTable specifies the peer table to be used with a node
func WithPeerTable(pt PeersTable) Option {
	return func(n *Node) { n.pt = pt }
}

// WithRegistry specifies the registry manager to be used with a node
func WithRegistry(reg registry.Registry) Option {
	return func(n *Node) { n.regMngr = registry.NewRegistryManager(reg) }
}

// WithEngineRegistry specifies the engine registry to be used with a node
func WithEngineRegistry(er *EngineRegistry) Option {
	return func(n *Node) { n.er = er }
}

// WithHashCache specifies the hash cache implementation to be used
func WithHashCache(hashCache HashCache) Option {
	return func(n *Node) { n.hashCache = hashCache }
}

// WithMessageDatabase specifies the Message storage cache implementation to be used
func WithMessageDatabase(msgStore storage.MessageStore) Option {
	return func(n *Node) { n.msgStore = msgStore }
}

// WithQueueSize specifies the queue size of the node
func WithQueueSize(size int) Option {
	return func(n *Node) { n.queue = make(chan *order.Order, size) }
}

// WithAddress specifies the node address
func WithAddress(address string) Option {
	sock, _ := util.NewSocket(address)
	return func(n *Node) { n.address = sock }
}

// WithPeers gives a peerlist of nodes to the node
func WithPeers(peers []string) Option {
	return func(n *Node) { n.peers = peers }
}

// WithFanout specifies the static fanout set of the node
func WithFanout(fanoutSet []string) Option {
	return func(n *Node) { n.fanoutSet = fanoutSet }
}

// WithStaticFanoutSize specifies the size of the static fanout set to be generated
func WithStaticFanoutSize(staticFanoutSize int) Option {
	return func(n *Node) { n.staticFanoutNum = staticFanoutSize }
}
