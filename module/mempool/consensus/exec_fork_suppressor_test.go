package consensus

import (
	"os"
	"testing"

	"github.com/cockroachdb/pebble"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	actormock "github.com/onflow/flow-go/module/mempool/consensus/mock"
	poolmock "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	mockstorage "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test_Construction verifies correctness of the initial size and limit values
func Test_Construction(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		wrappedMempool.On("Size").Return(uint(0)).Once()
		require.Equal(t, uint(0), wrapper.Size())
		wrappedMempool.On("Limit").Return(uint(0)).Once()
		require.Equal(t, uint(0), wrapper.Limit())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_Size checks that ExecForkSuppressor is reporting the size of the wrapped mempool
func Test_Size(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		wrappedMempool.On("Size").Return(uint(139)).Once()
		require.Equal(t, uint(139), wrapper.Size())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_Limit checks that ExecForkSuppressor is reporting the capacity limit of the wrapped mempool
func Test_Limit(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		wrappedMempool.On("Limit").Return(uint(227)).Once()
		require.Equal(t, uint(227), wrapper.Limit())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_Clear checks that, when clearing the ExecForkSuppressor:
//   - the wrapper also clears the wrapped mempool;
//   - the reported mempool size, _after_ clearing should be zero
func Test_Clear(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		wrappedMempool.On("Clear").Return().Once()

		wrapper.Clear()
		wrappedMempool.On("Size").Return(uint(0))
		require.Equal(t, uint(0), wrapper.Size())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_All checks that ExecForkSuppressor.All() is returning the elements of the wrapped mempool
func Test_All(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		expectedSeals := unittest.IncorporatedResultSeal.Fixtures(7)
		wrappedMempool.On("All").Return(expectedSeals)
		retrievedSeals := wrapper.All()
		require.ElementsMatch(t, expectedSeals, retrievedSeals)
	})
}

// Test_Add adds IncorporatedResultSeals for
//   - 2 different blocks
//   - for each block, we generate one specific result,
//     for which we add 3 IncorporatedResultSeals
//     o IncorporatedResultSeal (1):
//     incorporated in block B1
//     o IncorporatedResultSeal (2):
//     incorporated in block B2
//     o IncorporatedResultSeal (3):
//     same result as (1) and incorporated in same block B1;
//     should be automatically de-duplicated (irrespective of approvals on the seal).
func Test_Add(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		for _, block := range unittest.BlockFixtures(2) {
			result := unittest.ExecutionResultFixture(unittest.WithBlock(block))

			// IncorporatedResultSeal (1):
			irSeal1 := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(result))
			wrappedMempool.On("Add", irSeal1).Return(true, nil).Once()
			added, err := wrapper.Add(irSeal1)
			require.NoError(t, err)
			require.True(t, added)
			wrappedMempool.AssertExpectations(t)

			// IncorporatedResultSeal (2):
			// the value for IncorporatedResultSeal.IncorporatedResult.IncorporatedBlockID is randomly
			// generated and therefore, will be different from for irSeal1
			irSeal2 := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(result))
			require.False(t, irSeal1.ID() == irSeal2.ID()) // incorporated in different block => different seal ID expected
			wrappedMempool.On("Add", irSeal2).Return(true, nil).Once()
			added, err = wrapper.Add(irSeal2)
			require.NoError(t, err)
			require.True(t, added)
			wrappedMempool.AssertExpectations(t)

			// IncorporatedResultSeal (3):
			irSeal3 := unittest.IncorporatedResultSeal.Fixture(
				unittest.IncorporatedResultSeal.WithResult(result),
				unittest.IncorporatedResultSeal.WithIncorporatedBlockID(irSeal1.IncorporatedResult.IncorporatedBlockID),
			)
			require.True(t, irSeal1.ID() == irSeal3.ID())               // same result incorporated same block as (1) => identical ID expected
			wrappedMempool.On("Add", irSeal3).Return(false, nil).Once() // deduplicate
			added, err = wrapper.Add(irSeal3)
			require.NoError(t, err)
			require.False(t, added)
			wrappedMempool.AssertExpectations(t)
		}
	})
}

