package p2p

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p-core/connmgr"
	"github.com/libp2p/go-libp2p-core/host"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/config"
	madns "github.com/multiformats/go-multiaddr-dns"
	"github.com/rs/zerolog"

	fcrypto "github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/id"
	flownet "github.com/onflow/flow-go/network"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/network/p2p/unicast"
	"github.com/onflow/flow-go/utils/logging"
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
	rootBlockID flow.Identifier,
	idProvider id.IdentityProvider,
	maxPubSubMsgSize int,
	metrics module.NetworkMetrics,
	pingInfoProvider PingInfoProvider,
	dnsResolverTTL time.Duration,
	role string) (LibP2PFactoryFunc, error) {

	connManager := NewConnManager(log, metrics)

	connGater := NewConnGater(log)

	resolver := dns.NewResolver(metrics, dns.WithTTL(dnsResolverTTL))

	psOpts := DefaultPubsubOptions(maxPubSubMsgSize)

	if role != "ghost" {
		psOpts = append(psOpts, func(_ context.Context, h host.Host) (pubsub.Option, error) {
			return pubsub.WithSubscriptionFilter(NewRoleBasedFilter(
				h.ID(), rootBlockID, idProvider,
			)), nil
		})
	}

	return func(ctx context.Context) (*Node, error) {
		return NewDefaultLibP2PNodeBuilder(me, address, flowKey).
			SetRootBlockID(rootBlockID).
			SetConnectionGater(connGater).
			SetConnectionManager(connManager).
			SetPubsubOptions(psOpts...).
			SetPingInfoProvider(pingInfoProvider).
			SetLogger(log).
			SetResolver(resolver).
			Build(ctx)
	}, nil
}

type NodeBuilder interface {
	SetRootBlockID(flow.Identifier) NodeBuilder
	SetConnectionManager(connmgr.ConnManager) NodeBuilder
	SetConnectionGater(*ConnGater) NodeBuilder
	SetPubsubOptions(...PubsubOption) NodeBuilder
	SetPingInfoProvider(PingInfoProvider) NodeBuilder
	SetDHTOptions(...dht.Option) NodeBuilder
	SetTopicValidation(bool) NodeBuilder
	SetLogger(zerolog.Logger) NodeBuilder
	SetResolver(*dns.Resolver) NodeBuilder
	Build(context.Context) (*Node, error)
}

type DefaultLibP2PNodeBuilder struct {
	id               flow.Identifier
	rootBlockID      *flow.Identifier
	logger           zerolog.Logger
	connGater        *ConnGater
	connMngr         connmgr.ConnManager
	pingInfoProvider PingInfoProvider
	resolver         *dns.Resolver
	pubSubMaker      func(context.Context, host.Host, ...pubsub.Option) (*pubsub.PubSub, error)
	hostMaker        func(context.Context, ...config.Option) (host.Host, error)
	pubSubOpts       []PubsubOption
	dhtOpts          []dht.Option
	topicValidation  bool
}

func NewDefaultLibP2PNodeBuilder(id flow.Identifier, address string, flowKey fcrypto.PrivateKey) NodeBuilder {
	return &DefaultLibP2PNodeBuilder{
		id: id,
		pubSubMaker: func(ctx context.Context, h host.Host, opts ...pubsub.Option) (*pubsub.PubSub, error) {
			return DefaultPubSub(ctx, h, opts...)
		},
		hostMaker: func(ctx context.Context, opts ...config.Option) (host.Host, error) {
			return DefaultLibP2PHost(ctx, address, flowKey, opts...)
		},
		topicValidation: true,
	}
}

