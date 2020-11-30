package p2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	libp2pnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/protocol"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

type hostWrapper struct {
	connGater *connGater
	host      host.Host
	pubSub    *pubsub.PubSub
}

func (h hostWrapper) ConnenctionGater() *connGater {
	return h.connGater
}

func (h hostWrapper) Host() host.Host {
	return h.host
}

func (h hostWrapper) PubSub() *pubsub.PubSub {
	return h.pubSub
}

func (h *hostWrapper) SetStreamHandler(flowLibP2PProtocolID protocol.ID, handler libp2pnet.StreamHandler) {
	h.host.SetStreamHandler(flowLibP2PProtocolID, handler)
}

// bootstrapLibP2PHost creates a libp2p host
func bootstrapLibP2PHost(ctx context.Context,
	logger zerolog.Logger,
	nodeAddress NodeAddress,
	conMgr ConnManager,
	key crypto.PrivKey,
	allowList bool,
	allowListAddrs []NodeAddress,
	psOption ...pubsub.Option) (*hostWrapper, error) {

	var connGater *connGater
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
		libp2p.Identity(key),                // pass in the networking key
		libp2p.ConnectionManager(conMgr),    // set the connection manager
		transport,                           // set the protocol
		libp2p.Ping(true),                   // enable ping
	)

	// if allowlisting is enabled, create a connection gator with allowListAddrs
	if allowList {

		// convert each of the allowList address to libp2p peer infos
		allowListPInfos, err := GetPeerInfos(allowListAddrs...)
		if err != nil {
			return nil, fmt.Errorf("failed to create approved list of peers: %w", err)
		}

		// create a connection gater
		connGater = newConnGater(allowListPInfos, logger)

		// provide the connection gater as an option to libp2p
		options = append(options, libp2p.ConnectionGater(connGater))
	}

	// create the libp2p host
	libP2PHost, err := libp2p.New(ctx, options...)
	if err != nil {
		return nil, fmt.Errorf("could not create libp2p host: %w", err)
	}

	// Creating a new PubSub instance of the type GossipSub with psOption
	ps, err := pubsub.NewGossipSub(ctx, libP2PHost, psOption...)
	if err != nil {
		return nil, fmt.Errorf("could not create libp2p pubsub: %w", err)
	}

	return &hostWrapper{
		connGater: connGater,
		host:      libP2PHost,
		pubSub:    ps,
	}, nil
}