// Test_Remove checks that ExecForkSuppressor.Remove()
//   - delegates the call to the underlying mempool
func Test_Remove(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		// element is in wrapped mempool: Remove should be called
		seal := unittest.IncorporatedResultSeal.Fixture()
		wrappedMempool.On("Add", seal).Return(true, nil).Once()
		wrappedMempool.On("ByID", seal.ID()).Return(seal, true)
		added, err := wrapper.Add(seal)
		require.NoError(t, err)
		require.True(t, added)

		wrappedMempool.On("ByID", seal.ID()).Return(seal, true)
		wrappedMempool.On("Remove", seal.ID()).Return(true).Once()
		removed := wrapper.Remove(seal.ID())
		require.True(t, removed)
		wrappedMempool.AssertExpectations(t)

		// element _not_ in wrapped mempool: Remove might be called
		seal = unittest.IncorporatedResultSeal.Fixture()
		wrappedMempool.On("ByID", seal.ID()).Return(seal, false)
		wrappedMempool.On("Remove", seal.ID()).Return(false).Maybe()
		removed = wrapper.Remove(seal.ID())
		require.False(t, removed)
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_RejectInvalidSeals verifies that ExecForkSuppressor rejects seals whose
// which don't have a chunk (i.e. their start and end state of the result cannot be determined)
func Test_RejectInvalidSeals(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		irSeal := unittest.IncorporatedResultSeal.Fixture()
		irSeal.IncorporatedResult.Result.Chunks = make(flow.ChunkList, 0)
		irSeal.Seal.FinalState = flow.DummyStateCommitment

		added, err := wrapper.Add(irSeal)
		require.Error(t, err)
		require.True(t, engine.IsInvalidInputError(err))
		require.False(t, added)
	})
}

// Test_ConflictingResults verifies that ExecForkSuppressor detects a fork in the execution chain.
// The expected behaviour is:
//   - clear the wrapped mempool
//   - reject addition of all further entities (even valid seals)
//
// This logic has to be executed for all queries(`ByID`, `All`)
func Test_ConflictingResults(t *testing.T) {
	assertConflictingResult := func(t *testing.T, action func(irSeals []*flow.IncorporatedResultSeal, conflictingSeal *flow.IncorporatedResultSeal, wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals)) {
		WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
			// add 3 random irSeals
			irSeals := unittest.IncorporatedResultSeal.Fixtures(3)
			for _, s := range irSeals {
				wrappedMempool.On("Add", s).Return(true, nil).Once()
				added, err := wrapper.Add(s)
				require.NoError(t, err)
				require.True(t, added)
			}

			// add seal for result that is _conflicting_ with irSeals[1]
			result := unittest.ExecutionResultFixture()
			result.BlockID = irSeals[1].Seal.BlockID
			for _, c := range result.Chunks {
				c.BlockID = result.BlockID
			}
			conflictingSeal := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(result))

			wrappedMempool.On("Clear").Return().Once()
			wrappedMempool.On("Add", conflictingSeal).Return(true, nil).Once()
			// add conflicting seal
			added, err := wrapper.Add(conflictingSeal)
			require.NoError(t, err)
			require.True(t, added)

			execForkActor.On("OnExecFork", mock.Anything).Run(func(args mock.Arguments) {
				conflictingSeals := args.Get(0).([]*flow.IncorporatedResultSeal)
				require.ElementsMatch(t, []*flow.IncorporatedResultSeal{irSeals[1], conflictingSeal}, conflictingSeals)
			}).Return().Once()
			action(irSeals, conflictingSeal, wrapper, wrappedMempool)

			wrappedMempool.On("ByID", conflictingSeal.ID()).Return(nil, false).Once()
			byID, found := wrapper.ByID(conflictingSeal.ID())
			require.False(t, found)
			require.Nil(t, byID)

			// mempool should be cleared
			wrappedMempool.On("Size").Return(uint(0)) // we asserted that Clear was called on wrappedMempool
			require.Equal(t, uint(0), wrapper.Size())

			// additional seals should not be accepted anymore
			added, err = wrapper.Add(unittest.IncorporatedResultSeal.Fixture())
			require.NoError(t, err)
			require.False(t, added)
			require.Equal(t, uint(0), wrapper.Size())

			wrappedMempool.AssertExpectations(t)
			execForkActor.AssertExpectations(t)
		})
	}

	t.Run("all-query", func(t *testing.T) {
		assertConflictingResult(t, func(irSeals []*flow.IncorporatedResultSeal, conflictingSeal *flow.IncorporatedResultSeal, wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals) {
			wrappedMempool.On("All").Return(append(irSeals, conflictingSeal)).Once()
			allSeals := wrapper.All()
			require.Len(t, allSeals, 0)
		})
	})
	t.Run("by-id-query", func(t *testing.T) {
		assertConflictingResult(t, func(irSeals []*flow.IncorporatedResultSeal, conflictingSeal *flow.IncorporatedResultSeal, wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals) {
			wrappedMempool.On("ByID", conflictingSeal.ID()).Return(conflictingSeal, true).Once()
			byID, found := wrapper.ByID(conflictingSeal.ID())
			require.False(t, found)
			require.Nil(t, byID)
		})
	})

}

