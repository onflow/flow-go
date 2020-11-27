package p2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/network"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
)

// Start starts a libp2p node on the given address.
func NewLibP2PHost(ctx context.Context,
	nodeAddress NodeAddress,
	handler network.StreamHandler,
	rootBlockID string,
	allowList bool,
	allowListAddrs []NodeAddress,
	psOption ...pubsub.Option) (host.Host, error) {

	sourceMultiAddr, err := multiaddr.NewMultiaddr(MultiaddressStr(nodeAddress))
	if err != nil {
		return nil, fmt.Errorf("could not generate multi-address for host: %w", err)
	}

	// create a transport which disables port reuse and web socket.
	// Port reuse enables listening and dialing from the same TCP port (https://github.com/libp2p/go-reuseport)
	// While this sounds great, it intermittently causes a 'broken pipe' error
	// as the 1-k discovery process and the 1-1 messaging both sometimes attempt to open connection to the same target
	// As of now there is no requirement of client sockets to be a well-known port, so disabling port reuse all together.
	transport := libp2p.Transport(func(u *tptu.Upgrader) *tcp.TcpTransport {
		tpt := tcp.NewTCPTransport(u)
		tpt.DisableReuseport = true
		return tpt
	})

	// gather all the options for the libp2p node
	var options []config.Option
	options = append(options,
		libp2p.ListenAddrs(sourceMultiAddr), // set the listen address
		libp2p.Identity(n.key),              // pass in the networking key
		libp2p.ConnectionManager(n.conMgr),  // set the connection manager
		transport,                           // set the protocol
		libp2p.Ping(true),                   // enable ping
	)

	// if allowlisting is enabled, create a connection gator with allowListAddrs
	if allowList {

		// convert each of the allowList address to libp2p peer infos
		allowListPInfos, err := GetPeerInfos(allowListAddrs...)
		if err != nil {
			return fmt.Errorf("failed to create approved list of peers: %w", err)
		}

		// create a connection gater
		n.connGater = newConnGater(allowListPInfos, n.logger)

		// provide the connection gater as an option to libp2p
		options = append(options, libp2p.ConnectionGater(n.connGater))
	}

	// create the libp2p host
	host, err := libp2p.New(ctx, options...)
	if err != nil {
		return fmt.Errorf("could not create libp2p host: %w", err)
	}

	n.libP2PHost = host.
		host.SetStreamHandler(n.flowLibP2PProtocolID, handler)

	// Creating a new PubSub instance of the type GossipSub with psOption
	n.ps, err = pubsub.NewGossipSub(ctx, n.libP2PHost, psOption...)
	if err != nil {
		return fmt.Errorf("could not create libp2p pubsub: %w", err)
	}

	n.topics = make(map[string]*pubsub.Topic)
	n.subs = make(map[string]*pubsub.Subscription)

	ip, port, err := n.GetIPPort()
	if err != nil {
		return fmt.Errorf("failed to find IP and port on which the node was started: %w", err)
	}

	n.logger.Debug().
		Str("name", n.name).
		Str("address", fmt.Sprintf("%s:%s", ip, port)).
		Msg("libp2p node started successfully")

	return nil
}
