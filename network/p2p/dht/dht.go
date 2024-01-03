package dht

import (
	"context"

	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/module"
	p2plogging "github.com/onflow/flow-go/network/p2p/logging"
)

// This produces a new IPFS DHT
// on the name, see https://github.com/libp2p/go-libp2p-kad-dht/issues/337
func NewDHT(ctx context.Context, host host.Host, prefix protocol.ID, logger zerolog.Logger, metrics module.DHTMetrics, options ...dht.Option) (*dht.IpfsDHT, error) {
	allOptions := append(options, dht.ProtocolPrefix(prefix))

	kdht, err := dht.New(ctx, host, allOptions...)
	if err != nil {
		return nil, err
	}

	if err = kdht.Bootstrap(ctx); err != nil {
		return nil, err
	}

	dhtLogger := logger.With().Str("component", "dht").Logger()

	routingTable := kdht.RoutingTable()
	peerRemovedCb := routingTable.PeerRemoved
	peerAddedCb := routingTable.PeerAdded
	routingTable.PeerRemoved = func(pid peer.ID) {
		peerRemovedCb(pid)
		dhtLogger.Debug().Str("peer_id", p2plogging.PeerId(pid)).Msg("peer removed from routing table")
		metrics.RoutingTablePeerRemoved()
	}
	routingTable.PeerAdded = func(pid peer.ID) {
		peerAddedCb(pid)
		dhtLogger.Debug().Str("peer_id", p2plogging.PeerId(pid)).Msg("peer added to routing table")
		metrics.RoutingTablePeerAdded()
	}

	return kdht, nil
}

// DHT defaults to ModeAuto which will automatically switch the DHT between Server and Client modes based on
// whether the node appears to be publicly reachable (e.g. not behind a NAT and with a public IP address).
// This default tends to make test setups fail (since the test nodes are normally not reachable by the public
// network), but is useful for improving the stability and performance of live public networks.
// While we could force all nodes to be DHT Servers, a bunch of nodes otherwise not reachable by most of the
// network => network partition

func AsServer() dht.Option {
	return dht.Mode(dht.ModeServer)
}

func AsClient() dht.Option {
	return dht.Mode(dht.ModeClient)
}
