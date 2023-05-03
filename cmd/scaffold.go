package cmd

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"strings"
	"time"

	gcemd "cloud.google.com/go/compute/metadata"
	"github.com/dgraph-io/badger/v2"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"golang.org/x/time/rate"
	"google.golang.org/api/option"

	"github.com/onflow/flow-go/admin"
	"github.com/onflow/flow-go/admin/commands"
	"github.com/onflow/flow-go/admin/commands/common"
	storageCommands "github.com/onflow/flow-go/admin/commands/storage"
	"github.com/onflow/flow-go/cmd/build"
	"github.com/onflow/flow-go/consensus/hotstuff/persister"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/chainsync"
	"github.com/onflow/flow-go/module/compliance"
	"github.com/onflow/flow-go/module/component"
	"github.com/onflow/flow-go/module/id"
	"github.com/onflow/flow-go/module/irrecoverable"
	"github.com/onflow/flow-go/module/local"
	"github.com/onflow/flow-go/module/mempool/herocache"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/profiler"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/module/updatable_configs"
	"github.com/onflow/flow-go/module/util"
	"github.com/onflow/flow-go/network"
	netcache "github.com/onflow/flow-go/network/cache"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/cache"
	"github.com/onflow/flow-go/network/p2p/conduit"
	"github.com/onflow/flow-go/network/p2p/dns"
	"github.com/onflow/flow-go/network/p2p/inspector/validation"
	"github.com/onflow/flow-go/network/p2p/middleware"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder/inspector"
	"github.com/onflow/flow-go/network/p2p/ping"
	"github.com/onflow/flow-go/network/p2p/subscription"
	"github.com/onflow/flow-go/network/p2p/unicast/protocols"
	"github.com/onflow/flow-go/network/p2p/unicast/ratelimit"
	"github.com/onflow/flow-go/network/p2p/utils/ratelimiter"
	"github.com/onflow/flow-go/network/slashing"
	"github.com/onflow/flow-go/network/topology"
	"github.com/onflow/flow-go/state/protocol"
	badgerState "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/state/protocol/events/gadgets"
	"github.com/onflow/flow-go/storage"
	bstorage "github.com/onflow/flow-go/storage/badger"
	"github.com/onflow/flow-go/storage/badger/operation"
	sutil "github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	NetworkComponent        = "network"
	ConduitFactoryComponent = "conduit-factory"
	LibP2PNodeComponent     = "libp2p-node"
)

type Metrics struct {
	Network        module.NetworkMetrics
	Engine         module.EngineMetrics
	Compliance     module.ComplianceMetrics
	Cache          module.CacheMetrics
	Mempool        module.MempoolMetrics
	CleanCollector module.CleanerMetrics
	Bitswap        module.BitswapMetrics
}

type Storage = storage.All

type namedModuleFunc struct {
	fn   BuilderFunc
	name string
}

type namedComponentFunc struct {
	fn   ReadyDoneFactory
	name string

	errorHandler component.OnError
	dependencies *DependencyList
}

// FlowNodeBuilder is the default builder struct used for all flow nodes
// It runs a node process with following structure, in sequential order
// Base inits (network, storage, state, logger)
// PostInit handlers, if any
// Components handlers, if any, wait sequentially
// Run() <- main loop
// Components destructors, if any
// The initialization can be proceeded and succeeded with  PreInit and PostInit functions that allow customization
// of the process in case of nodes such as the unstaked access node where the NodeInfo is not part of the genesis data
type FlowNodeBuilder struct {
	*NodeConfig
	flags                    *pflag.FlagSet
	modules                  []namedModuleFunc
	components               []namedComponentFunc
	postShutdownFns          []func() error
	preInitFns               []BuilderFunc
	postInitFns              []BuilderFunc
	extraRootSnapshotCheck   func(protocol.Snapshot) error
	extraFlagCheck           func() error
	adminCommandBootstrapper *admin.CommandRunnerBootstrapper
	adminCommands            map[string]func(config *NodeConfig) commands.AdminCommand
	componentBuilder         component.ComponentManagerBuilder
}

var _ NodeBuilder = (*FlowNodeBuilder)(nil)

