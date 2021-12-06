package p2p

import (
	"context"

	"github.com/libp2p/go-libp2p-core/host"
	dht "github.com/libp2p/go-libp2p-kad-dht"

	"github.com/onflow/flow-go/network/p2p/unicast"
)

// This produces a new IPFS DHT
// on the name, see https://github.com/libp2p/go-libp2p-kad-dht/issues/337
func NewDHT(ctx context.Context, host host.Host, options ...dht.Option) (*dht.IpfsDHT, error) {

	defaultOptions := defaultDHTOptions()
	allOptions := append(defaultOptions, options...)

	kdht, err := dht.New(ctx, host, allOptions...)
	if err != nil {
		return nil, err
	}

	if err = kdht.Bootstrap(ctx); err != nil {
		return nil, err
	}

	return kdht, nil
}

// DHT defaults to ModeAuto which will automatically switch the DHT between Server and Client modes based on
// whether the node appears to be publicly reachable (e.g. not behind a NAT and with a public IP address).
// This default tends to make test setups fail (since the test nodes are normally not reachable by the public
// network), but is useful for improving the stability and performance of live public networks.
// While we could force all nodes to be DHT Servers, a bunch of nodes otherwise not reachable by most of the
// network => network partition
func AsServer(enable bool) dht.Option {
	if enable {
		return dht.Mode(dht.ModeServer)
	}
	return dht.Mode(dht.ModeClient)
}

func defaultDHTOptions() []dht.Option {
	return []dht.Option{
		dht.ProtocolPrefix(unicast.FlowLibP2PProtocolCommonPrefix),
	}
}
