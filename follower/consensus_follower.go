package follower

import (
	"context"
	"fmt"
	"sync"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/cmd"
	access "github.com/onflow/flow-go/cmd/access/node_builder"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications/pubsub"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
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
	networkPrivKey crypto.PrivateKey   // the network private key of this node
	bootstrapNodes []BootstrapNodeInfo // the bootstrap nodes to use
	bindAddr       string              // address to bind on
	db             *badger.DB          // the badger DB storage to use for the protocol state
	dataDir        string              // directory to store the protocol state (if the badger storage is not provided)
	bootstrapDir   string              // path to the bootstrap directory
	logLevel       string              // log level
}

type Option func(c *Config)

// WithDataDir sets the underlying directory to be used to store the database
// If a database is supplied, then data directory will be set to empty string
func WithDataDir(dataDir string) Option {
	return func(cf *Config) {
		if cf.db == nil {
			cf.dataDir = dataDir
		}
	}
}

func WithBootstrapDir(bootstrapDir string) Option {
	return func(cf *Config) {
		cf.bootstrapDir = bootstrapDir
	}
}

func WithLogLevel(level string) Option {
	return func(cf *Config) {
		cf.logLevel = level
	}
}

// WithDB sets the underlying database that will be used to store the chain state
// WithDB takes precedence over WithDataDir and datadir will be set to empty if DB is set using this option
func WithDB(db *badger.DB) Option {
	return func(cf *Config) {
		cf.db = db
		cf.dataDir = ""
	}
}

// BootstrapNodeInfo contains the details about the upstream bootstrap peer the consensus follower uses
type BootstrapNodeInfo struct {
	Host             string // ip or hostname
	Port             uint
	NetworkPublicKey crypto.PublicKey // the network public key of the bootstrap peer
}

func bootstrapIdentities(bootstrapNodes []BootstrapNodeInfo) flow.IdentityList {
	ids := make(flow.IdentityList, len(bootstrapNodes))
	for i, b := range bootstrapNodes {
		ids[i] = &flow.Identity{
			Role:          flow.RoleAccess,
			NetworkPubKey: b.NetworkPublicKey,
			Address:       fmt.Sprintf("%s:%d", b.Host, b.Port),
			StakingPubKey: nil,
		}
	}
	return ids
}

func getAccessNodeOptions(config *Config) []access.Option {
	ids := bootstrapIdentities(config.bootstrapNodes)
	return []access.Option{
		access.WithBootStrapPeers(ids...),
		access.WithBaseOptions(getBaseOptions(config)),
		access.WithNetworkKey(config.networkPrivKey),
	}
}

func getBaseOptions(config *Config) []cmd.Option {
	options := []cmd.Option{
		cmd.WithMetricsEnabled(false),
		cmd.WithSecretsDBEnabled(false),
	}
	if config.bootstrapDir != "" {
		options = append(options, cmd.WithBootstrapDir(config.bootstrapDir))
	}
	if config.dataDir != "" {
		options = append(options, cmd.WithDataDir(config.dataDir))
	}
	if config.bindAddr != "" {
		options = append(options, cmd.WithBindAddress(config.bindAddr))
	}
	if config.logLevel != "" {
		options = append(options, cmd.WithLogLevel(config.logLevel))
	}
	if config.db != nil {
		options = append(options, cmd.WithDB(config.db))
	}

	return options
}

func buildAccessNode(accessNodeOptions []access.Option) (*access.UnstakedAccessNodeBuilder, error) {
	anb := access.FlowAccessNode(accessNodeOptions...)
	nodeBuilder := access.NewUnstakedAccessNodeBuilder(anb)

	if err := nodeBuilder.Initialize(); err != nil {
		return nil, err
	}

	return nodeBuilder, nil
}

type ConsensusFollowerImpl struct {
	module.ReadyDoneAware
	*cmd.NodeConfig
	Logger      zerolog.Logger
	consumersMu sync.RWMutex
	consumers   []pubsub.OnBlockFinalizedConsumer
}

// NewConsensusFollower creates a new consensus follower.
func NewConsensusFollower(
	networkPrivKey crypto.PrivateKey,
	bindAddr string,
	bootstapIdentities []BootstrapNodeInfo,
	opts ...Option,
) (*ConsensusFollowerImpl, error) {
	config := &Config{
		networkPrivKey: networkPrivKey,
		bootstrapNodes: bootstapIdentities,
		bindAddr:       bindAddr,
		logLevel:       "info",
	}

	for _, opt := range opts {
		opt(config)
	}

	accessNodeOptions := getAccessNodeOptions(config)
	anb, err := buildAccessNode(accessNodeOptions)
	if err != nil {
		return nil, err
	}

	cf := &ConsensusFollowerImpl{Logger: anb.Logger}
	anb.BaseConfig.NodeRole = "consensus_follower"
	anb.FinalizationDistributor.AddOnBlockFinalizedConsumer(cf.onBlockFinalized)
	cf.NodeConfig = anb.NodeConfig
	cf.ReadyDoneAware = anb.Build()

	return cf, nil
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
	select {
	case <-ctx.Done():
		return
	default:
	}

	select {
	case <-cf.Ready():
		cf.Logger.Info().Msg("Access node startup complete")
	case <-ctx.Done():
		cf.Logger.Info().Msg("Access node startup aborted")
	}

	<-ctx.Done()
	cf.Logger.Info().Msg("Access node shutting down")
	<-cf.Done()
	cf.Logger.Info().Msg("Access node shutdown complete")
}