func (fnb *FlowNodeBuilder) BaseFlags() {
	defaultConfig := DefaultBaseConfig()

	// bind configuration parameters
	fnb.flags.StringVar(&fnb.BaseConfig.nodeIDHex, "nodeid", defaultConfig.nodeIDHex, "identity of our node")
	fnb.flags.StringVar(&fnb.BaseConfig.BindAddr, "bind", defaultConfig.BindAddr, "address to bind on")
	fnb.flags.StringVarP(&fnb.BaseConfig.BootstrapDir, "bootstrapdir", "b", defaultConfig.BootstrapDir, "path to the bootstrap directory")
	fnb.flags.StringVarP(&fnb.BaseConfig.datadir, "datadir", "d", defaultConfig.datadir, "directory to store the public database (protocol state)")
	fnb.flags.StringVar(&fnb.BaseConfig.secretsdir, "secretsdir", defaultConfig.secretsdir, "directory to store private database (secrets)")
	fnb.flags.StringVarP(&fnb.BaseConfig.level, "loglevel", "l", defaultConfig.level, "level for logging output")
	fnb.flags.Uint32Var(&fnb.BaseConfig.debugLogLimit, "debug-log-limit", defaultConfig.debugLogLimit, "max number of debug/trace log events per second")
	fnb.flags.DurationVar(&fnb.BaseConfig.PeerUpdateInterval, "peerupdate-interval", defaultConfig.PeerUpdateInterval, "how often to refresh the peer connections for the node")
	fnb.flags.DurationVar(&fnb.BaseConfig.UnicastMessageTimeout, "unicast-timeout", defaultConfig.UnicastMessageTimeout, "how long a unicast transmission can take to complete")
	fnb.flags.UintVarP(&fnb.BaseConfig.metricsPort, "metricport", "m", defaultConfig.metricsPort, "port for /metrics endpoint")
	fnb.flags.BoolVar(&fnb.BaseConfig.profilerConfig.Enabled, "profiler-enabled", defaultConfig.profilerConfig.Enabled, "whether to enable the auto-profiler")
	fnb.flags.BoolVar(&fnb.BaseConfig.profilerConfig.UploaderEnabled, "profile-uploader-enabled", defaultConfig.profilerConfig.UploaderEnabled,
		"whether to enable automatic profile upload to Google Cloud Profiler. "+
			"For autoupload to work forllowing should be true: "+
			"1) both -profiler-enabled=true and -profile-uploader-enabled=true need to be set. "+
			"2) node is running in GCE. "+
			"3) server or user has https://www.googleapis.com/auth/monitoring.write scope. ")
	fnb.flags.StringVar(&fnb.BaseConfig.profilerConfig.Dir, "profiler-dir", defaultConfig.profilerConfig.Dir, "directory to create auto-profiler profiles")
	fnb.flags.DurationVar(&fnb.BaseConfig.profilerConfig.Interval, "profiler-interval", defaultConfig.profilerConfig.Interval,
		"the interval between auto-profiler runs")
	fnb.flags.DurationVar(&fnb.BaseConfig.profilerConfig.Duration, "profiler-duration", defaultConfig.profilerConfig.Duration,
		"the duration to run the auto-profile for")

	fnb.flags.BoolVar(&fnb.BaseConfig.tracerEnabled, "tracer-enabled", defaultConfig.tracerEnabled,
		"whether to enable tracer")
	fnb.flags.UintVar(&fnb.BaseConfig.tracerSensitivity, "tracer-sensitivity", defaultConfig.tracerSensitivity,
		"adjusts the level of sampling when tracing is enabled. 0 means capture everything, higher value results in less samples")

	fnb.flags.StringVar(&fnb.BaseConfig.AdminAddr, "admin-addr", defaultConfig.AdminAddr, "address to bind on for admin HTTP server")
	fnb.flags.StringVar(&fnb.BaseConfig.AdminCert, "admin-cert", defaultConfig.AdminCert, "admin cert file (for TLS)")
	fnb.flags.StringVar(&fnb.BaseConfig.AdminKey, "admin-key", defaultConfig.AdminKey, "admin key file (for TLS)")
	fnb.flags.StringVar(&fnb.BaseConfig.AdminClientCAs, "admin-client-certs", defaultConfig.AdminClientCAs, "admin client certs (for mutual TLS)")
	fnb.flags.UintVar(&fnb.BaseConfig.AdminMaxMsgSize, "admin-max-response-size", defaultConfig.AdminMaxMsgSize, "admin server max response size in bytes")

	fnb.flags.Float64Var(&fnb.BaseConfig.LibP2PResourceManagerConfig.FileDescriptorsRatio, "libp2p-fd-ratio", defaultConfig.LibP2PResourceManagerConfig.FileDescriptorsRatio, "ratio of available file descriptors to be used by libp2p (in (0,1])")
	fnb.flags.Float64Var(&fnb.BaseConfig.LibP2PResourceManagerConfig.MemoryLimitRatio, "libp2p-memory-limit", defaultConfig.LibP2PResourceManagerConfig.MemoryLimitRatio, "ratio of available memory to be used by libp2p (in (0,1])")
	fnb.flags.IntVar(&fnb.BaseConfig.LibP2PResourceManagerConfig.PeerBaseLimitConnsInbound, "libp2p-inbound-conns-limit", defaultConfig.LibP2PResourceManagerConfig.PeerBaseLimitConnsInbound, "the maximum amount of allowed inbound connections per peer")
	fnb.flags.IntVar(&fnb.BaseConfig.ConnectionManagerConfig.LowWatermark, "libp2p-connmgr-low", defaultConfig.ConnectionManagerConfig.LowWatermark, "low watermarking for libp2p connection manager")
	fnb.flags.IntVar(&fnb.BaseConfig.ConnectionManagerConfig.HighWatermark, "libp2p-connmgr-high", defaultConfig.ConnectionManagerConfig.HighWatermark, "high watermarking for libp2p connection manager")
	fnb.flags.DurationVar(&fnb.BaseConfig.ConnectionManagerConfig.GracePeriod, "libp2p-connmgr-grace", defaultConfig.ConnectionManagerConfig.GracePeriod, "grace period for libp2p connection manager")
	fnb.flags.DurationVar(&fnb.BaseConfig.ConnectionManagerConfig.SilencePeriod, "libp2p-connmgr-silence", defaultConfig.ConnectionManagerConfig.SilencePeriod, "silence period for libp2p connection manager")

	fnb.flags.DurationVar(&fnb.BaseConfig.DNSCacheTTL, "dns-cache-ttl", defaultConfig.DNSCacheTTL, "time-to-live for dns cache")
	fnb.flags.StringSliceVar(&fnb.BaseConfig.PreferredUnicastProtocols, "preferred-unicast-protocols", nil, "preferred unicast protocols in ascending order of preference")
	fnb.flags.Uint32Var(&fnb.BaseConfig.NetworkReceivedMessageCacheSize, "networking-receive-cache-size", p2p.DefaultReceiveCacheSize,
		"incoming message cache size at networking layer")
	fnb.flags.BoolVar(&fnb.BaseConfig.NetworkConnectionPruning, "networking-connection-pruning", defaultConfig.NetworkConnectionPruning, "enabling connection trimming")
	fnb.flags.BoolVar(&fnb.BaseConfig.GossipSubConfig.PeerScoring, "peer-scoring-enabled", defaultConfig.GossipSubConfig.PeerScoring, "enabling peer scoring on pubsub network")
	fnb.flags.DurationVar(&fnb.BaseConfig.GossipSubConfig.LocalMeshLogInterval, "gossipsub-local-mesh-logging-interval", defaultConfig.GossipSubConfig.LocalMeshLogInterval, "logging interval for local mesh in gossipsub")
	fnb.flags.DurationVar(&fnb.BaseConfig.GossipSubConfig.ScoreTracerInterval, "gossipsub-score-tracer-interval", defaultConfig.GossipSubConfig.ScoreTracerInterval, "logging interval for peer score tracer in gossipsub, set to 0 to disable")
	fnb.flags.UintVar(&fnb.BaseConfig.guaranteesCacheSize, "guarantees-cache-size", bstorage.DefaultCacheSize, "collection guarantees cache size")
	fnb.flags.UintVar(&fnb.BaseConfig.receiptsCacheSize, "receipts-cache-size", bstorage.DefaultCacheSize, "receipts cache size")

	// dynamic node startup flags
	fnb.flags.StringVar(&fnb.BaseConfig.DynamicStartupANPubkey, "dynamic-startup-access-publickey", "", "the public key of the trusted secure access node to connect to when using dynamic-startup, this access node must be staked")
	fnb.flags.StringVar(&fnb.BaseConfig.DynamicStartupANAddress, "dynamic-startup-access-address", "", "the access address of the trusted secure access node to connect to when using dynamic-startup, this access node must be staked")
	fnb.flags.StringVar(&fnb.BaseConfig.DynamicStartupEpochPhase, "dynamic-startup-epoch-phase", "EpochPhaseSetup", "the target epoch phase for dynamic startup <EpochPhaseStaking|EpochPhaseSetup|EpochPhaseCommitted")
	fnb.flags.StringVar(&fnb.BaseConfig.DynamicStartupEpoch, "dynamic-startup-epoch", "current", "the target epoch for dynamic-startup, use \"current\" to start node in the current epoch")
	fnb.flags.DurationVar(&fnb.BaseConfig.DynamicStartupSleepInterval, "dynamic-startup-sleep-interval", time.Minute, "the interval in which the node will check if it can start")

	fnb.flags.BoolVar(&fnb.BaseConfig.InsecureSecretsDB, "insecure-secrets-db", false, "allow the node to start up without an secrets DB encryption key")
	fnb.flags.BoolVar(&fnb.BaseConfig.HeroCacheMetricsEnable, "herocache-metrics-collector", false, "enables herocache metrics collection")

	// sync core flags
	fnb.flags.DurationVar(&fnb.BaseConfig.SyncCoreConfig.RetryInterval, "sync-retry-interval", defaultConfig.SyncCoreConfig.RetryInterval, "the initial interval before we retry a sync request, uses exponential backoff")
	fnb.flags.UintVar(&fnb.BaseConfig.SyncCoreConfig.Tolerance, "sync-tolerance", defaultConfig.SyncCoreConfig.Tolerance, "determines how big of a difference in block heights we tolerate before actively syncing with range requests")
	fnb.flags.UintVar(&fnb.BaseConfig.SyncCoreConfig.MaxAttempts, "sync-max-attempts", defaultConfig.SyncCoreConfig.MaxAttempts, "the maximum number of attempts we make for each requested block/height before discarding")
	fnb.flags.UintVar(&fnb.BaseConfig.SyncCoreConfig.MaxSize, "sync-max-size", defaultConfig.SyncCoreConfig.MaxSize, "the maximum number of blocks we request in the same block request message")
	fnb.flags.UintVar(&fnb.BaseConfig.SyncCoreConfig.MaxRequests, "sync-max-requests", defaultConfig.SyncCoreConfig.MaxRequests, "the maximum number of requests we send during each scanning period")

	fnb.flags.Uint64Var(&fnb.BaseConfig.ComplianceConfig.SkipNewProposalsThreshold, "compliance-skip-proposals-threshold", defaultConfig.ComplianceConfig.SkipNewProposalsThreshold, "threshold at which new proposals are discarded rather than cached, if their height is this much above local finalized height")

	// unicast stream handler rate limits
	fnb.flags.IntVar(&fnb.BaseConfig.UnicastRateLimitersConfig.MessageRateLimit, "unicast-message-rate-limit", defaultConfig.UnicastRateLimitersConfig.MessageRateLimit, "maximum number of unicast messages that a peer can send per second")
	fnb.flags.IntVar(&fnb.BaseConfig.UnicastRateLimitersConfig.BandwidthRateLimit, "unicast-bandwidth-rate-limit", defaultConfig.UnicastRateLimitersConfig.BandwidthRateLimit, "bandwidth size in bytes a peer is allowed to send via unicast streams per second")
	fnb.flags.IntVar(&fnb.BaseConfig.UnicastRateLimitersConfig.BandwidthBurstLimit, "unicast-bandwidth-burst-limit", defaultConfig.UnicastRateLimitersConfig.BandwidthBurstLimit, "bandwidth size in bytes a peer is allowed to send at one time")
	fnb.flags.DurationVar(&fnb.BaseConfig.UnicastRateLimitersConfig.LockoutDuration, "unicast-rate-limit-lockout-duration", defaultConfig.UnicastRateLimitersConfig.LockoutDuration, "the number of seconds a peer will be forced to wait before being allowed to successful reconnect to the node after being rate limited")
	fnb.flags.BoolVar(&fnb.BaseConfig.UnicastRateLimitersConfig.DryRun, "unicast-rate-limit-dry-run", defaultConfig.UnicastRateLimitersConfig.DryRun, "disable peer disconnects and connections gating when rate limiting peers")

	// gossipsub RPC control message validation limits used for validation configuration and rate limiting
	fnb.flags.IntVar(&fnb.BaseConfig.GossipSubRPCInspectorsConfig.ValidationInspectorConfigs.NumberOfWorkers, "gossipsub-rpc-validation-inspector-workers", defaultConfig.GossipSubRPCInspectorsConfig.ValidationInspectorConfigs.NumberOfWorkers, "number of gossupsub RPC control message validation inspector component workers")
	fnb.flags.Uint32Var(&fnb.BaseConfig.GossipSubRPCInspectorsConfig.ValidationInspectorConfigs.CacheSize, "gossipsub-rpc-validation-inspector-cache-size", defaultConfig.GossipSubRPCInspectorsConfig.ValidationInspectorConfigs.CacheSize, "cache size for gossipsub RPC validation inspector events worker pool queue.")
	fnb.flags.StringToIntVar(&fnb.BaseConfig.GossipSubRPCInspectorsConfig.ValidationInspectorConfigs.GraftLimits, "gossipsub-rpc-graft-limits", defaultConfig.GossipSubRPCInspectorsConfig.ValidationInspectorConfigs.GraftLimits, fmt.Sprintf("discard threshold, safety and rate limits for gossipsub RPC GRAFT message validation e.g: %s=1000,%s=100,%s=1000", validation.DiscardThresholdMapKey, validation.SafetyThresholdMapKey, validation.RateLimitMapKey))
	fnb.flags.StringToIntVar(&fnb.BaseConfig.GossipSubRPCInspectorsConfig.ValidationInspectorConfigs.PruneLimits, "gossipsub-rpc-prune-limits", defaultConfig.GossipSubRPCInspectorsConfig.ValidationInspectorConfigs.PruneLimits, fmt.Sprintf("discard threshold, safety and rate limits for gossipsub RPC PRUNE message validation e.g: %s=1000,%s=20,%s=1000", validation.DiscardThresholdMapKey, validation.SafetyThresholdMapKey, validation.RateLimitMapKey))
	// gossipsub RPC control message metrics observer inspector configuration
	fnb.flags.IntVar(&fnb.BaseConfig.GossipSubRPCInspectorsConfig.MetricsInspectorConfigs.NumberOfWorkers, "gossipsub-rpc-metrics-inspector-workers", defaultConfig.GossipSubRPCInspectorsConfig.MetricsInspectorConfigs.NumberOfWorkers, "cache size for gossipsub RPC metrics inspector events worker pool queue.")
	fnb.flags.Uint32Var(&fnb.BaseConfig.GossipSubRPCInspectorsConfig.MetricsInspectorConfigs.CacheSize, "gossipsub-rpc-metrics-inspector-cache-size", defaultConfig.GossipSubRPCInspectorsConfig.MetricsInspectorConfigs.CacheSize, "cache size for gossipsub RPC metrics inspector events worker pool.")

	// networking event notifications
	fnb.flags.Uint32Var(&fnb.BaseConfig.GossipSubRPCInspectorsConfig.GossipSubRPCInspectorNotificationCacheSize, "gossipsub-rpc-inspector-notification-cache-size", defaultConfig.GossipSubRPCInspectorsConfig.GossipSubRPCInspectorNotificationCacheSize, "cache size for notification events from gossipsub rpc inspector")
	fnb.flags.Uint32Var(&fnb.BaseConfig.DisallowListNotificationCacheSize, "disallow-list-notification-cache-size", defaultConfig.DisallowListNotificationCacheSize, "cache size for notification events from disallow list")

	// unicast manager options
	fnb.flags.DurationVar(&fnb.BaseConfig.UnicastCreateStreamRetryDelay, "unicast-manager-create-stream-retry-delay", defaultConfig.NetworkConfig.UnicastCreateStreamRetryDelay, "Initial delay between failing to establish a connection with another node and retrying. This delay increases exponentially (exponential backoff) with the number of subsequent failures to establish a connection.")
}

