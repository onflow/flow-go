package state_test

import (
	"context"
	"strconv"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/golang/mock/gomock"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/utils/unittest"

	"github.com/onflow/flow-go/engine/execution/state"
	ledger "github.com/onflow/flow-go/ledger/complete"
	"github.com/onflow/flow-go/ledger/complete/wal/fixtures"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/metrics"
	"github.com/onflow/flow-go/module/trace"
	storage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/storage/mocks"
)

func prepareBenchmark(f func(b *testing.B, es state.ExecutionState, l *ledger.Ledger)) func(*testing.B) {
	return func(b *testing.B) {
		unittest.RunWithBadgerDB(b, func(badgerDB *badger.DB) {
			metricsCollector := &metrics.NoopCollector{}
			diskWal := &fixtures.NoopWAL{}
			ls, err := ledger.NewLedger(diskWal, 100, metricsCollector, zerolog.Nop(), ledger.DefaultPathFinderVersion)
			require.NoError(b, err)

			ctrl := gomock.NewController(b)

			stateCommitments := mocks.NewMockCommits(ctrl)
			blocks := mocks.NewMockBlocks(ctrl)
			headers := mocks.NewMockHeaders(ctrl)
			collections := mocks.NewMockCollections(ctrl)
			events := mocks.NewMockEvents(ctrl)
			serviceEvents := mocks.NewMockServiceEvents(ctrl)
			txResults := mocks.NewMockTransactionResults(ctrl)

			stateCommitment := ls.InitialState()

			stateCommitments.EXPECT().ByBlockID(gomock.Any()).Return(flow.StateCommitment(stateCommitment), nil)

			chunkDataPacks := new(storage.ChunkDataPacks)

			results := new(storage.ExecutionResults)
			myReceipts := new(storage.MyExecutionReceipts)

			es := state.NewExecutionState(
				ls, stateCommitments, blocks, headers, collections, chunkDataPacks, results, myReceipts, events, serviceEvents, txResults, badgerDB, trace.NewNoopTracer(),
			)

			f(b, es, ls)
		})
	}
}

func BenchmarkExecutionStateRead1(b *testing.B) {

	f := prepareBenchmark(func(b *testing.B, es state.ExecutionState, l *ledger.Ledger) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(context.Background(), flow.Identifier{})
		assert.NoError(b, err)

		view1 := es.NewView(sc1)

		registerIDs := make([]flow.RegisterID, b.N)
		for i := 0; i < len(registerIDs); i++ {
			s := strconv.Itoa(i)
			registerIDs[i] = flow.NewRegisterID("owner"+s, "controller"+s, "key"+s)

			err = view1.Set(registerIDs[i].Owner, registerIDs[i].Controller, registerIDs[i].Key, flow.RegisterValue("apple"))
			assert.NoError(b, err)
		}

		sc2, update, err := state.CommitDelta(l, view1.Delta(), sc1)
		assert.NoError(b, err)

		assert.Equal(b, sc1[:], update.RootHash[:])
		assert.Len(b, update.Paths, len(registerIDs))
		assert.Len(b, update.Payloads, len(registerIDs))

		view2 := es.NewView(sc2)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := view2.Get(registerIDs[i].Owner, registerIDs[i].Controller, registerIDs[i].Key)
			if err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
	})
	f(b)
}

func BenchmarkExecutionStateUpdate100(b *testing.B) {

	f := prepareBenchmark(func(b *testing.B, es state.ExecutionState, l *ledger.Ledger) {
		// TODO: use real block ID
		sc1, err := es.StateCommitmentByBlockID(context.Background(), flow.Identifier{})
		assert.NoError(b, err)

		view1 := es.NewView(sc1)

		registerIDs := make([]flow.RegisterID, 100)
		for i := 0; i < len(registerIDs); i++ {
			s := strconv.Itoa(i)
			registerIDs[i] = flow.NewRegisterID("owner"+s, "controller"+s, "key"+s)

			err = view1.Set(registerIDs[i].Owner, registerIDs[i].Controller, registerIDs[i].Key, flow.RegisterValue("apple"))
			assert.NoError(b, err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, _, err := state.CommitDelta(l, view1.Delta(), sc1)
			if err != nil {
				b.Fatal(err)
			}
		}
		b.StopTimer()
	})
	f(b)
}
