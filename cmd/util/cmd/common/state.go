package common

import (
	"fmt"

	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	protocolpebble "github.com/onflow/flow-go/state/protocol/pebble"
	"github.com/onflow/flow-go/storage"
)

func InitProtocolStatePebble(db *pebble.DB, storages *storage.All) (protocol.State, error) {
	metrics := &metrics.NoopCollector{}

	protocolState, err := protocolpebble.OpenState(
		metrics,
		db,
		storages.Headers,
		storages.Seals,
		storages.Results,
		storages.Blocks,
		storages.QuorumCertificates,
		storages.Setups,
		storages.EpochCommits,
		storages.Statuses,
		storages.VersionBeacons,
	)

	if err != nil {
		return nil, fmt.Errorf("could not init pebble based protocol state: %w", err)
	}

	return protocolState, nil
}