func (fnb *FlowNodeBuilder) EnqueuePingService() {
	fnb.Component("ping service", func(node *NodeConfig) (module.ReadyDoneAware, error) {
		pingLibP2PProtocolID := protocols.PingProtocolId(node.SporkID)

		// setup the Ping provider to return the software version and the sealed block height
		pingInfoProvider := &ping.InfoProvider{
			SoftwareVersionFun: func() string {
				return build.Semver()
			},
			SealedBlockHeightFun: func() (uint64, error) {
				head, err := node.State.Sealed().Head()
				if err != nil {
					return 0, err
				}
				return head.Height, nil
			},
			HotstuffViewFun: func() (uint64, error) {
				return 0, fmt.Errorf("hotstuff view reporting disabled")
			},
		}

		// only consensus roles will need to report hotstuff view
		if fnb.BaseConfig.NodeRole == flow.RoleConsensus.String() {
			// initialize the persister
			persist := persister.New(node.DB, node.RootChainID)

			pingInfoProvider.HotstuffViewFun = func() (uint64, error) {
				livenessData, err := persist.GetLivenessData()
				if err != nil {
					return 0, err
				}

				return livenessData.CurrentView, nil
			}
		}

		pingService, err := node.Network.RegisterPingService(pingLibP2PProtocolID, pingInfoProvider)

		node.PingService = pingService

		return &module.NoopReadyDoneAware{}, err
	})
}

func (fnb *FlowNodeBuilder) EnqueueResolver() {
	fnb.Component("resolver", func(node *NodeConfig) (module.ReadyDoneAware, error) {
		var dnsIpCacheMetricsCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
		var dnsTxtCacheMetricsCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
		if fnb.HeroCacheMetricsEnable {
			dnsIpCacheMetricsCollector = metrics.NetworkDnsIpCacheMetricsFactory(fnb.MetricsRegisterer)
			dnsTxtCacheMetricsCollector = metrics.NetworkDnsTxtCacheMetricsFactory(fnb.MetricsRegisterer)
		}

		cache := herocache.NewDNSCache(
			dns.DefaultCacheSize,
			node.Logger,
			dnsIpCacheMetricsCollector,
			dnsTxtCacheMetricsCollector,
		)

		resolver := dns.NewResolver(
			node.Logger,
			fnb.Metrics.Network,
			cache,
			dns.WithTTL(fnb.BaseConfig.DNSCacheTTL))

		fnb.Resolver = resolver
		return resolver, nil
	})
}

func (fnb *FlowNodeBuilder) EnqueueNetworkInit() {
	connGaterPeerDialFilters := make([]p2p.PeerFilter, 0)
	connGaterInterceptSecureFilters := make([]p2p.PeerFilter, 0)
	peerManagerFilters := make([]p2p.PeerFilter, 0)

	fnb.UnicastRateLimiterDistributor = ratelimit.NewUnicastRateLimiterDistributor()
	fnb.UnicastRateLimiterDistributor.AddConsumer(fnb.Metrics.Network)

	// setup default rate limiter options
	unicastRateLimiterOpts := []ratelimit.RateLimitersOption{
		ratelimit.WithDisabledRateLimiting(fnb.BaseConfig.UnicastRateLimitersConfig.DryRun),
		ratelimit.WithNotifier(fnb.UnicastRateLimiterDistributor),
	}

	// override noop unicast message rate limiter
	if fnb.BaseConfig.UnicastRateLimitersConfig.MessageRateLimit > 0 {
		unicastMessageRateLimiter := ratelimiter.NewRateLimiter(
			rate.Limit(fnb.BaseConfig.UnicastRateLimitersConfig.MessageRateLimit),
			fnb.BaseConfig.UnicastRateLimitersConfig.MessageRateLimit,
			fnb.BaseConfig.UnicastRateLimitersConfig.LockoutDuration,
		)
		unicastRateLimiterOpts = append(unicastRateLimiterOpts, ratelimit.WithMessageRateLimiter(unicastMessageRateLimiter))

		// avoid connection gating and pruning during dry run
		if !fnb.BaseConfig.UnicastRateLimitersConfig.DryRun {
			f := rateLimiterPeerFilter(unicastMessageRateLimiter)
			// add IsRateLimited peerFilters to conn gater intercept secure peer and peer manager filters list
			// don't allow rate limited peers to establishing incoming connections
			connGaterInterceptSecureFilters = append(connGaterInterceptSecureFilters, f)
			// don't create outbound connections to rate limited peers
			peerManagerFilters = append(peerManagerFilters, f)
		}
	}

	// override noop unicast bandwidth rate limiter
	if fnb.BaseConfig.UnicastRateLimitersConfig.BandwidthRateLimit > 0 && fnb.BaseConfig.UnicastRateLimitersConfig.BandwidthBurstLimit > 0 {
		unicastBandwidthRateLimiter := ratelimit.NewBandWidthRateLimiter(
			rate.Limit(fnb.BaseConfig.UnicastRateLimitersConfig.BandwidthRateLimit),
			fnb.BaseConfig.UnicastRateLimitersConfig.BandwidthBurstLimit,
			fnb.BaseConfig.UnicastRateLimitersConfig.LockoutDuration,
		)
		unicastRateLimiterOpts = append(unicastRateLimiterOpts, ratelimit.WithBandwidthRateLimiter(unicastBandwidthRateLimiter))

		// avoid connection gating and pruning during dry run
		if !fnb.BaseConfig.UnicastRateLimitersConfig.DryRun {
			f := rateLimiterPeerFilter(unicastBandwidthRateLimiter)
			// add IsRateLimited peerFilters to conn gater intercept secure peer and peer manager filters list
			connGaterInterceptSecureFilters = append(connGaterInterceptSecureFilters, f)
			peerManagerFilters = append(peerManagerFilters, f)
		}
	}

	// setup unicast rate limiters
	unicastRateLimiters := ratelimit.NewRateLimiters(unicastRateLimiterOpts...)

	uniCfg := &p2pbuilder.UnicastConfig{
		StreamRetryInterval:    fnb.UnicastCreateStreamRetryDelay,
		RateLimiterDistributor: fnb.UnicastRateLimiterDistributor,
	}

	connGaterCfg := &p2pbuilder.ConnectionGaterConfig{
		InterceptPeerDialFilters: connGaterPeerDialFilters,
		InterceptSecuredFilters:  connGaterInterceptSecureFilters,
	}

	peerManagerCfg := &p2pbuilder.PeerManagerConfig{
		ConnectionPruning: fnb.NetworkConnectionPruning,
		UpdateInterval:    fnb.PeerUpdateInterval,
	}

	fnb.Component(LibP2PNodeComponent, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		myAddr := fnb.NodeConfig.Me.Address()
		if fnb.BaseConfig.BindAddr != NotSet {
			myAddr = fnb.BaseConfig.BindAddr
		}

		fnb.GossipSubInspectorNotifDistributor = BuildGossipsubRPCValidationInspectorNotificationDisseminator(fnb.GossipSubRPCInspectorsConfig.GossipSubRPCInspectorNotificationCacheSize, fnb.MetricsRegisterer, fnb.Logger, fnb.MetricsEnabled)

		rpcInspectorBuilder := inspector.NewGossipSubInspectorBuilder(fnb.Logger, fnb.SporkID, fnb.GossipSubRPCInspectorsConfig, fnb.GossipSubInspectorNotifDistributor)
		rpcInspectors, err := rpcInspectorBuilder.
			SetPublicNetwork(p2p.PublicNetworkDisabled).
			SetMetrics(fnb.Metrics.Network, fnb.MetricsRegisterer).
			SetMetricsEnabled(fnb.MetricsEnabled).Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create gossipsub rpc inspectors: %w", err)
		}

		// set rpc inspectors on gossipsub config
		fnb.GossipSubConfig.RPCInspectors = rpcInspectors

		libP2PNodeFactory := p2pbuilder.DefaultLibP2PNodeFactory(
			fnb.Logger,
			myAddr,
			fnb.NetworkKey,
			fnb.SporkID,
			fnb.IdentityProvider,
			fnb.Metrics.Network,
			fnb.Resolver,
			fnb.BaseConfig.NodeRole,
			connGaterCfg,
			peerManagerCfg,
			// run peer manager with the specified interval and let it also prune connections
			fnb.GossipSubConfig,
			fnb.LibP2PResourceManagerConfig,
			uniCfg,
		)

		libp2pNode, err := libP2PNodeFactory()
		if err != nil {
			return nil, fmt.Errorf("failed to create libp2p node: %w", err)
		}
		fnb.LibP2PNode = libp2pNode

		return libp2pNode, nil
	})
	fnb.Component("gossipsub inspector notification distributor", func(node *NodeConfig) (module.ReadyDoneAware, error) {
		// distributor is returned as a component to be started and stopped.
		if fnb.GossipSubInspectorNotifDistributor == nil {
			return nil, fmt.Errorf("gossipsub inspector notification distributor has not been set")
		}
		return fnb.GossipSubInspectorNotifDistributor, nil
	})
	fnb.Component(NetworkComponent, func(node *NodeConfig) (module.ReadyDoneAware, error) {
		cf := conduit.NewDefaultConduitFactory(fnb.Logger, fnb.Metrics.Network)
		fnb.Logger.Info().Hex("node_id", logging.ID(fnb.NodeID)).Msg("default conduit factory initiated")
		return fnb.InitFlowNetworkWithConduitFactory(node, cf, unicastRateLimiters, peerManagerFilters)
	})

	fnb.Module("middleware dependency", func(node *NodeConfig) error {
		fnb.middlewareDependable = module.NewProxiedReadyDoneAware()
		fnb.PeerManagerDependencies.Add(fnb.middlewareDependable)
		return nil
	})

	// peer manager won't be created until all PeerManagerDependencies are ready.
	fnb.DependableComponent("peer manager", func(node *NodeConfig) (module.ReadyDoneAware, error) {
		return fnb.LibP2PNode.PeerManagerComponent(), nil
	}, fnb.PeerManagerDependencies)
}

