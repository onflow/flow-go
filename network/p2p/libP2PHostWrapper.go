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

// libP2PHostWrapper is an internal data structure to this package that encapsulates
// all components that p2p package affiliates with a libp2p host.
// Purpose of encapsulation is to ease passing the entire libp2p host affiliated components
// around.
type libP2PHostWrapper struct {
	connGater *connGater     // used to provide white listing
	host      host.Host      // reference to the libp2p host (https://godoc.org/github.com/libp2p/go-libp2p-core/host)
	pubSub    *pubsub.PubSub // reference to the libp2p PubSub component
}

// ConnectionGater returns `connGater` component of the libP2PHostWrapper.
func (h libP2PHostWrapper) ConnectionGater() *connGater {
	return h.connGater
}

// Host returns `host` component of the libP2PHostWrapper.
func (h libP2PHostWrapper) Host() host.Host {
	return h.host
}

// PubSub returns `pubsub` component of the libP2PHostWrapper.
func (h libP2PHostWrapper) PubSub() *pubsub.PubSub {
	return h.pubSub
}

// SetStreamHandler sets the stream handler of the host component in the libP2PHostWrapper.
func (h *libP2PHostWrapper) SetStreamHandler(flowLibP2PProtocolID protocol.ID, handler libp2pnet.StreamHandler) {
	h.host.SetStreamHandler(flowLibP2PProtocolID, handler)
}

// bootstrapLibP2PHost creates and starts a libp2p host as well as a pubsub component for it, and returns all in a
// libP2PHostWrapper.
// In case `allowList` is true, it also creates and embeds a connection gater in the returned libP2PHostWrapper, which
// whitelists the `allowListAddres` nodes.
func bootstrapLibP2PHost(ctx context.Context,
	logger zerolog.Logger,
	nodeAddress NodeAddress,
	conMgr ConnManager,
	key crypto.PrivKey,
	allowList bool,
	allowListAddrs []NodeAddress,
	psOption ...pubsub.Option) (*libP2PHostWrapper, error) {

	var connGater *connGater

	sourceMultiAddr, err := multiaddr.NewMultiaddr(MultiaddressStr(nodeAddress))
	if err != nil {
		return nil, fmt.Errorf("failed to translate Flow address to Libp2p multiaddress: %w", err)
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
	options := []config.Option{
		libp2p.ListenAddrs(sourceMultiAddr), // set the listen address
		libp2p.Identity(key),                // pass in the networking key
		libp2p.ConnectionManager(conMgr),    // set the connection manager
		transport,                           // set the protocol
		libp2p.Ping(true),                   // enable ping
	}

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

	return &libP2PHostWrapper{
		connGater: connGater,
		host:      libP2PHost,
		pubSub:    ps,
	}, nil
}
