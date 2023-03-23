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
	"github.com/onflow/flow-go/module/mempool/queue"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/network/p2p"
	"github.com/onflow/flow-go/network/p2p/distributor"
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

func BuildDisallowListNotificationDisseminator(size uint32, metricsRegistry prometheus.Registerer, logger zerolog.Logger, metricsEnabled bool) p2p.DisallowListNotificationDistributor {
	heroStoreOpts := []queue.HeroStoreConfigOption{queue.WithHeroStoreSizeLimit(size)}
	if metricsEnabled {
		collector := metrics.DisallowListNotificationQueueMetricFactory(metricsRegistry)
		heroStoreOpts = append(heroStoreOpts, queue.WithHeroStoreCollector(collector))
	}
	return distributor.DefaultDisallowListNotificationDistributor(logger, heroStoreOpts...)
}

func BuildGossipsubRPCValidationInspectorNotificationDisseminator(size uint32, metricsRegistry prometheus.Registerer, logger zerolog.Logger, metricsEnabled bool) p2p.GossipSubInspectorNotificationDistributor {
	heroStoreOpts := []queue.HeroStoreConfigOption{queue.WithHeroStoreSizeLimit(size)}
	if metricsEnabled {
		collector := metrics.RpcInspectorNotificationQueueMetricFactory(metricsRegistry)
		heroStoreOpts = append(heroStoreOpts, queue.WithHeroStoreCollector(collector))
	}
	return distributor.DefaultGossipSubInspectorNotificationDistributor(logger, heroStoreOpts...)
}

func BuildGossipsubRPCValidationInspectorHeroStoreOpts(size uint32, metricsRegistry prometheus.Registerer, metricsEnabled bool) []queue.HeroStoreConfigOption {
	heroStoreOpts := []queue.HeroStoreConfigOption{queue.WithHeroStoreSizeLimit(size)}
	if metricsEnabled {
		collector := metrics.GossipSubRPCInspectorQueueMetricFactory(metricsRegistry)
		heroStoreOpts = append(heroStoreOpts, queue.WithHeroStoreCollector(collector))
	}
	return heroStoreOpts
}