func (fnb *FlowNodeBuilder) InitFlowNetworkWithConduitFactory(
	node *NodeConfig,
	cf network.ConduitFactory,
	unicastRateLimiters *ratelimit.RateLimiters,
	peerManagerFilters []p2p.PeerFilter) (network.Network, error) {
	var mwOpts []middleware.MiddlewareOption
	if len(fnb.MsgValidators) > 0 {
		mwOpts = append(mwOpts, middleware.WithMessageValidators(fnb.MsgValidators...))
	}

	// by default if no rate limiter configuration was provided in the CLI args the default
	// noop rate limiter will be used.
	mwOpts = append(mwOpts, middleware.WithUnicastRateLimiters(unicastRateLimiters))

	mwOpts = append(mwOpts,
		middleware.WithPreferredUnicastProtocols(protocols.ToProtocolNames(fnb.PreferredUnicastProtocols)),
	)

	// peerManagerFilters are used by the peerManager via the middleware to filter peers from the topology.
	if len(peerManagerFilters) > 0 {
		mwOpts = append(mwOpts, middleware.WithPeerManagerFilters(peerManagerFilters))
	}

	slashingViolationsConsumer := slashing.NewSlashingViolationsConsumer(fnb.Logger, fnb.Metrics.Network)
	mw := middleware.NewMiddleware(
		fnb.Logger,
		fnb.LibP2PNode,
		fnb.Me.NodeID(),
		fnb.Metrics.Bitswap,
		fnb.SporkID,
		fnb.BaseConfig.UnicastMessageTimeout,
		fnb.IDTranslator,
		fnb.CodecFactory(),
		slashingViolationsConsumer,
		mwOpts...)
	fnb.NodeDisallowListDistributor.AddConsumer(mw)
	fnb.Middleware = mw

	subscriptionManager := subscription.NewChannelSubscriptionManager(fnb.Middleware)
	var heroCacheCollector module.HeroCacheMetrics = metrics.NewNoopCollector()
	if fnb.HeroCacheMetricsEnable {
		heroCacheCollector = metrics.NetworkReceiveCacheMetricsFactory(fnb.MetricsRegisterer)
	}

	receiveCache := netcache.NewHeroReceiveCache(fnb.NetworkReceivedMessageCacheSize,
		fnb.Logger,
		heroCacheCollector)

	err := node.Metrics.Mempool.Register(metrics.ResourceNetworkingReceiveCache, receiveCache.Size)
	if err != nil {
		return nil, fmt.Errorf("could not register networking receive cache metric: %w", err)
	}

	// creates network instance
	net, err := p2p.NewNetwork(&p2p.NetworkParameters{
		Logger:              fnb.Logger,
		Codec:               fnb.CodecFactory(),
		Me:                  fnb.Me,
		MiddlewareFactory:   func() (network.Middleware, error) { return fnb.Middleware, nil },
		Topology:            topology.NewFullyConnectedTopology(),
		SubscriptionManager: subscriptionManager,
		Metrics:             fnb.Metrics.Network,
		IdentityProvider:    fnb.IdentityProvider,
		ReceiveCache:        receiveCache,
		Options:             []p2p.NetworkOptFunction{p2p.WithConduitFactory(cf)},
	})
	if err != nil {
		return nil, fmt.Errorf("could not initialize network: %w", err)
	}

	fnb.Network = net

	// register middleware's ReadyDoneAware interface so other components can depend on it for startup
	if fnb.middlewareDependable != nil {
		fnb.middlewareDependable.Init(fnb.Middleware)
	}

	idEvents := gadgets.NewIdentityDeltas(fnb.Middleware.UpdateNodeAddresses)
	fnb.ProtocolEvents.AddConsumer(idEvents)

	return net, nil
}

func (fnb *FlowNodeBuilder) EnqueueMetricsServerInit() {
	fnb.Component("metrics server", func(node *NodeConfig) (module.ReadyDoneAware, error) {
		server := metrics.NewServer(fnb.Logger, fnb.BaseConfig.metricsPort)
		return server, nil
	})
}

func (fnb *FlowNodeBuilder) EnqueueAdminServerInit() error {
	if fnb.AdminAddr == NotSet {
		return nil
	}

	if (fnb.AdminCert != NotSet || fnb.AdminKey != NotSet || fnb.AdminClientCAs != NotSet) &&
		!(fnb.AdminCert != NotSet && fnb.AdminKey != NotSet && fnb.AdminClientCAs != NotSet) {
		return fmt.Errorf("admin cert / key and client certs must all be provided to enable mutual TLS")
	}

	// create the updatable config manager
	fnb.RegisterDefaultAdminCommands()
	fnb.Component("admin server", func(node *NodeConfig) (module.ReadyDoneAware, error) {
		// set up all admin commands
		for commandName, commandFunc := range fnb.adminCommands {
			command := commandFunc(fnb.NodeConfig)
			fnb.adminCommandBootstrapper.RegisterHandler(commandName, command.Handler)
			fnb.adminCommandBootstrapper.RegisterValidator(commandName, command.Validator)
		}

		opts := []admin.CommandRunnerOption{
			admin.WithMaxMsgSize(int(fnb.AdminMaxMsgSize)),
		}

		if node.AdminCert != NotSet {
			serverCert, err := tls.LoadX509KeyPair(node.AdminCert, node.AdminKey)
			if err != nil {
				return nil, err
			}
			clientCAs, err := os.ReadFile(node.AdminClientCAs)
			if err != nil {
				return nil, err
			}
			certPool := x509.NewCertPool()
			certPool.AppendCertsFromPEM(clientCAs)
			config := &tls.Config{
				MinVersion:   tls.VersionTLS13,
				Certificates: []tls.Certificate{serverCert},
				ClientAuth:   tls.RequireAndVerifyClientCert,
				ClientCAs:    certPool,
			}

			opts = append(opts, admin.WithTLS(config))
		}

		runner := fnb.adminCommandBootstrapper.Bootstrap(fnb.Logger, fnb.AdminAddr, opts...)

		return runner, nil
	})

	return nil
}

func (fnb *FlowNodeBuilder) RegisterBadgerMetrics() error {
	return metrics.RegisterBadgerMetrics()
}

func (fnb *FlowNodeBuilder) EnqueueTracer() {
	fnb.Component("tracer", func(node *NodeConfig) (module.ReadyDoneAware, error) {
		return fnb.Tracer, nil
	})
}

func (fnb *FlowNodeBuilder) ParseAndPrintFlags() error {
	// parse configuration parameters
	pflag.Parse()

	// print all flags
	log := fnb.Logger.Info()

	pflag.VisitAll(func(flag *pflag.Flag) {
		log = log.Str(flag.Name, flag.Value.String())
	})

	log.Msg("flags loaded")

	return fnb.extraFlagsValidation()
}

func (fnb *FlowNodeBuilder) ValidateRootSnapshot(f func(protocol.Snapshot) error) NodeBuilder {
	fnb.extraRootSnapshotCheck = f
	return fnb
}

func (fnb *FlowNodeBuilder) ValidateFlags(f func() error) NodeBuilder {
	fnb.extraFlagCheck = f
	return fnb
}

func (fnb *FlowNodeBuilder) PrintBuildVersionDetails() {
	fnb.Logger.Info().Str("version", build.Semver()).Str("commit", build.Commit()).Msg("build details")
}

func (fnb *FlowNodeBuilder) initNodeInfo() error {
	if fnb.BaseConfig.nodeIDHex == NotSet {
		return fmt.Errorf("cannot start without node ID")
	}

	nodeID, err := flow.HexStringToIdentifier(fnb.BaseConfig.nodeIDHex)
	if err != nil {
		return fmt.Errorf("could not parse node ID from string (id: %v): %w", fnb.BaseConfig.nodeIDHex, err)
	}

	info, err := LoadPrivateNodeInfo(fnb.BaseConfig.BootstrapDir, nodeID)
	if err != nil {
		return fmt.Errorf("failed to load private node info: %w", err)
	}

	fnb.NodeID = nodeID
	fnb.NetworkKey = info.NetworkPrivKey.PrivateKey
	fnb.StakingKey = info.StakingPrivKey.PrivateKey

	return nil
}

func (fnb *FlowNodeBuilder) initLogger() error {
	// configure logger with standard level, node ID and UTC timestamp
	zerolog.TimeFieldFormat = time.RFC3339Nano
	zerolog.TimestampFunc = func() time.Time { return time.Now().UTC() }

	// Drop all log events that exceed this rate limit
	throttledSampler := logging.BurstSampler(fnb.BaseConfig.debugLogLimit, time.Second)

	log := fnb.Logger.With().
		Timestamp().
		Str("node_role", fnb.BaseConfig.NodeRole).
		Str("node_id", fnb.NodeID.String()).
		Logger().
		Sample(zerolog.LevelSampler{
			TraceSampler: throttledSampler,
			DebugSampler: throttledSampler,
		})

	log.Info().Msgf("flow %s node starting up", fnb.BaseConfig.NodeRole)

	// parse config log level and apply to logger
	lvl, err := zerolog.ParseLevel(strings.ToLower(fnb.BaseConfig.level))
	if err != nil {
		return fmt.Errorf("invalid log level: %w", err)
	}

	// Minimum log level is set to trace, then overridden by SetGlobalLevel.
	// this allows admin commands to modify the level to any value during runtime
	log = log.Level(zerolog.TraceLevel)
	zerolog.SetGlobalLevel(lvl)

	fnb.Logger = log

	return nil
}

func (fnb *FlowNodeBuilder) initMetrics() error {

	fnb.Tracer = trace.NewNoopTracer()
	if fnb.BaseConfig.tracerEnabled {
		nodeIdHex := fnb.NodeID.String()
		if len(nodeIdHex) > 8 {
			nodeIdHex = nodeIdHex[:8]
		}

		serviceName := fnb.BaseConfig.NodeRole + "-" + nodeIdHex
		tracer, err := trace.NewTracer(
			fnb.Logger,
			serviceName,
			fnb.RootChainID.String(),
			fnb.tracerSensitivity,
		)
		if err != nil {
			return fmt.Errorf("could not initialize tracer: %w", err)
		}

		fnb.Logger.Info().Msg("Tracer Started")
		fnb.Tracer = tracer
	}

	fnb.Metrics = Metrics{
		Network:        metrics.NewNoopCollector(),
		Engine:         metrics.NewNoopCollector(),
		Compliance:     metrics.NewNoopCollector(),
		Cache:          metrics.NewNoopCollector(),
		Mempool:        metrics.NewNoopCollector(),
		CleanCollector: metrics.NewNoopCollector(),
		Bitswap:        metrics.NewNoopCollector(),
	}
	if fnb.BaseConfig.MetricsEnabled {
		fnb.MetricsRegisterer = prometheus.DefaultRegisterer

		mempools := metrics.NewMempoolCollector(5 * time.Second)

		fnb.Metrics = Metrics{
			Network:    metrics.NewNetworkCollector(fnb.Logger),
			Engine:     metrics.NewEngineCollector(),
			Compliance: metrics.NewComplianceCollector(),
			// CacheControl metrics has been causing memory abuse, disable for now
			// Cache:          metrics.NewCacheCollector(fnb.RootChainID),
			Cache:          metrics.NewNoopCollector(),
			CleanCollector: metrics.NewCleanerCollector(),
			Mempool:        mempools,
			Bitswap:        metrics.NewBitswapCollector(),
		}

		// registers mempools as a Component so that its Ready method is invoked upon startup
		fnb.Component("mempools metrics", func(node *NodeConfig) (module.ReadyDoneAware, error) {
			return mempools, nil
		})

		// metrics enabled, report node info metrics as post init event
		fnb.PostInit(func(nodeConfig *NodeConfig) error {
			nodeInfoMetrics := metrics.NewNodeInfoCollector()
			protocolVersion, err := fnb.RootSnapshot.Params().ProtocolVersion()
			if err != nil {
				return fmt.Errorf("could not query root snapshoot protocol version: %w", err)
			}
			nodeInfoMetrics.NodeInfo(build.Semver(), build.Commit(), nodeConfig.SporkID.String(), protocolVersion)
			return nil
		})
	}
	return nil
}

