package consensus

import (
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	actormock "github.com/onflow/flow-go/module/mempool/consensus/mock"
	poolmock "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/module/mempool/stdmap"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test_ImplementsInterfaces is a compile-time check:
// verifies that ExecForkSuppressor implements mempool.IncorporatedResultSeals interface
func Test_ImplementsInterfaces(t *testing.T) {
	var _ mempool.IncorporatedResultSeals = &ExecForkSuppressor{}
}

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
//   * the wrapper also clears the wrapped mempool;
//   * the reported mempool size, _after_ clearing should be zero
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
		require.Equal(t, len(expectedSeals), len(retrievedSeals))
		for i := 0; i < len(expectedSeals); i++ {
			require.Equal(t, expectedSeals[i].ID(), retrievedSeals[i].ID())
		}
	})
}

// Test_Add adds IncorporatedResultSeals for
//   * 2 different blocks
//   * for each block, we generate one specific result,
//     for which we add 3 IncorporatedResultSeals
//      o IncorporatedResultSeal (1):
//        incorporated in block B1
//      o IncorporatedResultSeal (2):
//        incorporated in block B2
//      o IncorporatedResultSeal (3):
//        same result as (1) and incorporated in same block B1;
//        should be automatically de-duplicated (irrespective of approvals on the seal).
func Test_Add(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		for _, block := range unittest.BlockFixtures(2) {
			result := unittest.ExecutionResultFixture(unittest.WithBlock(block))

			// IncorporatedResultSeal (1):
			irSeal1 := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(result))
			wrappedMempool.On("Add", irSeal1).Return(true, nil).Once()
			wrappedMempool.On("ByID", irSeal1.ID()).Return(irSeal1, true)
			added, err := wrapper.Add(irSeal1)
			assert.NoError(t, err)
			assert.True(t, added)
			wrappedMempool.AssertExpectations(t)

			// IncorporatedResultSeal (2):
			// the value for IncorporatedResultSeal.IncorporatedResult.IncorporatedBlockID is randomly
			// generated and therefore, will be different than for irSeal1
			irSeal2 := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(result))
			assert.False(t, irSeal1.ID() == irSeal2.ID()) // incorporated in different block => different seal ID expected
			wrappedMempool.On("Add", irSeal2).Return(true, nil).Once()
			wrappedMempool.On("ByID", irSeal2.ID()).Return(irSeal2, true)
			added, err = wrapper.Add(irSeal2)
			assert.NoError(t, err)
			assert.True(t, added)
			wrappedMempool.AssertExpectations(t)

			// IncorporatedResultSeal (3):
			irSeal3 := unittest.IncorporatedResultSeal.Fixture(
				unittest.IncorporatedResultSeal.WithResult(result),
				unittest.IncorporatedResultSeal.WithIncorporatedBlockID(irSeal1.IncorporatedResult.IncorporatedBlockID),
			)
			assert.True(t, irSeal1.ID() == irSeal3.ID())                // same result incorporated same block as (1) => identical ID expected
			wrappedMempool.On("Add", irSeal3).Return(false, nil).Once() // deduplicate
			wrappedMempool.On("ByID", irSeal3.ID()).Return(nil, false)
			added, err = wrapper.Add(irSeal3)
			assert.NoError(t, err)
			assert.False(t, added)
			wrappedMempool.AssertExpectations(t)
		}
	})
}

// Test_Rem checks that ExecForkSuppressor.Rem()
//   * delegates the call to the underlying mempool
func Test_Rem(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		// element is in wrapped mempool: Rem should be called
		seal := unittest.IncorporatedResultSeal.Fixture()
		wrappedMempool.On("Add", seal).Return(true, nil).Once()
		wrappedMempool.On("ByID", seal.ID()).Return(seal, true)
		added, err := wrapper.Add(seal)
		assert.NoError(t, err)
		assert.True(t, added)

		wrappedMempool.On("ByID", seal.ID()).Return(seal, true)
		wrappedMempool.On("Rem", seal.ID()).Return(true).Once()
		removed := wrapper.Rem(seal.ID())
		require.Equal(t, true, removed)
		wrappedMempool.AssertExpectations(t)

		// element _not_ in wrapped mempool: Rem might be called
		seal = unittest.IncorporatedResultSeal.Fixture()
		wrappedMempool.On("ByID", seal.ID()).Return(seal, false)
		wrappedMempool.On("Rem", seal.ID()).Return(false).Maybe()
		removed = wrapper.Rem(seal.ID())
		require.Equal(t, false, removed)
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_RejectInvalidSeals verifies that ExecForkSuppressor rejects seals whose
// end state is invalid. Specifically, this can happen if:
//  * the end state of the last chunk is empty
//  * there are no chunks in the result (invalid result, as system chunk is missing)
func Test_RejectInvalidSeals(t *testing.T) {

	t.Run("reject seal without chunks", func(t *testing.T) {
		WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
			irSeal := unittest.IncorporatedResultSeal.Fixture()
			irSeal.IncorporatedResult.Result.Chunks = make(flow.ChunkList, 0)
			irSeal.Seal.FinalState = flow.DummyStateCommitment

			added, err := wrapper.Add(irSeal)
			assert.Error(t, err)
			assert.True(t, engine.IsInvalidInputError(err))
			assert.False(t, added)
		})
	})
}

