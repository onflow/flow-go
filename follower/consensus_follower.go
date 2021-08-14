package follower

import (
	"context"
	"sync"

	"github.com/onflow/flow-go/cmd"
	access "github.com/onflow/flow-go/cmd/access/node_builder"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

// ConsensusFollower is a standalone module run by third parties which provides
// a mechanism for observing the block chain. It maintains a set of subscribers
// and delivers block proposals broadcasted by the consensus nodes to each one.
type ConsensusFollower interface {
	// Run starts the consensus follower.
	Run(context.Context)
	// AddOnBlockFinalizedConsumer adds a new block finalization subscriber.
	AddOnBlockFinalizedConsumer(pubsub.OnBlockFinalizedConsumer)
}

// Config contains the configurable fields for a `ConsensusFollower`.
type Config struct {
	networkPubKey        crypto.PublicKey // the network public key of this node
	upstreamAccessNodeID flow.Identifier  // the node ID of the upstream access node
	bindAddr             string           // address to bind on
	dataDir              string           // directory to store the protocol state
	bootstrapDir         string           // path to the bootstrap directory
}

type Option func(c *Config)

func WithDataDir(dataDir string) Option {
	return func(cf *Config) {
		cf.dataDir = dataDir
	}
}

func WithBootstrapDir(bootstrapDir string) Option {
	return func(cf *Config) {
		cf.bootstrapDir = bootstrapDir
	}
}

func getAccessNodeOptions(config *Config) []access.Option {
	return []access.Option{
		access.WithUpstreamAccessNodeID(config.upstreamAccessNodeID),
		access.WithBindAddr(config.bindAddr),
		access.WithBaseOptions(getBaseOptions(config)),
	}
}

func getBaseOptions(config *Config) []cmd.Option {
	options := []cmd.Option{
		cmd.WithNetworkPublicKey(config.networkPubKey),
		cmd.WithMetricsEnabled(false),
	}
	if config.bootstrapDir != "" {
		options = append(options, cmd.WithBootstrapDir(config.bootstrapDir))
	}
	if config.dataDir != "" {
		options = append(options, cmd.WithDataDir(config.dataDir))
	}

	return options
}

func buildAccessNode(accessNodeOptions []access.Option) *access.UnstakedAccessNodeBuilder {
	anb := access.FlowAccessNode(accessNodeOptions...)
	nodeBuilder := access.NewUnstakedAccessNodeBuilder(anb)

	nodeBuilder.Initialize()
	nodeBuilder.BuildConsensusFollower()

	return nodeBuilder
}

type ConsensusFollowerImpl struct {
	nodeBuilder *access.UnstakedAccessNodeBuilder
	consumersMu sync.RWMutex
	consumers   []pubsub.OnBlockFinalizedConsumer
}

// NewConsensusFollower creates a new consensus follower.
func NewConsensusFollower(
	networkPublicKey crypto.PublicKey,
	upstreamAccessNodeID flow.Identifier,
	bindAddr string,
	opts ...Option,
) *ConsensusFollowerImpl {
	config := &Config{
		networkPublicKey:     networkPublicKey,
		upstreamAccessNodeID: upstreamAccessNodeID,
		bindAddr:             bindAddr,
	}

	for _, opt := range opts {
		opt(config)
	}

	accessNodeOptions := getAccessNodeOptions(config)
	anb := buildAccessNode(accessNodeOptions)

	consensusFollower := &ConsensusFollowerImpl{nodeBuilder: anb}

	anb.FinalizationDistributor.AddOnBlockFinalizedConsumer(consensusFollower.onBlockFinalized)

	return consensusFollower
}

// onBlockFinalized relays the block finalization event to all registered consumers.
func (cf *ConsensusFollowerImpl) onBlockFinalized(finalizedBlockID flow.Identifier) {
	cf.consumersMu.RLock()
	for _, consumer := range cf.consumers {
		cf.consumersMu.RUnlock()
		consumer(finalizedBlockID)
		cf.consumersMu.RLock()
	}
	cf.consumersMu.RUnlock()
}

// AddOnBlockFinalizedConsumer adds a new block finalization subscriber.
func (cf *ConsensusFollowerImpl) AddOnBlockFinalizedConsumer(consumer pubsub.OnBlockFinalizedConsumer) {
	cf.consumersMu.Lock()
	defer cf.consumersMu.Unlock()
	cf.consumers = append(cf.consumers, consumer)
}

// Run starts the consensus follower.
func (cf *ConsensusFollowerImpl) Run(ctx context.Context) {
	runAccessNode(ctx, cf.nodeBuilder)
}

func runAccessNode(ctx context.Context, anb *access.UnstakedAccessNodeBuilder) {
	select {
	case <-ctx.Done():
		return
	default:
	}

	select {
	case <-anb.Ready():
		anb.Logger.Info().Msg("Access node startup complete")
	case <-ctx.Done():
		anb.Logger.Info().Msg("Access node startup aborted")
	}

	<-ctx.Done()
	anb.Logger.Info().Msg("Access node shutting down")
	<-anb.Done()
	anb.Logger.Info().Msg("Access node shutdown complete")
}
