package util

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/state/protocol"
	pbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

func ProtocolState(t testing.TB, db *badger.DB) *pbadger.State {
	return WithProtocolState(t, db, pbadger.NewState)
}

type createState func(
	metrics module.ComplianceMetrics,
	db *badger.DB,
	headers storage.Headers,
	seals storage.Seals,
	index storage.Index,
	payloads storage.Payloads,
	blocks storage.Blocks,
	setups storage.EpochSetups,
	commits storage.EpochCommits,
	statuses storage.EpochStatuses,
	consumer protocol.Consumer,
) (*pbadger.State, error)

func WithProtocolState(t testing.TB, db *badger.DB, create createState) *pbadger.State {
	metrics := metrics.NewNoopCollector()
	consumer := events.NewNoop()
	headers, _, seals, index, payloads, blocks, setups, commits, statuses := util.StorageLayer(t, db)
	proto, err := create(metrics, db, headers, seals, index, payloads, blocks, setups, commits, statuses, consumer)
	require.NoError(t, err)
	return proto
}

func RunWithProtocolState(t testing.TB, f func(*badger.DB, *pbadger.State)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		proto := ProtocolState(t, db)
		f(db, proto)
	})
}

// use this function if you'd like to create your own protocol state with customized dependencies
func RunWithProtocolStateDeps(t testing.TB, create createState, f func(*badger.DB, *pbadger.State)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		proto := WithProtocolState(t, db, create)
		f(db, proto)
	})
}

func RunWithProtocolStateAndConsumer(t testing.TB, consumer protocol.Consumer, f func(*badger.DB, *pbadger.State)) {
	RunWithProtocolStateDeps(t, func(
		metrics module.ComplianceMetrics,
		db *badger.DB,
		headers storage.Headers,
		seals storage.Seals,
		index storage.Index,
		payloads storage.Payloads,
		blocks storage.Blocks,
		setups storage.EpochSetups,
		commits storage.EpochCommits,
		statuses storage.EpochStatuses,
		c protocol.Consumer,
	) (*pbadger.State, error) {
		proto, err := pbadger.NewState(metrics, db, headers, seals, index, payloads, blocks, setups, commits, statuses, consumer)
		require.NoError(t, err)
		return proto, nil
	}, f)
}