// Test_ConflictingResults verifies that ExecForkSuppressor detects a fork in the execution chain.
// The expected behaviour is:
//  * clear the wrapped mempool
//  * reject addition of all further entities (even valid seals)
func Test_ConflictingResults(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock) {
		// add 3 random irSeals
		irSeals := unittest.IncorporatedResultSeal.Fixtures(3)
		for _, s := range irSeals {
			wrappedMempool.On("Add", s).Return(true, nil).Once()
			wrappedMempool.On("ByID", s.ID()).Return(s, true)
			added, err := wrapper.Add(s)
			assert.NoError(t, err)
			assert.True(t, added)
		}

		// add seal for result that is _conflicting_ with irSeals[1]
		result := unittest.ExecutionResultFixture()
		result.BlockID = irSeals[1].Seal.BlockID
		for _, c := range result.Chunks {
			c.BlockID = result.BlockID
		}
		conflictingSeal := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(result))

		wrappedMempool.On("Clear").Return().Once()
		execForkActor.On("OnExecFork", []*flow.IncorporatedResultSeal{conflictingSeal, irSeals[1]}).Return().Once()
		added, err := wrapper.Add(conflictingSeal)
		assert.NoError(t, err)
		assert.False(t, added)
		wrappedMempool.AssertExpectations(t)

		// mempool should be cleared
		wrappedMempool.On("Size").Return(uint(0)) // we asserted that Clear was called on wrappedMempool
		assert.Equal(t, uint(0), wrapper.Size())

		// additional seals should not be accepted anymore
		added, err = wrapper.Add(unittest.IncorporatedResultSeal.Fixture())
		assert.NoError(t, err)
		assert.False(t, added)
		assert.Equal(t, uint(0), wrapper.Size())
		wrappedMempool.AssertExpectations(t)
		execForkActor.AssertExpectations(t)
	})
}

// Test_ForkDetectionPersisted verifies that, when ExecForkSuppressor detects a fork, this information is
// persisted in the data base
func Test_ForkDetectionPersisted(t *testing.T) {
	unittest.RunWithTempDir(t, func(dir string) {
		db := unittest.BadgerDB(t, dir)
		defer db.Close()

		// initialize ExecForkSuppressor
		wrappedMempool := &poolmock.IncorporatedResultSeals{}
		wrappedMempool.On("RegisterEjectionCallbacks", mock.Anything).Return()
		execForkActor := &actormock.ExecForkActorMock{}
		wrapper, _ := NewExecStateForkSuppressor(execForkActor.OnExecFork, wrappedMempool, db, zerolog.New(os.Stderr))

		// add seal
		block := unittest.BlockFixture()
		sealA := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&block))))
		wrappedMempool.On("Add", sealA).Return(true, nil).Once()
		wrappedMempool.On("ByID", sealA.ID()).Return(sealA, true)
		_, _ = wrapper.Add(sealA)

		// add conflicting seal
		sealB := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(unittest.ExecutionResultFixture(unittest.WithBlock(&block))))
		execForkActor.On("OnExecFork", []*flow.IncorporatedResultSeal{sealB, sealA}).Return().Once()
		wrappedMempool.On("Clear").Return().Once()
		added, _ := wrapper.Add(sealB) // should be rejected because it is conflicting with sealA
		assert.False(t, added)
		wrappedMempool.AssertExpectations(t)
		execForkActor.AssertExpectations(t)

		// crash => re-initialization
		db.Close()
		db2 := unittest.BadgerDB(t, dir)
		wrappedMempool2 := &poolmock.IncorporatedResultSeals{}
		wrappedMempool2.On("RegisterEjectionCallbacks", mock.Anything).Return()
		execForkActor2 := &actormock.ExecForkActorMock{}
		execForkActor2.On("OnExecFork", mock.Anything).
			Run(func(args mock.Arguments) {
				conflictingSeals := args[0].([]*flow.IncorporatedResultSeal)
				assert.Equal(t, 2, len(conflictingSeals))
				assert.Equal(t, sealB.ID(), conflictingSeals[0].ID())
				assert.Equal(t, sealA.ID(), conflictingSeals[1].ID())
			}).Return().Once()
		wrapper2, _ := NewExecStateForkSuppressor(execForkActor2.OnExecFork, wrappedMempool2, db2, zerolog.New(os.Stderr))

		// add another (non-conflicting) seal to ExecForkSuppressor
		// fail test if seal is added to wrapped mempool
		wrappedMempool2.On("Add", mock.Anything).
			Run(func(args mock.Arguments) { assert.Fail(t, "seal was added to wrapped mempool") }).
			Return(true, nil).Maybe()
		added, _ = wrapper2.Add(unittest.IncorporatedResultSeal.Fixture())
		assert.False(t, added)
		wrappedMempool2.On("Size").Return(uint(0)) // we asserted that Clear was called on wrappedMempool
		assert.Equal(t, uint(0), wrapper2.Size())

		wrappedMempool2.AssertExpectations(t)
		execForkActor2.AssertExpectations(t)
	})
}

