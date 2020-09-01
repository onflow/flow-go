package util

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/module/metrics"
	pbadger "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/storage/util"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func ProtocolState(t testing.TB, db *badger.DB) *pbadger.State {
	metrics := metrics.NewNoopCollector()
	headers, _, seals, index, payloads, blocks, setups, commits := util.StorageLayer(t, db)
	proto, err := pbadger.NewState(metrics, db, headers, seals, index, payloads, blocks, setups, commits)
	require.NoError(t, err)
	return proto
}

func RunWithProtocolState(t testing.TB, f func(*badger.DB, *pbadger.State)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		proto := ProtocolState(t, db)
		f(db, proto)
	})
}
