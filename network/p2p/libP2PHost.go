package p2p

import (
	"context"
	"fmt"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	tptu "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	"github.com/rs/zerolog"
)

type LibP2PHost struct {
	nodeAddress NodeAddress
	connGater   *connGater
	host        host.Host
}

func (l LibP2PHost) Name() string {
	return l.nodeAddress.Name
}

func (l LibP2PHost) ConnenctionGater() *connGater {
	return l.connGater
}

func (l LibP2PHost) Host() host.Host {
	return l.host
}

// Start starts a libp2p node on the given address.
func NewLibP2PHost(ctx context.Context,
	logger zerolog.Logger,
	nodeAddress NodeAddress,
	conMgr ConnManager,
	key crypto.PrivKey,
	allowList bool,
	allowListAddrs []NodeAddress) (*LibP2PHost, error) {

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

	return &LibP2PHost{
		nodeAddress: nodeAddress,
		connGater:   connGater,
		host:        libP2PHost,
	}, nil
}