// Test_EjectorRemovesNewSeal covers the following edge case:
//   * upon adding a seal, the ejector of the wrapped mempool decides to eject the element which was just added
// We verify this by inspecting the internal data structure of ExecForkSuppressor
func Test_EjectorRemovesNewSeal(t *testing.T) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		wrappedMempool := &poolmock.IncorporatedResultSeals{}
		var ejectionCallback mempool.OnEjection
		wrappedMempool.On("RegisterEjectionCallbacks", mock.Anything).
			Run(func(args mock.Arguments) { ejectionCallback = args[0].(mempool.OnEjection) }).
			Return()
		execForkActor := &actormock.ExecForkActorMock{}
		wrapper, _ := NewExecStateForkSuppressor(execForkActor.OnExecFork, wrappedMempool, db, zerolog.New(os.Stderr))

		// as soon as a seal is added, the underlying mempool ejects it right away again
		seal := unittest.IncorporatedResultSeal.Fixture()
		wrappedMempool.On("Add", seal).
			Run(func(args mock.Arguments) { ejectionCallback(seal) }).
			Return(true, nil)
		wrappedMempool.On("ByID", seal.ID()).Return(nil, false)

		added, err := wrapper.Add(seal)
		require.NoError(t, err)
		assert.True(t, added)
		assert.Equal(t, 0, len(wrapper.sealsForBlock))
	})
}

// Test_AddRem_SmokeTest tests a real system of stdmap.IncorporatedResultSeals mempool
// which is wrapped in an ExecForkSuppressor.
// We add and remove lots of different seals.
func Test_AddRem_SmokeTest(t *testing.T) {
	onExecFork := func([]*flow.IncorporatedResultSeal) {
		assert.Fail(t, "no call to onExecFork expected ")
	}
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		wrappedMempool := stdmap.NewIncorporatedResultSeals(stdmap.WithLimit(3))
		wrapper, err := NewExecStateForkSuppressor(onExecFork, wrappedMempool, db, zerolog.New(os.Stderr))
		require.NoError(t, err)
		require.NotNil(t, wrapper)

		// add custom ejector to wrapped mempool to track ejected elements
		var ejected map[flow.Identifier]struct{}
		wrapper.RegisterEjectionCallbacks(
			func(entity flow.Entity) {
				ejected[entity.ID()] = struct{}{}
			})

		// Run 100 experiments of the following kind:
		//  * add 10 seals to mempool, which should eject 7 seals
		//  * test that ejected seals are not in mempool anymore
		//  * remove remaining seals
		for i := 100; i > 0; i-- {
			ejected = make(map[flow.Identifier]struct{})
			seals := unittest.IncorporatedResultSeal.Fixtures(10)
			for _, s := range seals {
				added, err := wrapper.Add(s)
				require.NoError(t, err)
				require.True(t, added)
			}

			require.Equal(t, 7, len(ejected))
			require.Equal(t, uint(3), wrappedMempool.Size())
			require.Equal(t, uint(3), wrapper.Size())
			for _, s := range seals {
				id := s.ID()
				if _, wasEjected := ejected[id]; wasEjected {
					_, found := wrapper.ByID(id)
					require.False(t, found)
				} else {
					_, found := wrapper.ByID(id)
					require.True(t, found)
					wrapper.Rem(id)

					_, found = wrapper.ByID(id)
					require.False(t, found)
					_, found = wrappedMempool.ByID(id)
					require.False(t, found)
				}
			}

			require.Equal(t, uint(0), wrappedMempool.Size())
			require.Equal(t, uint(0), wrapper.Size())
			require.Equal(t, 0, len(wrapper.sealsForBlock))
		}

	})
}

// WithExecStateForkSuppressor
//  1. constructs a mock (aka `wrappedMempool`) of an IncorporatedResultSeals mempool
//  2. wraps `wrappedMempool` in a ExecForkSuppressor
//  3. ensures that initializing the wrapper did not error
//  4. executes the `testLogic`
func WithExecStateForkSuppressor(t testing.TB, testLogic func(wrapper *ExecForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, execForkActor *actormock.ExecForkActorMock)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		wrappedMempool := &poolmock.IncorporatedResultSeals{}
		wrappedMempool.On("RegisterEjectionCallbacks", mock.Anything).Return()

		execForkActor := &actormock.ExecForkActorMock{}
		wrapper, err := NewExecStateForkSuppressor(execForkActor.OnExecFork, wrappedMempool, db, zerolog.New(os.Stderr))
		require.NoError(t, err)
		require.NotNil(t, wrapper)
		testLogic(wrapper, wrappedMempool, execForkActor)
	})
}
