package common

import (
	"fmt"

	"github.com/jordanschalm/lockctx"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	protocolbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/store"
)

func OpenProtocolState(lockManager lockctx.Manager, db storage.DB, storages *store.All) (protocol.State, error) {
	metrics := &metrics.NoopCollector{}

	protocolState, err := protocolbadger.OpenState(
		metrics,
		db,
		lockManager,
		storages.Headers,
		storages.Seals,
		storages.Results,
		storages.Blocks,
		storages.QuorumCertificates,
		storages.EpochSetups,
		storages.EpochCommits,
		storages.EpochProtocolStateEntries,
		storages.ProtocolKVStore,
		storages.VersionBeacons,
	)

	if err != nil {
		return nil, fmt.Errorf("could not open protocol state: %w", err)
	}

	return protocolState, nil
}
