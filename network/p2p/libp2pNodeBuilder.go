package p2p

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/routing"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	stream "github.com/libp2p/go-libp2p-transport-upgrader"
	"github.com/libp2p/go-libp2p/config"
	"github.com/libp2p/go-tcp-transport"
	"github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/network/p2p/keyutils"
	"github.com/onflow/flow-go/network/p2p/unicast"
)

// LibP2PFactoryFunc is a factory function type for generating libp2p Node instances.
type LibP2PFactoryFunc func(context.Context) (*Node, error)

// DefaultLibP2PNodeFactory returns a LibP2PFactoryFunc which generates the libp2p host initialized with the
// default options for the host, the pubsub and the ping service.
func DefaultLibP2PNodeFactory(
	log zerolog.Logger,
	me flow.Identifier,
	address string,
	flowKey fcrypto.PrivateKey,
	sporkId flow.Identifier,
	idProvider id.IdentityProvider,
	maxPubSubMsgSize int,
	metrics module.NetworkMetrics,
	pingInfoProvider PingInfoProvider,
	dnsResolverTTL time.Duration,
	role string,
) LibP2PFactoryFunc {

	return func(ctx context.Context) (*Node, error) {
		connManager := NewConnManager(log, metrics)

		resolver := dns.NewResolver(metrics, dns.WithTTL(dnsResolverTTL))
		libp2pResolver, err := madns.NewResolver(madns.WithDefaultResolver(resolver))

		if err != nil {
			return nil, err
		}

		psOpts := DefaultPubsubOptions(maxPubSubMsgSize)

		var opts []libp2p.Option = []libp2p.Option{
			libp2p.ConnectionManager(connManager),
			libp2p.ConnectionGater(NewConnGater(log, func(pid peer.ID) bool {
				_, found := idProvider.ByPeerID(pid)

				return found
			})),
			libp2p.MultiaddrResolver(libp2pResolver),
			libp2p.Ping(true),
		}

		h, err := DefaultLibP2PHost(ctx, address, flowKey, opts...)

		if err != nil {
			return nil, err
		}

		if role != "ghost" {
			psOpts = append(psOpts, pubsub.WithSubscriptionFilter(NewRoleBasedFilter(h.ID(), idProvider)))
		}

		ps, err := DefaultPubSub(ctx, h, psOpts...)

		if err != nil {
			return nil, err
		}

		return NewNodeBuilder(log, h, ps, sporkId).
			SetConnectionManager(connManager).
			SetPingInfoProvider(pingInfoProvider).
			Build()
	}
}

type NodeBuilder interface {
	SetTopicValidation(bool) NodeBuilder
	SetConnectionManager(connmgr.ConnManager) NodeBuilder
	SetRoutingSystem(routing.Routing) NodeBuilder
	SetPingInfoProvider(PingInfoProvider) NodeBuilder
	Build() (*Node, error)
}

type LibP2PNodeBuilder struct {
	sporkID          flow.Identifier
	logger           zerolog.Logger
	host             host.Host
	pubSub           *pubsub.PubSub
	topicValidation  bool
	connMngr         connmgr.ConnManager
	rsys             routing.Routing
	pingInfoProvider PingInfoProvider
}

func NewNodeBuilder(logger zerolog.Logger, h host.Host, ps *pubsub.PubSub, sporkID flow.Identifier) *LibP2PNodeBuilder {
	return &LibP2PNodeBuilder{
		logger:          logger,
		host:            h,
		pubSub:          ps,
		topicValidation: true,
		sporkID:         sporkID,
	}
}

func (builder *LibP2PNodeBuilder) SetTopicValidation(enabled bool) NodeBuilder {
	builder.topicValidation = enabled
	return builder
}

func (builder *LibP2PNodeBuilder) SetConnectionManager(connMngr connmgr.ConnManager) NodeBuilder {
	builder.connMngr = connMngr
	return builder
}

func (builder *LibP2PNodeBuilder) SetRoutingSystem(rsys routing.Routing) NodeBuilder {
	builder.rsys = rsys
	return builder
}

func (builder *LibP2PNodeBuilder) SetPingInfoProvider(pingInfoProvider PingInfoProvider) NodeBuilder {
	builder.pingInfoProvider = pingInfoProvider
	return builder
}

