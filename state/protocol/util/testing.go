package util

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/module/metrics"
	protocol "github.com/dapperlabs/flow-go/state/protocol/badger"
	"github.com/dapperlabs/flow-go/storage/util"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func ProtocolState(t testing.TB, db *badger.DB, options ...func(*protocol.State)) *protocol.State {
	metrics := metrics.NewNoopCollector()
	headers, identities, _, seals, index, payloads, blocks := util.StorageLayer(t, db)
	proto, err := protocol.NewState(metrics, db, headers, identities, seals, index, payloads, blocks, options...)
	require.NoError(t, err)
	return proto
}

func RunWithProtocolState(t testing.TB, f func(*badger.DB, *protocol.State), options ...func(*protocol.State)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		proto := ProtocolState(t, db, options...)
		f(db, proto)
	})
}