func (fnb *FlowNodeBuilder) createGCEProfileUploader(client *gcemd.Client, opts ...option.ClientOption) (profiler.Uploader, error) {
	projectID, err := client.ProjectID()
	if err != nil {
		return &profiler.NoopUploader{}, fmt.Errorf("failed to get project ID: %w", err)
	}

	instance, err := client.InstanceID()
	if err != nil {
		return &profiler.NoopUploader{}, fmt.Errorf("failed to get instance ID: %w", err)
	}

	chainID := fnb.RootChainID.String()
	if chainID == "" {
		fnb.Logger.Warn().Msg("RootChainID is not set, using default value")
		chainID = "unknown"
	}

	params := profiler.Params{
		ProjectID: projectID,
		ChainID:   chainID,
		Role:      fnb.NodeConfig.NodeRole,
		Version:   build.Semver(),
		Commit:    build.Commit(),
		Instance:  instance,
	}
	fnb.Logger.Info().Msgf("creating pprof profile uploader with params: %+v", params)

	return profiler.NewUploader(fnb.Logger, params, opts...)
}

func (fnb *FlowNodeBuilder) createProfileUploader() (profiler.Uploader, error) {
	switch {
	case fnb.BaseConfig.profilerConfig.UploaderEnabled && gcemd.OnGCE():
		return fnb.createGCEProfileUploader(gcemd.NewClient(nil))
	default:
		fnb.Logger.Info().Msg("not running on GCE, setting pprof uploader to noop")
		return &profiler.NoopUploader{}, nil
	}
}

func (fnb *FlowNodeBuilder) initProfiler() error {
	uploader, err := fnb.createProfileUploader()
	if err != nil {
		fnb.Logger.Warn().Err(err).Msg("failed to create pprof uploader, falling back to noop")
		uploader = &profiler.NoopUploader{}
	}

	profiler, err := profiler.New(fnb.Logger, uploader, fnb.BaseConfig.profilerConfig)
	if err != nil {
		return fmt.Errorf("could not initialize profiler: %w", err)
	}

	// register the enabled state of the profiler for dynamic configuring
	err = fnb.ConfigManager.RegisterBoolConfig("profiler-enabled", profiler.Enabled, profiler.SetEnabled)
	if err != nil {
		return fmt.Errorf("could not register profiler-enabled config: %w", err)
	}

	err = fnb.ConfigManager.RegisterDurationConfig(
		"profiler-trigger",
		func() time.Duration { return fnb.BaseConfig.profilerConfig.Duration },
		func(d time.Duration) error { return profiler.TriggerRun(d) },
	)
	if err != nil {
		return fmt.Errorf("could not register profiler-trigger config: %w", err)
	}

	err = fnb.ConfigManager.RegisterUintConfig(
		"profiler-set-mem-profile-rate",
		func() uint { return uint(runtime.MemProfileRate) },
		func(r uint) error { runtime.MemProfileRate = int(r); return nil },
	)
	if err != nil {
		return fmt.Errorf("could not register profiler-set-mem-profile-rate setting: %w", err)
	}

	// There is no way to get the current block profile rate so we keep track of it ourselves.
	currentRate := new(uint)
	err = fnb.ConfigManager.RegisterUintConfig(
		"profiler-set-block-profile-rate",
		func() uint { return *currentRate },
		func(r uint) error { currentRate = &r; runtime.SetBlockProfileRate(int(r)); return nil },
	)
	if err != nil {
		return fmt.Errorf("could not register profiler-set-block-profile-rate setting: %w", err)
	}

	err = fnb.ConfigManager.RegisterUintConfig(
		"profiler-set-mutex-profile-fraction",
		func() uint { return uint(runtime.SetMutexProfileFraction(-1)) },
		func(r uint) error { _ = runtime.SetMutexProfileFraction(int(r)); return nil },
	)
	if err != nil {
		return fmt.Errorf("could not register profiler-set-mutex-profile-fraction setting: %w", err)
	}

	// registering as a DependableComponent with no dependencies so that it's started immediately on startup
	// without being blocked by other component's Ready()
	fnb.DependableComponent("profiler", func(node *NodeConfig) (module.ReadyDoneAware, error) {
		return profiler, nil
	}, NewDependencyList())

	return nil
}

func (fnb *FlowNodeBuilder) initDB() error {

	// if a db has been passed in, use that instead of creating one
	if fnb.BaseConfig.db != nil {
		fnb.DB = fnb.BaseConfig.db
		return nil
	}

	// Pre-create DB path (Badger creates only one-level dirs)
	err := os.MkdirAll(fnb.BaseConfig.datadir, 0700)
	if err != nil {
		return fmt.Errorf("could not create datadir (path: %s): %w", fnb.BaseConfig.datadir, err)
	}

	log := sutil.NewLogger(fnb.Logger)

	// we initialize the database with options that allow us to keep the maximum
	// item size in the trie itself (up to 1MB) and where we keep all level zero
	// tables in-memory as well; this slows down compaction and increases memory
	// usage, but it improves overall performance and disk i/o
	opts := badger.
		DefaultOptions(fnb.BaseConfig.datadir).
		WithKeepL0InMemory(true).
		WithLogger(log).

		// the ValueLogFileSize option specifies how big the value of a
		// key-value pair is allowed to be saved into badger.
		// exceeding this limit, will fail with an error like this:
		// could not store data: Value with size <xxxx> exceeded 1073741824 limit
		// Maximum value size is 10G, needed by execution node
		// TODO: finding a better max value for each node type
		WithValueLogFileSize(128 << 23).
		WithValueLogMaxEntries(100000) // Default is 1000000

	publicDB, err := bstorage.InitPublic(opts)
	if err != nil {
		return fmt.Errorf("could not open public db: %w", err)
	}
	fnb.DB = publicDB

	fnb.ShutdownFunc(func() error {
		if err := fnb.DB.Close(); err != nil {
			return fmt.Errorf("error closing protocol database: %w", err)
		}
		return nil
	})

	fnb.Component("badger log cleaner", func(node *NodeConfig) (module.ReadyDoneAware, error) {
		return bstorage.NewCleaner(node.Logger, node.DB, node.Metrics.CleanCollector, flow.DefaultValueLogGCWaitDuration), nil
	})

	return nil
}

func (fnb *FlowNodeBuilder) initSecretsDB() error {

	// if the secrets DB is disabled (only applicable for Consensus Follower,
	// which makes use of this same logic), skip this initialization
	if !fnb.BaseConfig.secretsDBEnabled {
		return nil
	}

	if fnb.BaseConfig.secretsdir == NotSet {
		return fmt.Errorf("missing required flag '--secretsdir'")
	}

	err := os.MkdirAll(fnb.BaseConfig.secretsdir, 0700)
	if err != nil {
		return fmt.Errorf("could not create secrets db dir (path: %s): %w", fnb.BaseConfig.secretsdir, err)
	}

	log := sutil.NewLogger(fnb.Logger)

	opts := badger.DefaultOptions(fnb.BaseConfig.secretsdir).WithLogger(log)

	// NOTE: SN nodes need to explicitly set --insecure-secrets-db to true in order to
	// disable secrets database encryption
	if fnb.NodeRole == flow.RoleConsensus.String() && fnb.InsecureSecretsDB {
		fnb.Logger.Warn().Msg("starting with secrets database encryption disabled")
	} else {
		encryptionKey, err := loadSecretsEncryptionKey(fnb.BootstrapDir, fnb.NodeID)
		if errors.Is(err, os.ErrNotExist) {
			if fnb.NodeRole == flow.RoleConsensus.String() {
				// missing key is a fatal error for SN nodes
				return fmt.Errorf("secrets db encryption key not found: %w", err)
			}
			fnb.Logger.Warn().Msg("starting with secrets database encryption disabled")
		} else if err != nil {
			return fmt.Errorf("failed to read secrets db encryption key: %w", err)
		} else {
			opts = opts.WithEncryptionKey(encryptionKey)
		}
	}

	secretsDB, err := bstorage.InitSecret(opts)
	if err != nil {
		return fmt.Errorf("could not open secrets db: %w", err)
	}
	fnb.SecretsDB = secretsDB

	fnb.ShutdownFunc(func() error {
		if err := fnb.SecretsDB.Close(); err != nil {
			return fmt.Errorf("error closing secrets database: %w", err)
		}
		return nil
	})

	return nil
}

