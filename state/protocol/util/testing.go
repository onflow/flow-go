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

// use this function if you'd like to create your own protocol state with customized dependencies
//func RunWithProtocolStateDeps(t testing.TB, create createState, f func(*badger.DB, *pbadger.State)) {
//	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
//		proto := WithProtocolState(t, db, create)
//		f(db, proto)
//	})
//}

//func RunWithProtocolStateAndConsumer(t testing.TB, consumer protocol.Consumer, f func(*badger.DB, *pbadger.State)) {
//	RunWithProtocolStateDeps(t, func(
//		metrics module.ComplianceMetrics,
//		tracer module.Tracer,
//		db *badger.DB,
//		headers storage.Headers,
//		seals storage.Seals,
//		index storage.Index,
//		payloads storage.Payloads,
//		blocks storage.Blocks,
//		setups storage.EpochSetups,
//		commits storage.EpochCommits,
//		statuses storage.EpochStatuses,
//		c protocol.Consumer,
//	) (*pbadger.State, error) {
//		proto, err := pbadger.NewState(metrics, tracer, db, headers, seals, index, payloads, blocks, setups, commits, statuses, consumer)
//		require.NoError(t, err)
//		return proto, nil
//	}, f)
//}
