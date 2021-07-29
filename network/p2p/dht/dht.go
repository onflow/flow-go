package dht

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

const routingTableRefresh time.Duration = time.Second * 100
const peerDiscoveryTimeout time.Duration = time.Second * 2
const FlowRendezVousStr string = "flow"

// THis produces a new IPFS DHT
// on the name, see https://github.com/libp2p/go-libp2p-kad-dht/issues/337
func NewDHT(ctx context.Context, host host.Host, bootstrapPeers []peer.AddrInfo) (*discovery.RoutingDiscovery, error) {
	var defaultOptions = []dht.Option{

		dht.ProtocolPrefix("/flow"),

		dht.RoutingTableRefreshPeriod(routingTableRefresh),        // this is 100 seconds
		dht.RoutingTableRefreshQueryTimeout(peerDiscoveryTimeout), // this is 2 seconds

		// public capabilities we don't need
		dht.DisableProviders(),
		dht.DisableValues(),
	}

	// If we have no bootstrapPeers, we're the server
	if len(bootstrapPeers) == 0 {
		// DHT defaults to ModeAuto which will automatically switch the DHT between Server and Client modes based on whether the node appears to be publicly reachable (e.g. not behind a NAT and with a public IP address).
		// This default tends to make test setups fail (since the test nodes are normally not reachable by the public network), but is useful for improving the stability and performance of live public networks.
		// While we could force all nodes to be DHT Servers, a bunch of nodes otherwise not reachable by most of the network => network partition
		defaultOptions = append(defaultOptions, dht.Mode(dht.ModeServer))
	}

	kdht, err := dht.New(ctx, host, defaultOptions...)
	if err != nil {
		return nil, err
	}

	if err = kdht.Bootstrap(ctx); err != nil {
		return nil, err
	}

	var wg sync.WaitGroup
	for _, peerInfo := range bootstrapPeers {

		wg.Add(1)
		go func(pInfo peer.AddrInfo) {
			defer wg.Done()
			if err := host.Connect(ctx, pInfo); err != nil {
				log.Printf("Error while connecting to node %q: %-v", pInfo, err)
			} else {
				log.Printf("Connection established with bootstrap node: %q", pInfo)
			}
		}(peerInfo)
	}
	wg.Wait()

	var routingDiscovery = discovery.NewRoutingDiscovery(kdht)
	discovery.Advertise(ctx, routingDiscovery, FlowRendezVousStr)

	return routingDiscovery, nil
}