func (builder *DefaultLibP2PNodeBuilder) SetDHTOptions(opts ...dht.Option) NodeBuilder {
	builder.dhtOpts = opts
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetTopicValidation(enabled bool) NodeBuilder {
	builder.topicValidation = enabled
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetRootBlockID(rootBlockId flow.Identifier) NodeBuilder {
	builder.rootBlockID = &rootBlockId
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetConnectionManager(connMngr connmgr.ConnManager) NodeBuilder {
	builder.connMngr = connMngr
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetConnectionGater(connGater *ConnGater) NodeBuilder {
	builder.connGater = connGater
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetPubsubOptions(opts ...PubsubOption) NodeBuilder {
	builder.pubSubOpts = opts
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetPingInfoProvider(pingInfoProvider PingInfoProvider) NodeBuilder {
	builder.pingInfoProvider = pingInfoProvider
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetLogger(logger zerolog.Logger) NodeBuilder {
	builder.logger = logger
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) SetResolver(resolver *dns.Resolver) NodeBuilder {
	builder.resolver = resolver
	return builder
}

func (builder *DefaultLibP2PNodeBuilder) Build(ctx context.Context) (*Node, error) {
	node := &Node{
		id:              builder.id,
		topics:          make(map[flownet.Topic]*pubsub.Topic),
		subs:            make(map[flownet.Topic]*pubsub.Subscription),
		logger:          builder.logger,
		topicValidation: builder.topicValidation,
	}

	if builder.hostMaker == nil {
		return nil, errors.New("unable to create libp2p host: factory function not provided")
	}

	if builder.pubSubMaker == nil {
		return nil, errors.New("unable to create libp2p pubsub: factory function not provided")
	}

	if builder.rootBlockID == nil {
		return nil, errors.New("root block ID must be provided")
	}
	node.flowLibP2PProtocolID = unicast.FlowProtocolID(*builder.rootBlockID)

	var opts []config.Option

	if builder.connGater != nil {
		opts = append(opts, libp2p.ConnectionGater(builder.connGater))
		node.connGater = builder.connGater
	}

	if builder.connMngr != nil {
		opts = append(opts, libp2p.ConnectionManager(builder.connMngr))
		node.connMgr = builder.connMngr
	}

	if builder.pingInfoProvider != nil {
		opts = append(opts, libp2p.Ping(true))
	}

	if builder.resolver != nil { // sets DNS resolver
		libp2pResolver, err := madns.NewResolver(madns.WithDefaultResolver(builder.resolver))
		if err != nil {
			return nil, fmt.Errorf("could not create libp2p resolver: %w", err)
		}

		select {
		case <-builder.resolver.Ready():
		case <-time.After(30 * time.Second):
			return nil, fmt.Errorf("could not start resolver on time")
		}

		opts = append(opts, libp2p.MultiaddrResolver(libp2pResolver))
	}

	libp2pHost, err := builder.hostMaker(ctx, opts...)
	if err != nil {
		return nil, err
	}
	node.host = libp2pHost
	node.unicastManager = unicast.NewUnicastManager(builder.logger, node.host, *builder.rootBlockID)

	node.pCache, err = newProtocolPeerCache(node.logger, libp2pHost)
	if err != nil {
		return nil, err
	}

	if len(builder.dhtOpts) != 0 {
		kdht, err := NewDHT(ctx, node.host, builder.dhtOpts...)
		if err != nil {
			return nil, err
		}
		node.dht = kdht
		builder.pubSubOpts = append(builder.pubSubOpts, withDHTDiscovery(kdht))
	}

	if builder.pingInfoProvider != nil {
		pingLibP2PProtocolID := unicast.PingProtocolId(*builder.rootBlockID)
		pingService := NewPingService(libp2pHost, pingLibP2PProtocolID, builder.pingInfoProvider, node.logger)
		node.pingService = pingService
	}

	var libp2pPSOptions []pubsub.Option
	// generate the libp2p Pubsub options from the given context and host
	for _, optionGenerator := range builder.pubSubOpts {
		option, err := optionGenerator(ctx, libp2pHost)
		if err != nil {
			return nil, err
		}
		libp2pPSOptions = append(libp2pPSOptions, option)
	}

	ps, err := builder.pubSubMaker(ctx, libp2pHost, libp2pPSOptions...)
	if err != nil {
		return nil, err
	}
	node.pubSub = ps

	ip, port, err := node.GetIPPort()
	if err != nil {
		return nil, fmt.Errorf("failed to find IP and port on which the node was started: %w", err)
	}

	node.logger.Debug().
		Hex("node_id", logging.ID(node.id)).
		Str("address", fmt.Sprintf("%s:%s", ip, port)).
		Msg("libp2p node started successfully")

	return node, nil
}