func (fnb *FlowNodeBuilder) initStorage() error {

	// in order to void long iterations with big keys when initializing with an
	// already populated database, we bootstrap the initial maximum key size
	// upon starting
	err := operation.RetryOnConflict(fnb.DB.Update, func(tx *badger.Txn) error {
		return operation.InitMax(tx)
	})
	if err != nil {
		return fmt.Errorf("could not initialize max tracker: %w", err)
	}

	headers := bstorage.NewHeaders(fnb.Metrics.Cache, fnb.DB)
	guarantees := bstorage.NewGuarantees(fnb.Metrics.Cache, fnb.DB, fnb.BaseConfig.guaranteesCacheSize)
	seals := bstorage.NewSeals(fnb.Metrics.Cache, fnb.DB)
	results := bstorage.NewExecutionResults(fnb.Metrics.Cache, fnb.DB)
	receipts := bstorage.NewExecutionReceipts(fnb.Metrics.Cache, fnb.DB, results, fnb.BaseConfig.receiptsCacheSize)
	index := bstorage.NewIndex(fnb.Metrics.Cache, fnb.DB)
	payloads := bstorage.NewPayloads(fnb.DB, index, guarantees, seals, receipts, results)
	blocks := bstorage.NewBlocks(fnb.DB, headers, payloads)
	qcs := bstorage.NewQuorumCertificates(fnb.Metrics.Cache, fnb.DB, bstorage.DefaultCacheSize)
	transactions := bstorage.NewTransactions(fnb.Metrics.Cache, fnb.DB)
	collections := bstorage.NewCollections(fnb.DB, transactions)
	setups := bstorage.NewEpochSetups(fnb.Metrics.Cache, fnb.DB)
	epochCommits := bstorage.NewEpochCommits(fnb.Metrics.Cache, fnb.DB)
	statuses := bstorage.NewEpochStatuses(fnb.Metrics.Cache, fnb.DB)
	commits := bstorage.NewCommits(fnb.Metrics.Cache, fnb.DB)
	versionBeacons := bstorage.NewVersionBeacons(fnb.DB)

	fnb.Storage = Storage{
		Headers:            headers,
		Guarantees:         guarantees,
		Receipts:           receipts,
		Results:            results,
		Seals:              seals,
		Index:              index,
		Payloads:           payloads,
		Blocks:             blocks,
		QuorumCertificates: qcs,
		Transactions:       transactions,
		Collections:        collections,
		Setups:             setups,
		EpochCommits:       epochCommits,
		VersionBeacons:     versionBeacons,
		Statuses:           statuses,
		Commits:            commits,
	}

	return nil
}

func (fnb *FlowNodeBuilder) InitIDProviders() {
	fnb.Component("disallow list notification distributor", func(node *NodeConfig) (module.ReadyDoneAware, error) {
		// distributor is returned as a component to be started and stopped.
		if fnb.NodeDisallowListDistributor == nil {
			return nil, fmt.Errorf("disallow list notification distributor has not been set")
		}
		return fnb.NodeDisallowListDistributor, nil
	})
	fnb.Module("id providers", func(node *NodeConfig) error {
		idCache, err := cache.NewProtocolStateIDCache(node.Logger, node.State, node.ProtocolEvents)
		if err != nil {
			return fmt.Errorf("could not initialize ProtocolStateIDCache: %w", err)
		}
		node.IDTranslator = idCache

		fnb.NodeDisallowListDistributor = BuildDisallowListNotificationDisseminator(fnb.DisallowListNotificationCacheSize, fnb.MetricsRegisterer, fnb.Logger, fnb.MetricsEnabled)

		// The following wrapper allows to disallow-list byzantine nodes via an admin command:
		// the wrapper overrides the 'Ejected' flag of disallow-listed nodes to true
		disallowListWrapper, err := cache.NewNodeBlocklistWrapper(idCache, node.DB, fnb.NodeDisallowListDistributor)
		if err != nil {
			return fmt.Errorf("could not initialize NodeBlockListWrapper: %w", err)
		}
		node.IdentityProvider = disallowListWrapper

		// register the disallow list wrapper for dynamic configuration via admin command
		err = node.ConfigManager.RegisterIdentifierListConfig("network-id-provider-blocklist",
			disallowListWrapper.GetBlocklist, disallowListWrapper.Update)
		if err != nil {
			return fmt.Errorf("failed to register blocklist with config manager: %w", err)
		}

		node.SyncEngineIdentifierProvider = id.NewIdentityFilterIdentifierProvider(
			filter.And(
				filter.HasRole(flow.RoleConsensus),
				filter.Not(filter.HasNodeID(node.Me.NodeID())),
				p2p.NotEjectedFilter,
			),
			node.IdentityProvider,
		)
		return nil
	})
}

func (fnb *FlowNodeBuilder) initState() error {
	fnb.ProtocolEvents = events.NewDistributor()

	isBootStrapped, err := badgerState.IsBootstrapped(fnb.DB)
	if err != nil {
		return fmt.Errorf("failed to determine whether database contains bootstrapped state: %w", err)
	}

	if isBootStrapped {
		fnb.Logger.Info().Msg("opening already bootstrapped protocol state")
		state, err := badgerState.OpenState(
			fnb.Metrics.Compliance,
			fnb.DB,
			fnb.Storage.Headers,
			fnb.Storage.Seals,
			fnb.Storage.Results,
			fnb.Storage.Blocks,
			fnb.Storage.QuorumCertificates,
			fnb.Storage.Setups,
			fnb.Storage.EpochCommits,
			fnb.Storage.Statuses,
			fnb.Storage.VersionBeacons,
		)
		if err != nil {
			return fmt.Errorf("could not open protocol state: %w", err)
		}
		fnb.State = state

		// set root snapshot field
		rootBlock, err := state.Params().Root()
		if err != nil {
			return fmt.Errorf("could not get root block from protocol state: %w", err)
		}

		rootSnapshot := state.AtBlockID(rootBlock.ID())
		if err := fnb.setRootSnapshot(rootSnapshot); err != nil {
			return err
		}
	} else {
		// Bootstrap!
		fnb.Logger.Info().Msg("bootstrapping empty protocol state")

		// if no root snapshot is configured, attempt to load the file from disk
		var rootSnapshot = fnb.RootSnapshot
		if rootSnapshot == nil {
			fnb.Logger.Info().Msgf("loading root protocol state snapshot from disk")
			rootSnapshot, err = loadRootProtocolSnapshot(fnb.BaseConfig.BootstrapDir)
			if err != nil {
				return fmt.Errorf("failed to read protocol snapshot from disk: %w", err)
			}
		}
		// set root snapshot fields
		if err := fnb.setRootSnapshot(rootSnapshot); err != nil {
			return err
		}

		// generate bootstrap config options as per NodeConfig
		var options []badgerState.BootstrapConfigOptions
		if fnb.SkipNwAddressBasedValidations {
			options = append(options, badgerState.SkipNetworkAddressValidation)
		}

		fnb.State, err = badgerState.Bootstrap(
			fnb.Metrics.Compliance,
			fnb.DB,
			fnb.Storage.Headers,
			fnb.Storage.Seals,
			fnb.Storage.Results,
			fnb.Storage.Blocks,
			fnb.Storage.QuorumCertificates,
			fnb.Storage.Setups,
			fnb.Storage.EpochCommits,
			fnb.Storage.Statuses,
			fnb.Storage.VersionBeacons,
			fnb.RootSnapshot,
			options...,
		)
		if err != nil {
			return fmt.Errorf("could not bootstrap protocol state: %w", err)
		}

		fnb.Logger.Info().
			Hex("root_result_id", logging.Entity(fnb.RootResult)).
			Hex("root_state_commitment", fnb.RootSeal.FinalState[:]).
			Hex("root_block_id", logging.Entity(fnb.RootBlock)).
			Uint64("root_block_height", fnb.RootBlock.Header.Height).
			Msg("protocol state bootstrapped")
	}

	// initialize local if it hasn't been initialized yet
	if fnb.Me == nil {
		if err := fnb.initLocal(); err != nil {
			return err
		}
	}

	lastFinalized, err := fnb.State.Final().Head()
	if err != nil {
		return fmt.Errorf("could not get last finalized block header: %w", err)
	}

	fnb.Logger.Info().
		Hex("root_block_id", logging.Entity(fnb.RootBlock)).
		Uint64("root_block_height", fnb.RootBlock.Header.Height).
		Hex("finalized_block_id", logging.Entity(lastFinalized)).
		Uint64("finalized_block_height", lastFinalized.Height).
		Msg("successfully opened protocol state")

	return nil
}

// setRootSnapshot sets the root snapshot field and all related fields in the NodeConfig.
func (fnb *FlowNodeBuilder) setRootSnapshot(rootSnapshot protocol.Snapshot) error {
	var err error

	// validate the root snapshot QCs
	err = badgerState.IsValidRootSnapshotQCs(rootSnapshot)
	if err != nil {
		return fmt.Errorf("failed to validate root snapshot QCs: %w", err)
	}

	// perform extra checks requested by specific node types
	if fnb.extraRootSnapshotCheck != nil {
		err = fnb.extraRootSnapshotCheck(rootSnapshot)
		if err != nil {
			return fmt.Errorf("failed to perform extra checks on root snapshot: %w", err)
		}
	}

	fnb.RootSnapshot = rootSnapshot
	// cache properties of the root snapshot, for convenience
	fnb.RootResult, fnb.RootSeal, err = fnb.RootSnapshot.SealedResult()
	if err != nil {
		return fmt.Errorf("failed to read root sealed result: %w", err)
	}

	sealingSegment, err := fnb.RootSnapshot.SealingSegment()
	if err != nil {
		return fmt.Errorf("failed to read root sealing segment: %w", err)
	}

	fnb.RootBlock = sealingSegment.Highest()
	fnb.RootQC, err = fnb.RootSnapshot.QuorumCertificate()
	if err != nil {
		return fmt.Errorf("failed to read root QC: %w", err)
	}

	fnb.RootChainID = fnb.RootBlock.Header.ChainID
	fnb.SporkID, err = fnb.RootSnapshot.Params().SporkID()
	if err != nil {
		return fmt.Errorf("failed to read spork ID: %w", err)
	}

	return nil
}

func (fnb *FlowNodeBuilder) initLocal() error {
	// Verify that my ID (as given in the configuration) is known to the network
	// (i.e. protocol state). There are two cases that will cause the following error:
	// 1) used the wrong node id, which is not part of the identity list of the finalized state
	// 2) the node id is a new one for a new spork, but the bootstrap data has not been updated.
	myID, err := flow.HexStringToIdentifier(fnb.BaseConfig.nodeIDHex)
	if err != nil {
		return fmt.Errorf("could not parse node identifier: %w", err)
	}

	self, err := fnb.State.Final().Identity(myID)
	if err != nil {
		return fmt.Errorf("node identity not found in the identity list of the finalized state (id: %v): %w", myID, err)
	}

	// Verify that my role (as given in the configuration) is consistent with the protocol state.
	// We enforce this strictly for MainNet. For other networks (e.g. TestNet or BenchNet), we
	// are lenient, to allow ghost node to run as any role.
	if self.Role.String() != fnb.BaseConfig.NodeRole {
		rootBlockHeader, err := fnb.State.Params().Root()
		if err != nil {
			return fmt.Errorf("could not get root block from protocol state: %w", err)
		}

		if rootBlockHeader.ChainID == flow.Mainnet {
			return fmt.Errorf("running as incorrect role, expected: %v, actual: %v, exiting",
				self.Role.String(),
				fnb.BaseConfig.NodeRole,
			)
		}

		fnb.Logger.Warn().Msgf("running as incorrect role, expected: %v, actual: %v, continuing",
			self.Role.String(),
			fnb.BaseConfig.NodeRole)
	}

	// ensure that the configured staking/network keys are consistent with the protocol state
	if !self.NetworkPubKey.Equals(fnb.NetworkKey.PublicKey()) {
		return fmt.Errorf("configured networking key does not match protocol state")
	}
	if !self.StakingPubKey.Equals(fnb.StakingKey.PublicKey()) {
		return fmt.Errorf("configured staking key does not match protocol state")
	}

	fnb.Me, err = local.New(self, fnb.StakingKey)
	if err != nil {
		return fmt.Errorf("could not initialize local: %w", err)
	}

	return nil
}

