package util

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	pbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

func ProtocolState(t testing.TB, db *badger.DB) *pbadger.State {
	metrics := metrics.NewNoopCollector()
	consumer := events.NewNoop()
	headers, _, seals, index, payloads, blocks, setups, commits, statuses := util.StorageLayer(t, db)
	proto, err := pbadger.NewState(metrics, db, headers, seals, index, payloads, blocks, setups, commits, statuses, consumer)
	require.NoError(t, err)
	return proto
}

func RunWithProtocolState(t testing.TB, f func(*badger.DB, *pbadger.State)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		proto := ProtocolState(t, db)
		f(db, proto)
	})
}
