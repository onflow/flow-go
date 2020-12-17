package util

import (
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	"github.com/onflow/flow-go/state/protocol"
	pbadger "github.com/onflow/flow-go/state/protocol/badger"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage/util"
	"github.com/onflow/flow-go/utils/unittest"
)

func RunWithBootstrapState(t testing.TB, stateRoot *pbadger.StateRoot, f func(*badger.DB, *pbadger.State)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		headers, _, seals, _, _, blocks, setups, commits, statuses := util.StorageLayer(t, db)
		stateRoot, err := pbadger.NewStateRoot(stateRoot.Block(), stateRoot.Result(), stateRoot.Seal(), stateRoot.EpochSetupEvent().FirstView)
		require.NoError(t, err)
		state, err := pbadger.Bootstrap(metrics, db, headers, seals, blocks, setups, commits, statuses, stateRoot)
		require.NoError(t, err)
		f(db, state)
	})
}

func RunWithFullProtocolState(t testing.TB, stateRoot *pbadger.StateRoot, f func(*badger.DB, *pbadger.MutableState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		consumer := events.NewNoop()
		headers, _, seals, index, payloads, blocks, setups, commits, statuses := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(metrics, db, headers, seals, blocks, setups, commits, statuses, stateRoot)
		require.NoError(t, err)
		fullState, err := pbadger.NewFullConsensusState(state, index, payloads, tracer, consumer)
		require.NoError(t, err)
		f(db, fullState)
	})
}

func RunWithFollowerProtocolState(t testing.TB, stateRoot *pbadger.StateRoot, f func(*badger.DB, *pbadger.FollowerState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		consumer := events.NewNoop()
		headers, _, seals, index, payloads, blocks, setups, commits, statuses := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(metrics, db, headers, seals, blocks, setups, commits, statuses, stateRoot)
		require.NoError(t, err)
		followerState, err := pbadger.NewFollowerState(state, index, payloads, tracer, consumer)
		require.NoError(t, err)
		f(db, followerState)
	})
}

func RunWithFullProtocolStateAndConsumer(t testing.TB, stateRoot *pbadger.StateRoot, consumer protocol.Consumer, f func(*badger.DB, *pbadger.MutableState)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		metrics := metrics.NewNoopCollector()
		tracer := trace.NewNoopTracer()
		headers, _, seals, index, payloads, blocks, setups, commits, statuses := util.StorageLayer(t, db)
		state, err := pbadger.Bootstrap(metrics, db, headers, seals, blocks, setups, commits, statuses, stateRoot)
		require.NoError(t, err)
		fullState, err := pbadger.NewFullConsensusState(state, index, payloads, tracer, consumer)
		require.NoError(t, err)
		f(db, fullState)
	})
}
