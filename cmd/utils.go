package cmd

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/distributor"
	"github.com/onflow/flow-go/network/p2p/inspector"
	"github.com/onflow/flow-go/network/p2p/p2pbuilder"
	"github.com/onflow/flow-go/network/p2p/p2pnode"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/io"
)

// loadRootProtocolSnapshot loads the root protocol snapshot from disk
func loadRootProtocolSnapshot(dir string) (*inmem.Snapshot, error) {
	path := filepath.Join(dir, bootstrap.PathRootProtocolStateSnapshot)
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read root snapshot (path=%s): %w", path, err)
	}

	var snapshot inmem.EncodableSnapshot
	err = json.Unmarshal(data, &snapshot)
	if err != nil {
		return nil, err
	}

	return inmem.SnapshotFromEncodable(snapshot), nil
}

// LoadPrivateNodeInfo the private info for this node from disk (e.g., private staking/network keys).
func LoadPrivateNodeInfo(dir string, myID flow.Identifier) (*bootstrap.NodeInfoPriv, error) {
	path := filepath.Join(dir, fmt.Sprintf(bootstrap.PathNodeInfoPriv, myID))
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read private node info (path=%s): %w", path, err)
	}
	var info bootstrap.NodeInfoPriv
	err = json.Unmarshal(data, &info)
	return &info, err
}

// loadSecretsEncryptionKey loads the encryption key for the secrets database.
// If the file does not exist, returns os.ErrNotExist.
func loadSecretsEncryptionKey(dir string, myID flow.Identifier) ([]byte, error) {
	path := filepath.Join(dir, fmt.Sprintf(bootstrap.PathSecretsEncryptionKey, myID))
	data, err := io.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("could not read secrets db encryption key (path=%s): %w", path, err)
	}
	return data, nil
}

func rateLimiterPeerFilter(rateLimiter p2p.RateLimiter) p2p.PeerFilter {
	return func(p peer.ID) error {
		if rateLimiter.IsRateLimited(p) {
			return fmt.Errorf("peer is rate limited")
		}

		return nil
	}
}

// BuildDisallowListNotificationDisseminator builds the disallow list notification distributor.
func BuildDisallowListNotificationDisseminator(size uint32, metricsRegistry prometheus.Registerer, logger zerolog.Logger, metricsEnabled bool) p2p.DisallowListNotificationDistributor {
	heroStoreOpts := []queue.HeroStoreConfigOption{queue.WithHeroStoreSizeLimit(size)}
	if metricsEnabled {
		collector := metrics.DisallowListNotificationQueueMetricFactory(metricsRegistry)
		heroStoreOpts = append(heroStoreOpts, queue.WithHeroStoreCollector(collector))
	}
	return distributor.DefaultDisallowListNotificationDistributor(logger, heroStoreOpts...)
}

// BuildGossipsubRPCValidationInspectorNotificationDisseminator builds the gossipsub rpc validation inspector notification distributor.
func BuildGossipsubRPCValidationInspectorNotificationDisseminator(size uint32, metricsRegistry prometheus.Registerer, logger zerolog.Logger, metricsEnabled bool) p2p.GossipSubInspectorNotificationDistributor {
	heroStoreOpts := []queue.HeroStoreConfigOption{queue.WithHeroStoreSizeLimit(size)}
	if metricsEnabled {
		collector := metrics.RpcInspectorNotificationQueueMetricFactory(metricsRegistry)
		heroStoreOpts = append(heroStoreOpts, queue.WithHeroStoreCollector(collector))
	}
	return distributor.DefaultGossipSubInspectorNotificationDistributor(logger, heroStoreOpts...)
}

// buildGossipsubRPCInspectorHeroStoreOpts builds the gossipsub rpc validation inspector hero store opts.
// These options are used in the underlying worker pool hero store.
func buildGossipsubRPCInspectorHeroStoreOpts(size uint32, collectorFactory func() *metrics.HeroCacheCollector, metricsEnabled bool) []queue.HeroStoreConfigOption {
	heroStoreOpts := []queue.HeroStoreConfigOption{queue.WithHeroStoreSizeLimit(size)}
	if metricsEnabled {
		heroStoreOpts = append(heroStoreOpts, queue.WithHeroStoreCollector(collectorFactory()))
	}
	return heroStoreOpts
}

// BuildGossipSubRPCInspectors builds the gossipsub metrics and validation inspectors.
func BuildGossipSubRPCInspectors(logger zerolog.Logger,
	sporkID flow.Identifier,
	inspectorsConfig *GossipSubRPCInspectorsConfig,
	distributor p2p.GossipSubInspectorNotificationDistributor,
	netMetrics module.NetworkMetrics,
	metricsRegistry prometheus.Registerer,
	metricsEnabled,
	publicNetwork bool) ([]p2p.GossipSubRPCInspector, error) {
	// setup RPC metrics inspector
	gossipSubMetrics := p2pnode.NewGossipSubControlMessageMetrics(netMetrics, logger)
	metricsInspectorHeroStoreOpts := buildGossipsubRPCInspectorHeroStoreOpts(inspectorsConfig.MetricsInspectorConfigs.CacheSize, func() *metrics.HeroCacheCollector {
		return metrics.GossipSubRPCMetricsObserverInspectorQueueMetricFactory(publicNetwork, metricsRegistry)
	}, metricsEnabled)
	metricsInspector := inspector.NewControlMsgMetricsInspector(logger, gossipSubMetrics, inspectorsConfig.MetricsInspectorConfigs.NumberOfWorkers, metricsInspectorHeroStoreOpts...)
	// setup RPC validation inspector
	rpcValidationInspectorHeroStoreOpts := buildGossipsubRPCInspectorHeroStoreOpts(inspectorsConfig.ValidationInspectorConfigs.CacheSize, func() *metrics.HeroCacheCollector {
		return metrics.GossipSubRPCValidationInspectorQueueMetricFactory(publicNetwork, metricsRegistry)
	}, metricsEnabled)
	validationInspector, err := p2pbuilder.BuildGossipSubRPCValidationInspector(logger, sporkID, inspectorsConfig.ValidationInspectorConfigs, distributor, rpcValidationInspectorHeroStoreOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create gossipsub rpc validation inspector: %w", err)
	}

	return []p2p.GossipSubRPCInspector{metricsInspector, validationInspector}, nil
}