func (fnb *FlowNodeBuilder) initFvmOptions() {
	blockFinder := environment.NewBlockFinder(fnb.Storage.Headers)
	vmOpts := []fvm.Option{
		fvm.WithChain(fnb.RootChainID.Chain()),
		fvm.WithBlocks(blockFinder),
		fvm.WithAccountStorageLimit(true),
	}
	if fnb.RootChainID == flow.Testnet || fnb.RootChainID == flow.Sandboxnet || fnb.RootChainID == flow.Mainnet {
		vmOpts = append(vmOpts,
			fvm.WithTransactionFeesEnabled(true),
		)
	}
	if fnb.RootChainID == flow.Testnet || fnb.RootChainID == flow.Sandboxnet || fnb.RootChainID == flow.Localnet || fnb.RootChainID == flow.Benchnet {
		vmOpts = append(vmOpts,
			fvm.WithContractDeploymentRestricted(false),
		)
	}
	fnb.FvmOptions = vmOpts
}

// handleModules initializes the given module.
func (fnb *FlowNodeBuilder) handleModule(v namedModuleFunc) error {
	err := v.fn(fnb.NodeConfig)
	if err != nil {
		return fmt.Errorf("module %s initialization failed: %w", v.name, err)
	}

	fnb.Logger.Info().Str("module", v.name).Msg("module initialization complete")
	return nil
}

// handleModules initializes all modules that have been enqueued on this node builder.
func (fnb *FlowNodeBuilder) handleModules() error {
	for _, f := range fnb.modules {
		if err := fnb.handleModule(f); err != nil {
			return err
		}
	}

	return nil
}

// handleComponents registers the component's factory method with the ComponentManager to be run
// when the node starts.
// It uses signal channels to ensure that components are started serially.
func (fnb *FlowNodeBuilder) handleComponents() error {
	// The parent/started channels are used to enforce serial startup.
	// - parent is the started channel of the previous component.
	// - when a component is ready, it closes its started channel by calling the provided callback.
	// Components wait for their parent channel to close before starting, this ensures they start
	// up serially, even though the ComponentManager will launch the goroutines in parallel.

	// The first component is always started immediately
	parent := make(chan struct{})
	close(parent)

	var err error
	asyncComponents := []namedComponentFunc{}

	// Run all components
	for _, f := range fnb.components {
		// Components with explicit dependencies are not started serially
		if f.dependencies != nil {
			asyncComponents = append(asyncComponents, f)
			continue
		}

		started := make(chan struct{})

		if f.errorHandler != nil {
			err = fnb.handleRestartableComponent(f, parent, func() { close(started) })
		} else {
			err = fnb.handleComponent(f, parent, func() { close(started) })
		}

		if err != nil {
			return fmt.Errorf("could not handle component %s: %w", f.name, err)
		}

		parent = started
	}

	// Components with explicit dependencies are run asynchronously, which means dependencies in
	// the dependency list must be initialized outside of the component factory.
	for _, f := range asyncComponents {
		fnb.Logger.Debug().Str("component", f.name).Int("dependencies", len(f.dependencies.components)).Msg("handling component asynchronously")
		err = fnb.handleComponent(f, util.AllReady(f.dependencies.components...), func() {})
		if err != nil {
			return fmt.Errorf("could not handle dependable component %s: %w", f.name, err)
		}
	}

	return nil
}

// handleComponent constructs a component using the provided ReadyDoneFactory, and registers a
// worker with the ComponentManager to be run when the node is started.
//
// The ComponentManager starts all workers in parallel. Since some components have non-idempotent
// ReadyDoneAware interfaces, we need to ensure that they are started serially. This is accomplished
// using the parentReady channel and the started closure. Components wait for the parentReady channel
// to close before starting, and then call the started callback after they are ready(). The started
// callback closes the parentReady channel of the next component, and so on.
//
// TODO: Instead of this serial startup, components should wait for their dependencies to be ready
// using their ReadyDoneAware interface. After components are updated to use the idempotent
// ReadyDoneAware interface and explicitly wait for their dependencies to be ready, we can remove
// this channel chaining.
func (fnb *FlowNodeBuilder) handleComponent(v namedComponentFunc, dependencies <-chan struct{}, started func()) error {
	// Add a closure that starts the component when the node is started, and then waits for it to exit
	// gracefully.
	// Startup for all components will happen in parallel, and components can use their dependencies'
	// ReadyDoneAware interface to wait until they are ready.
	fnb.componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		// wait for the dependencies to be ready before starting
		if err := util.WaitClosed(ctx, dependencies); err != nil {
			return
		}

		logger := fnb.Logger.With().Str("component", v.name).Logger()

		// First, build the component using the factory method.
		readyAware, err := v.fn(fnb.NodeConfig)
		if err != nil {
			ctx.Throw(fmt.Errorf("component %s initialization failed: %w", v.name, err))
		}
		logger.Info().Msg("component initialization complete")

		// if this is a Component, use the Startable interface to start the component, otherwise
		// Ready() will launch it.
		cmp, isComponent := readyAware.(component.Component)
		if isComponent {
			cmp.Start(ctx)
		}

		// Wait until the component is ready
		if err := util.WaitClosed(ctx, readyAware.Ready()); err != nil {
			// The context was cancelled. Continue to shutdown logic.
			logger.Warn().Msg("component startup aborted")

			// Non-idempotent ReadyDoneAware components trigger shutdown by calling Done(). Don't
			// do that here since it may not be safe if the component is not Ready().
			if !isComponent {
				return
			}
		} else {
			logger.Info().Msg("component startup complete")
			ready()

			// Signal to the next component that we're ready.
			started()
		}

		// Component shutdown is signaled by cancelling its context.
		<-ctx.Done()
		logger.Info().Msg("component shutdown started")

		// Finally, wait until component has finished shutting down.
		<-readyAware.Done()
		logger.Info().Msg("component shutdown complete")
	})

	return nil
}

// handleRestartableComponent constructs a component using the provided ReadyDoneFactory, and
// registers a worker with the ComponentManager to be run when the node is started.
//
// Restartable Components are components that can be restarted after successfully handling
// an irrecoverable error.
//
// Any irrecoverable errors thrown by the component will be passed to the provided error handler.
func (fnb *FlowNodeBuilder) handleRestartableComponent(v namedComponentFunc, parentReady <-chan struct{}, started func()) error {
	fnb.componentBuilder.AddWorker(func(ctx irrecoverable.SignalerContext, ready component.ReadyFunc) {
		// wait for the previous component to be ready before starting
		if err := util.WaitClosed(ctx, parentReady); err != nil {
			return
		}

		// Note: we're marking the worker routine ready before we even attempt to start the
		// component. the idea behind a restartable component is that the node should not depend
		// on it for safe operation, so the node does not need to wait for it to be ready.
		ready()

		// do not block serial startup. started can only be called once, so it cannot be called
		// from within the componentFactory
		started()

		log := fnb.Logger.With().Str("component", v.name).Logger()

		// This may be called multiple times if the component is restarted
		componentFactory := func() (component.Component, error) {
			c, err := v.fn(fnb.NodeConfig)
			if err != nil {
				return nil, err
			}
			log.Info().Msg("component initialization complete")

			go func() {
				if err := util.WaitClosed(ctx, c.Ready()); err != nil {
					log.Info().Msg("component startup aborted")
				} else {
					log.Info().Msg("component startup complete")
				}

				<-ctx.Done()
				log.Info().Msg("component shutdown started")
			}()
			return c.(component.Component), nil
		}

		err := component.RunComponent(ctx, componentFactory, v.errorHandler)
		if err != nil && !errors.Is(err, ctx.Err()) {
			ctx.Throw(fmt.Errorf("component %s encountered an unhandled irrecoverable error: %w", v.name, err))
		}

		log.Info().Msg("component shutdown complete")
	})

	return nil
}

// ExtraFlags enables binding additional flags beyond those defined in BaseConfig.
func (fnb *FlowNodeBuilder) ExtraFlags(f func(*pflag.FlagSet)) NodeBuilder {
	f(fnb.flags)
	return fnb
}

// Module enables setting up dependencies of the engine with the builder context.
func (fnb *FlowNodeBuilder) Module(name string, f BuilderFunc) NodeBuilder {
	fnb.modules = append(fnb.modules, namedModuleFunc{
		fn:   f,
		name: name,
	})
	return fnb
}

// ShutdownFunc adds a callback function that is called after all components have exited.
func (fnb *FlowNodeBuilder) ShutdownFunc(fn func() error) NodeBuilder {
	fnb.postShutdownFns = append(fnb.postShutdownFns, fn)
	return fnb
}

func (fnb *FlowNodeBuilder) AdminCommand(command string, f func(config *NodeConfig) commands.AdminCommand) NodeBuilder {
	fnb.adminCommands[command] = f
	return fnb
}

// Component adds a new component to the node that conforms to the ReadyDoneAware
// interface.
//
// The ReadyDoneFactory may return either a `Component` or `ReadyDoneAware` instance.
// In both cases, the object is started when the node is run, and the node will wait for the
// component to exit gracefully.
func (fnb *FlowNodeBuilder) Component(name string, f ReadyDoneFactory) NodeBuilder {
	fnb.components = append(fnb.components, namedComponentFunc{
		fn:   f,
		name: name,
	})
	return fnb
}

