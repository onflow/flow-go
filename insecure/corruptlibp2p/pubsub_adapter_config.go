package corruptlibp2p

import (
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	pb "github.com/libp2p/go-libp2p-pubsub/pb"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/routing"
	discoveryRouting "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	corrupt "github.com/yhassanzadeh13/go-libp2p-pubsub"

	"github.com/onflow/flow-go/network/p2p"
)

// CorruptPubSubAdapterConfig is a wrapper around the forked pubsub topic from
// github.com/yhassanzadeh13/go-libp2p-pubsub that implements the p2p.PubSubAdapterConfig.
// This is needed because in order to use the forked pubsub module, we need to
// use the entire dependency tree of the forked module which is resolved to
// github.com/yhassanzadeh13/go-libp2p-pubsub. This means that we cannot use
// the original libp2p pubsub module in the same package.
// Note: we use the forked pubsub module for sake of BFT testing and attack vector
// implementation, it is designed to be completely isolated in the "insecure" package, and
// totally separated from the rest of the codebase.
type CorruptPubSubAdapterConfig struct {
	options                         []corrupt.Option
	inspector                       func(peer.ID, *corrupt.RPC) error
	withMessageSigning              bool
	withStrictSignatureVerification bool
}

type CorruptPubSubAdapterConfigOption func(config *CorruptPubSubAdapterConfig)

// WithMessageSigning overrides the libp2p node message signing option. This option can be used to enable or disable message signing.
func WithMessageSigning(withMessageSigning bool) CorruptPubSubAdapterConfigOption {
	return func(config *CorruptPubSubAdapterConfig) {
		config.withMessageSigning = withMessageSigning
	}
}

// WithStrictSignatureVerification overrides the libp2p node message signature verification option. This option can be used to enable or disable message signature verification.
func WithStrictSignatureVerification(withStrictSignatureVerification bool) CorruptPubSubAdapterConfigOption {
	return func(config *CorruptPubSubAdapterConfig) {
		config.withStrictSignatureVerification = withStrictSignatureVerification
	}
}

var _ p2p.PubSubAdapterConfig = (*CorruptPubSubAdapterConfig)(nil)

func WithInspector(inspector func(peer.ID, *corrupt.RPC) error) func(config *CorruptPubSubAdapterConfig) {
	return func(config *CorruptPubSubAdapterConfig) {
		config.inspector = inspector
		config.options = append(config.options, corrupt.WithAppSpecificRpcInspector(func(id peer.ID, rpc *corrupt.RPC) error {
			return config.inspector(id, rpc)
		}))
	}
}

func NewCorruptPubSubAdapterConfig(base *p2p.BasePubSubAdapterConfig, opts ...CorruptPubSubAdapterConfigOption) *CorruptPubSubAdapterConfig {
	cfg := &CorruptPubSubAdapterConfig{
		options: defaultCorruptPubsubOptions(base),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	return cfg
}

//

func (c *CorruptPubSubAdapterConfig) WithRoutingDiscovery(routing routing.ContentRouting) {
	c.options = append(c.options, corrupt.WithDiscovery(discoveryRouting.NewRoutingDiscovery(routing)))
}

func (c *CorruptPubSubAdapterConfig) WithSubscriptionFilter(filter p2p.SubscriptionFilter) {
	c.options = append(c.options, corrupt.WithSubscriptionFilter(filter))
}

func (c *CorruptPubSubAdapterConfig) WithScoreOption(_ p2p.ScoreOptionBuilder) {
	// Corrupt does not support score options. This is a no-op.
}

func (c *CorruptPubSubAdapterConfig) WithAppSpecificRpcInspector(_ func(peer.ID, *pubsub.RPC) error) {
	// Corrupt does not support app specific rpc inspectors. This is a no-op.
}

func (c *CorruptPubSubAdapterConfig) WithMessageIdFunction(f func([]byte) string) {
	if c.inspector != nil {
		c.options = append(c.options, corrupt.WithAppSpecificRpcInspector(func(id peer.ID, rpc *corrupt.RPC) error {
			return c.inspector(id, rpc)
		}))
	}

	c.options = append(c.options, corrupt.WithMessageIdFn(func(pmsg *pb.Message) string {
		return f(pmsg.Data)
	}))
}

func (c *CorruptPubSubAdapterConfig) Build() []corrupt.Option {
	return c.options
}

func defaultCorruptPubsubOptions(base *p2p.BasePubSubAdapterConfig) []corrupt.Option {
	return []corrupt.Option{
		corrupt.WithMessageSigning(true),
		corrupt.WithStrictSignatureVerification(true),
		corrupt.WithMaxMessageSize(base.MaxMessageSize),
	}
}
