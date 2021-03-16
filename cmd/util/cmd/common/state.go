package common

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	protocolbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/storage"
)

func InitProtocolState(db *badger.DB, storages *storage.All) (protocol.State, error) {
	metrics := &metrics.NoopCollector{}

	protocolState, err := protocolbadger.OpenState(
		metrics,
		db,
		storages.Headers,
		storages.Seals,
		storages.Results,
		storages.Blocks,
		storages.Setups,
		storages.EpochCommits,
		storages.Statuses,
	)

	if err != nil {
		return nil, fmt.Errorf("could not init protocol state: %w", err)
	}

	return protocolState, nil
}