// Test_ForkDetectionPersisted verifies that, when ExecForkSuppressor detects a fork, this information is
// persisted in the data base
func Test_ForkDetectionPersisted(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		db := unittest.PebbleDB(t, dir)
		defer db.Close()

		// initialize ExecForkSuppressor
		wrappedMempool := &poolmock.IncorporatedResultSeals{}
		execForkActor := &actormock.ExecForkActorMock{}
		wrapper, _ := NewExecStateForkSuppressor(wrappedMempool, execForkActor.OnExecFork, db, zerolog.New(os.Stderr))

		// add seal
		block := unittest.BlockFixture()
		sealA := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&block))))
		wrappedMempool.On("Add", sealA).Return(true, nil).Once()
		_, _ = wrapper.Add(sealA)

		// add conflicting seal
		sealB := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&block))))
		wrappedMempool.On("Add", sealB).Return(true, nil).Once()
		added, _ := wrapper.Add(sealB) // should be rejected because it is conflicting with sealA
		require.True(t, added)

		wrappedMempool.On("ByID", sealA.ID()).Return(sealA, true).Once()
		execForkActor.On("OnExecFork", mock.Anything).Run(func(args mock.Arguments) {
			conflictingSeals := args.Get(0).([]*flow.IncorporatedResultSeal)
			require.ElementsMatch(t, []*flow.IncorporatedResultSeal{sealA, sealB}, conflictingSeals)
		}).Return().Once()
		wrappedMempool.On("Clear").Return().Once()
		// try to query, at this point we will detect a conflicting seal
		wrapper.ByID(sealA.ID())

		wrappedMempool.AssertExpectations(t)
		execForkActor.AssertExpectations(t)

		// crash => re-initialization
		db.Close()
		db2 := unittest.PebbleDB(t, dir)
		wrappedMempool2 := &poolmock.IncorporatedResultSeals{}
		execForkActor2 := &actormock.ExecForkActorMock{}
		execForkActor2.On("OnExecFork", mock.Anything).
			Run(func(args mock.Arguments) {
				conflictingSeals := args.Get(0).([]*flow.IncorporatedResultSeal)
				require.ElementsMatch(t, []*flow.IncorporatedResultSeal{sealA, sealB}, conflictingSeals)
			}).Return().Once()
		wrapper2, _ := NewExecStateForkSuppressor(wrappedMempool2, execForkActor2.OnExecFork, db2, zerolog.New(os.Stderr))

		// add another (non-conflicting) seal to ExecForkSuppressor
		// fail test if seal is added to wrapped mempool
		wrappedMempool2.On("Add", mock.Anything).
			Run(func(args mock.Arguments) { require.Fail(t, "seal was added to wrapped mempool") }).
			Return(true, nil).Maybe()
		added, _ = wrapper2.Add(unittest.IncorporatedResultSeal.Fixture())
		require.False(t, added)
		wrappedMempool2.On("Size").Return(uint(0)) // we asserted that Clear was called on wrappedMempool
		require.Equal(t, uint(0), wrapper2.Size())

		wrappedMempool2.AssertExpectations(t)
		execForkActor2.AssertExpectations(t)
	})
}

// Test_AddRemove_SmokeTest tests a real system of stdmap.IncorporatedResultSeals mempool
// which is wrapped in an ExecForkSuppressor.
// We add and remove lots of different seals.
func Test_AddRemove_SmokeTest(t *testing.T) {
	onExecFork := func([]*flow.IncorporatedResultSeal) {
		require.Fail(t, "no call to onExecFork expected ")
	}
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		wrappedMempool := stdmap.NewIncorporatedResultSeals(100)
		wrapper, err := NewExecStateForkSuppressor(wrappedMempool, onExecFork, db, zerolog.New(os.Stderr))
		require.NoError(t, err)
		require.NotNil(t, wrapper)

		// Run 100 experiments of the following kind:
		//  * add 10 seals to mempool, which should eject 7 seals
		//  * test that ejected seals are not in mempool anymore
		//  * remove remaining seals
		for i := 0; i < 100; i++ {
			seals := unittest.IncorporatedResultSeal.Fixtures(10)
			for j, s := range seals {
				// fix height for each seal
				s.Header.Height = uint64(i*100 + j)
				added, err := wrapper.Add(s)
				require.NoError(t, err)
				require.True(t, added)
			}

			require.Equal(t, uint(10), wrapper.seals.Size())
			require.Equal(t, uint(10), wrapper.Size())

			err := wrapper.PruneUpToHeight(uint64((i + 1) * 100))
			require.NoError(t, err)

			require.Equal(t, uint(0), wrapper.seals.Size())
			require.Equal(t, uint(0), wrapper.Size())
			require.Len(t, wrapper.sealsForBlock, 0)
		}
	})
}

