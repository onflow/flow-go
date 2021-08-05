package p2p

import (
	"context"
	"time"

	"github.com/libp2p/go-libp2p-core/host"
	discovery "github.com/libp2p/go-libp2p-discovery"
	dht "github.com/libp2p/go-libp2p-kad-dht"
)

const routingTableRefresh  = time.Second // * 100
const peerDiscoveryTimeout = time.Second * 2
const FlowRendezVousStr string = "flow"

// THis produces a new IPFS DHT
// on the name, see https://github.com/libp2p/go-libp2p-kad-dht/issues/337
func newDHT(ctx context.Context, host host.Host, serverMode bool) (*discovery.RoutingDiscovery, error) {
	defaultOptions := defaultDHTOptions()

	// If we have no bootstrapPeers, we're the server
	if serverMode {
		// DHT defaults to ModeAuto which will automatically switch the DHT between Server and Client modes based on
		// whether the node appears to be publicly reachable (e.g. not behind a NAT and with a public IP address).
		// This default tends to make test setups fail (since the test nodes are normally not reachable by the public
		// network), but is useful for improving the stability and performance of live public networks.
		// While we could force all nodes to be DHT Servers, a bunch of nodes otherwise not reachable by most of the
		// network => network partition
		defaultOptions = append(defaultOptions, dht.Mode(dht.ModeServer))
	}

	kdht, err := dht.New(ctx, host, defaultOptions...)
	if err != nil {
		return nil, err
	}

	if err = kdht.Bootstrap(ctx); err != nil {
		return nil, err
	}

	//var wg sync.WaitGroup
	//for _, peerInfo := range bootstrapPeers {
	//
	//	wg.Add(1)
	//	go func(pInfo peer.AddrInfo) {
	//		defer wg.Done()
	//		if err := host.Connect(ctx, pInfo); err != nil {
	//			log.Printf("Error while connecting to node %q: %-v", pInfo, err)
	//		} else {
	//			log.Printf("Connection established with bootstrap node: %q", pInfo)
	//		}
	//	}(peerInfo)
	//}
	//wg.Wait()

	var routingDiscovery = discovery.NewRoutingDiscovery(kdht)
//	discovery.Advertise(ctx, routingDiscovery, FlowRendezVousStr)

	return routingDiscovery, nil
}

func NewDHTServer(ctx context.Context, host host.Host) (*discovery.RoutingDiscovery, error) {
	return newDHT(ctx, host, true)
}

func NewDHTClient(ctx context.Context, host host.Host) (*discovery.RoutingDiscovery, error) {
	return newDHT(ctx, host, false)
}

func defaultDHTOptions() []dht.Option {
	return []dht.Option{

		dht.ProtocolPrefix(FlowLibP2PProtocolCommonPrefix),

		dht.RoutingTableRefreshPeriod(routingTableRefresh),        // this is 100 seconds
		dht.RoutingTableRefreshQueryTimeout(peerDiscoveryTimeout), // this is 2 seconds

		// public capabilities we don't need - disabling these capabilities does not allow peer discovery
		//dht.DisableProviders(),
		//dht.DisableValues(),
	}
}
