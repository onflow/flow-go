package common

import (
	"fmt"

	"github.com/dgraph-io/badger/v2"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	protocolbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
)

func InitProtocolState(db *badger.DB, storages *Storages) (protocol.State, error) {
	metrics := &metrics.NoopCollector{}
	tracer := trace.NewNoopTracer()
	distributor := events.NewDistributor()

	protocolState, err := protocolbadger.NewState(
		metrics,
		tracer,
		db,
		storages.Headers,
		storages.Seals,
		storages.Index,
		storages.Payloads,
		storages.Blocks,
		storages.Setups,
		storages.EpochCommits,
		storages.Statuses,
		distributor,
	)

	if err != nil {
		return nil, fmt.Errorf("could not init protocol state: %w", err)
	}

	return protocolState, nil
}