// DependableComponent adds a new component to the node that conforms to the ReadyDoneAware
// interface. The builder will wait until all of the components in the dependencies list are ready
// before constructing the component.
//
// The ReadyDoneFactory may return either a `Component` or `ReadyDoneAware` instance.
// In both cases, the object is started when the node is run, and the node will wait for the
// component to exit gracefully.
//
// IMPORTANT: Dependable components are started in parallel with no guaranteed run order, so all
// dependencies must be initialized outside of the ReadyDoneFactory, and their `Ready()` method
// MUST be idempotent.
func (fnb *FlowNodeBuilder) DependableComponent(name string, f ReadyDoneFactory, dependencies *DependencyList) NodeBuilder {
	// Note: dependencies are passed as a struct to allow updating the list after calling this method.
	// Passing a slice instead would result in out of sync metadata since slices are passed by reference
	fnb.components = append(fnb.components, namedComponentFunc{
		fn:           f,
		name:         name,
		dependencies: dependencies,
	})
	return fnb
}

// OverrideComponent adds given builder function to the components set of the node builder. If a builder function with that name
// already exists, it will be overridden.
func (fnb *FlowNodeBuilder) OverrideComponent(name string, f ReadyDoneFactory) NodeBuilder {
	for i := 0; i < len(fnb.components); i++ {
		if fnb.components[i].name == name {
			// found component with the name, override it.
			fnb.components[i] = namedComponentFunc{
				fn:   f,
				name: name,
			}

			return fnb
		}
	}

	// no component found with the same name, hence just adding it.
	return fnb.Component(name, f)
}

// RestartableComponent adds a new component to the node that conforms to the ReadyDoneAware
// interface, and calls the provided error handler when an irrecoverable error is encountered.
// Use RestartableComponent if the component is not critical to the node's safe operation and
// can/should be independently restarted when an irrecoverable error is encountered.
//
// IMPORTANT: Since a RestartableComponent can be restarted independently of the node, the node and
// other components must not rely on it for safe operation, and failures must be handled gracefully.
// As such, RestartableComponents do not block the node from becoming ready, and do not block
// subsequent components from starting serially. They do start in serial order.
//
// Note: The ReadyDoneFactory method may be called multiple times if the component is restarted.
//
// Any irrecoverable errors thrown by the component will be passed to the provided error handler.
func (fnb *FlowNodeBuilder) RestartableComponent(name string, f ReadyDoneFactory, errorHandler component.OnError) NodeBuilder {
	fnb.components = append(fnb.components, namedComponentFunc{
		fn:           f,
		name:         name,
		errorHandler: errorHandler,
	})
	return fnb
}

// OverrideModule adds given builder function to the modules set of the node builder. If a builder function with that name
// already exists, it will be overridden.
func (fnb *FlowNodeBuilder) OverrideModule(name string, f BuilderFunc) NodeBuilder {
	for i := 0; i < len(fnb.modules); i++ {
		if fnb.modules[i].name == name {
			// found module with the name, override it.
			fnb.modules[i] = namedModuleFunc{
				fn:   f,
				name: name,
			}

			return fnb
		}
	}

	// no module found with the same name, hence just adding it.
	return fnb.Module(name, f)
}

func (fnb *FlowNodeBuilder) PreInit(f BuilderFunc) NodeBuilder {
	fnb.preInitFns = append(fnb.preInitFns, f)
	return fnb
}

func (fnb *FlowNodeBuilder) PostInit(f BuilderFunc) NodeBuilder {
	fnb.postInitFns = append(fnb.postInitFns, f)
	return fnb
}

type Option func(*BaseConfig)

func WithBootstrapDir(bootstrapDir string) Option {
	return func(config *BaseConfig) {
		config.BootstrapDir = bootstrapDir
	}
}

func WithBindAddress(bindAddress string) Option {
	return func(config *BaseConfig) {
		config.BindAddr = bindAddress
	}
}

func WithDataDir(dataDir string) Option {
	return func(config *BaseConfig) {
		if config.db == nil {
			config.datadir = dataDir
		}
	}
}

func WithSecretsDBEnabled(enabled bool) Option {
	return func(config *BaseConfig) {
		config.secretsDBEnabled = enabled
	}
}

func WithMetricsEnabled(enabled bool) Option {
	return func(config *BaseConfig) {
		config.MetricsEnabled = enabled
	}
}

func WithSyncCoreConfig(syncConfig chainsync.Config) Option {
	return func(config *BaseConfig) {
		config.SyncCoreConfig = syncConfig
	}
}

func WithComplianceConfig(complianceConfig compliance.Config) Option {
	return func(config *BaseConfig) {
		config.ComplianceConfig = complianceConfig
	}
}

func WithLogLevel(level string) Option {
	return func(config *BaseConfig) {
		config.level = level
	}
}

// WithDB takes precedence over WithDataDir and datadir will be set to empty if DB is set using this option
func WithDB(db *badger.DB) Option {
	return func(config *BaseConfig) {
		config.db = db
		config.datadir = ""
	}
}

// FlowNode creates a new Flow node builder with the given name.
func FlowNode(role string, opts ...Option) *FlowNodeBuilder {
	config := DefaultBaseConfig()
	config.NodeRole = role
	for _, opt := range opts {
		opt(config)
	}

	builder := &FlowNodeBuilder{
		NodeConfig: &NodeConfig{
			BaseConfig:              *config,
			Logger:                  zerolog.New(os.Stderr),
			PeerManagerDependencies: NewDependencyList(),
			ConfigManager:           updatable_configs.NewManager(),
		},
		flags:                    pflag.CommandLine,
		adminCommandBootstrapper: admin.NewCommandRunnerBootstrapper(),
		adminCommands:            make(map[string]func(*NodeConfig) commands.AdminCommand),
		componentBuilder:         component.NewComponentManagerBuilder(),
	}
	return builder
}

func (fnb *FlowNodeBuilder) Initialize() error {
	fnb.PrintBuildVersionDetails()

	fnb.BaseFlags()

	if err := fnb.ParseAndPrintFlags(); err != nil {
		return err
	}

	// ID providers must be initialized before the network
	fnb.InitIDProviders()

	fnb.EnqueueResolver()

	fnb.EnqueueNetworkInit()

	fnb.EnqueuePingService()

	if fnb.MetricsEnabled {
		fnb.EnqueueMetricsServerInit()
		if err := fnb.RegisterBadgerMetrics(); err != nil {
			return err
		}
	}

	fnb.EnqueueTracer()

	return nil
}

func (fnb *FlowNodeBuilder) RegisterDefaultAdminCommands() {
	fnb.AdminCommand("set-log-level", func(config *NodeConfig) commands.AdminCommand {
		return &common.SetLogLevelCommand{}
	}).AdminCommand("set-golog-level", func(config *NodeConfig) commands.AdminCommand {
		return &common.SetGologLevelCommand{}
	}).AdminCommand("get-config", func(config *NodeConfig) commands.AdminCommand {
		return common.NewGetConfigCommand(config.ConfigManager)
	}).AdminCommand("set-config", func(config *NodeConfig) commands.AdminCommand {
		return common.NewSetConfigCommand(config.ConfigManager)
	}).AdminCommand("list-configs", func(config *NodeConfig) commands.AdminCommand {
		return common.NewListConfigCommand(config.ConfigManager)
	}).AdminCommand("read-blocks", func(config *NodeConfig) commands.AdminCommand {
		return storageCommands.NewReadBlocksCommand(config.State, config.Storage.Blocks)
	}).AdminCommand("read-results", func(config *NodeConfig) commands.AdminCommand {
		return storageCommands.NewReadResultsCommand(config.State, config.Storage.Results)
	}).AdminCommand("read-seals", func(config *NodeConfig) commands.AdminCommand {
		return storageCommands.NewReadSealsCommand(config.State, config.Storage.Seals, config.Storage.Index)
	}).AdminCommand("get-latest-identity", func(config *NodeConfig) commands.AdminCommand {
		return common.NewGetIdentityCommand(config.IdentityProvider)
	})
}

func (fnb *FlowNodeBuilder) Build() (Node, error) {
	// Run the prestart initialization. This includes anything that should be done before
	// starting the components.
	if err := fnb.onStart(); err != nil {
		return nil, err
	}

	return NewNode(
		fnb.componentBuilder.Build(),
		fnb.NodeConfig,
		fnb.Logger,
		fnb.postShutdown,
		fnb.handleFatal,
	), nil
}

func (fnb *FlowNodeBuilder) onStart() error {

	// seed random generator
	rand.Seed(time.Now().UnixNano())

	// init nodeinfo by reading the private bootstrap file if not already set
	if fnb.NodeID == flow.ZeroID {
		if err := fnb.initNodeInfo(); err != nil {
			return err
		}
	}

	if err := fnb.initLogger(); err != nil {
		return err
	}

	if err := fnb.initDB(); err != nil {
		return err
	}

	if err := fnb.initSecretsDB(); err != nil {
		return err
	}

	if err := fnb.initMetrics(); err != nil {
		return err
	}

	if err := fnb.initStorage(); err != nil {
		return err
	}

	for _, f := range fnb.preInitFns {
		if err := fnb.handlePreInit(f); err != nil {
			return err
		}
	}

	if err := fnb.initState(); err != nil {
		return err
	}

	if err := fnb.initProfiler(); err != nil {
		return err
	}

	fnb.initFvmOptions()

	for _, f := range fnb.postInitFns {
		if err := fnb.handlePostInit(f); err != nil {
			return err
		}
	}

	if err := fnb.EnqueueAdminServerInit(); err != nil {
		return err
	}

	// run all modules
	if err := fnb.handleModules(); err != nil {
		return fmt.Errorf("could not handle modules: %w", err)
	}

	// run all components
	return fnb.handleComponents()
}

// postShutdown is called by the node before exiting
// put any cleanup code here that should be run after all components have stopped
func (fnb *FlowNodeBuilder) postShutdown() error {
	var errs *multierror.Error

	for _, fn := range fnb.postShutdownFns {
		err := fn()
		if err != nil {
			errs = multierror.Append(errs, err)
		}
	}
	fnb.Logger.Info().Msg("database has been closed")
	return errs.ErrorOrNil()
}

// handleFatal handles irrecoverable errors by logging them and exiting the process.
func (fnb *FlowNodeBuilder) handleFatal(err error) {
	fnb.Logger.Fatal().Err(err).Msg("unhandled irrecoverable error")
}

func (fnb *FlowNodeBuilder) handlePreInit(f BuilderFunc) error {
	return f(fnb.NodeConfig)
}

func (fnb *FlowNodeBuilder) handlePostInit(f BuilderFunc) error {
	return f(fnb.NodeConfig)
}

func (fnb *FlowNodeBuilder) extraFlagsValidation() error {
	if fnb.extraFlagCheck != nil {
		err := fnb.extraFlagCheck()
		if err != nil {
			return fmt.Errorf("invalid flags: %w", err)
		}
	}
	return nil
}