func (builder *LibP2PNodeBuilder) Build() (*Node, error) {
	pCache, err := newProtocolPeerCache(builder.logger, builder.host)

	if err != nil {
		return nil, err
	}

	var pingService *PingService

	if builder.pingInfoProvider != nil {
		pingLibP2PProtocolID := unicast.PingProtocolId(builder.sporkID)
		pingService = NewPingService(builder.host, pingLibP2PProtocolID, builder.pingInfoProvider, builder.logger)
	}

	node := &Node{
		topics:          make(map[flownet.Topic]*pubsub.Topic),
		subs:            make(map[flownet.Topic]*pubsub.Subscription),
		logger:          builder.logger,
		topicValidation: builder.topicValidation,
		connMgr:         builder.connMngr,
		routing:         builder.rsys,
		host:            builder.host,
		unicastManager: unicast.NewUnicastManager(
			builder.logger,
			unicast.NewLibP2PStreamFactory(builder.host),
			builder.sporkID,
		),
		pCache:      pCache,
		pingService: pingService,
		pubSub:      builder.pubSub,
	}

	return node, nil
}

// DefaultLibP2PHost returns a libp2p host initialized to listen on the given address and using the given private key and
// customized with options
func DefaultLibP2PHost(ctx context.Context, address string, key fcrypto.PrivateKey, options ...config.Option) (host.Host,
	error) {
	defaultOptions, err := defaultLibP2POptions(address, key)
	if err != nil {
		return nil, err
	}

	allOptions := append(defaultOptions, options...)

	// create the libp2p host
	libP2PHost, err := libp2p.New(allOptions...)
	if err != nil {
		return nil, fmt.Errorf("could not create libp2p host: %w", err)
	}

	return libP2PHost, nil
}

// defaultLibP2POptions creates and returns the standard LibP2P host options that are used for the Flow Libp2p network
func defaultLibP2POptions(address string, key fcrypto.PrivateKey) ([]config.Option, error) {

	libp2pKey, err := keyutils.LibP2PPrivKeyFromFlow(key)
	if err != nil {
		return nil, fmt.Errorf("could not generate libp2p key: %w", err)
	}

	ip, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("could not split node address %s:%w", address, err)
	}

	sourceMultiAddr, err := multiaddr.NewMultiaddr(MultiAddressStr(ip, port))
	if err != nil {
		return nil, fmt.Errorf("failed to translate Flow address to Libp2p multiaddress: %w", err)
	}

	// create a transport which disables port reuse and web socket.
	// Port reuse enables listening and dialing from the same TCP port (https://github.com/libp2p/go-reuseport)
	// While this sounds great, it intermittently causes a 'broken pipe' error
	// as the 1-k discovery process and the 1-1 messaging both sometimes attempt to open connection to the same target
	// As of now there is no requirement of client sockets to be a well-known port, so disabling port reuse all together.
	transport := libp2p.Transport(func(u *stream.Upgrader) (*tcp.TcpTransport, error) {
		return tcp.NewTCPTransport(u, tcp.DisableReuseport())
	})

	// gather all the options for the libp2p node
	options := []config.Option{
		libp2p.ListenAddrs(sourceMultiAddr), // set the listen address
		libp2p.Identity(libp2pKey),          // pass in the networking key
		transport,                           // set the protocol
	}

	return options, nil
}

// DefaultPubSub returns initializes and returns a GossipSub object for the given libp2p host and options
func DefaultPubSub(ctx context.Context, host host.Host, psOption ...pubsub.Option) (*pubsub.PubSub, error) {
	// Creating a new PubSub instance of the type GossipSub with psOption
	pubSub, err := pubsub.NewGossipSub(ctx, host, psOption...)
	if err != nil {
		return nil, fmt.Errorf("could not create libp2p gossipsub: %w", err)
	}
	return pubSub, nil
}

func DefaultPubsubOptions(maxPubSubMsgSize int) []pubsub.Option {
	return []pubsub.Option{
		// skip message signing
		pubsub.WithMessageSigning(true),
		// skip message signature
		pubsub.WithStrictSignatureVerification(true),
		// set max message size limit for 1-k PubSub messaging
		pubsub.WithMaxMessageSize(maxPubSubMsgSize),
		// no discovery
	}
}