// Test_ConflictingSeal_SmokeTest tests a real system where we combine stdmap.IncorporatedResultSeals, consensus.IncorporatedResultSeals and
// ExecForkSuppressor. We wrap stdmap.IncorporatedResultSeals with consensus.IncorporatedResultSeals which is wrapped with ExecForkSuppressor.
// Test adding conflicting seals with different number of matching receipts.
func Test_ConflictingSeal_SmokeTest(t *testing.T) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		executingForkDetected := atomic.NewBool(false)
		onExecFork := func([]*flow.IncorporatedResultSeal) {
			executingForkDetected.Store(true)
		}

		rawMempool := stdmap.NewIncorporatedResultSeals(100)
		receiptsDB := mockstorage.NewExecutionReceipts(t)
		wrappedMempool := NewIncorporatedResultSeals(rawMempool, receiptsDB)
		wrapper, err := NewExecStateForkSuppressor(wrappedMempool, onExecFork, db, zerolog.New(os.Stderr))
		require.NoError(t, err)
		require.NotNil(t, wrapper)

		// add three seals
		// two of them are non-conflicting but for same block and one is conflicting.

		block := unittest.BlockFixture()
		sealA := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&block))))
		_, _ = wrapper.Add(sealA)

		// different seal but for same result
		sealB := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(sealA.IncorporatedResult.Result))
		_, _ = wrapper.Add(sealB)

		receiptsDB.On("ByBlockID", block.ID()).Return(nil, nil).Twice()

		// two seals, but they are not confirmed with receipts
		seals := wrapper.All()
		require.Empty(t, seals)

		receipts := flow.ExecutionReceiptList{
			unittest.ExecutionReceiptFixture(unittest.WithResult(sealA.IncorporatedResult.Result)),
			unittest.ExecutionReceiptFixture(unittest.WithResult(sealB.IncorporatedResult.Result)),
		}
		receiptsDB.On("ByBlockID", block.ID()).Return(receipts, nil).Twice()

		// at this point we have two seals, confirmed by two receipts but no execution fork
		seals = wrapper.All()
		require.ElementsMatch(t, []*flow.IncorporatedResultSeal{sealA, sealB}, seals)

		// add conflicting seal, which doesn't have any receipts yet
		conflictingSeal := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&block))))
		_, _ = wrapper.Add(conflictingSeal)

		// conflicting seal doesn't have any receipts yet
		receiptsDB.On("ByBlockID", block.ID()).Return(receipts, nil).Times(3)

		seals = wrapper.All()
		require.ElementsMatch(t, []*flow.IncorporatedResultSeal{sealA, sealB}, seals)

		// add two receipts for conflicting result
		receipts = append(receipts,
			unittest.ExecutionReceiptFixture(unittest.WithResult(conflictingSeal.IncorporatedResult.Result)),
			unittest.ExecutionReceiptFixture(unittest.WithResult(conflictingSeal.IncorporatedResult.Result)),
		)

		receiptsDB.On("ByBlockID", block.ID()).Return(receipts, nil).Times(3)

		// querying should detect execution fork
		seals = wrapper.All()
		require.Empty(t, seals)
		require.True(t, executingForkDetected.Load())
	})
}

// WithExecStateForkSuppressor
//  1. constructs a mock (aka `wrappedMempool`) of an IncorporatedResultSeals mempool
//  2. wraps `wrappedMempool` in a ExecForkSuppressor
//  3. ensures that initializing the wrapper did not error
//  4. executes the `testLogic`
func WithExecStateForkSuppressor(t testing.TB, testLogic func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock)) {
	unittest.RunWithPebbleDB(t, func(db *pebble.DB) {
		wrappedMempool := &poolmock.IncorporatedResultSeals{}
		execForkActor := &actormock.ExecForkActorMock{}
		wrapper, err := NewExecStateForkSuppressor(wrappedMempool, execForkActor.OnExecFork, db, zerolog.New(os.Stderr))
		require.NoError(t, err)
		require.NotNil(t, wrapper)
		testLogic(wrapper, wrappedMempool, execForkActor)
	})
}
