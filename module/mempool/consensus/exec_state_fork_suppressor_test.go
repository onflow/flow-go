package consensus

import (
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/mempool"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	poolmock "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test_Construction verifies correctness of the initial size and limit values
func Test_Construction(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, ejector mempool.OnEjection) {
		wrappedMempool.On("Size").Return(uint(0)).Once()
		require.Equal(t, uint(0), wrapper.Size())
		wrappedMempool.On("Limit").Return(uint(0)).Once()
		require.Equal(t, uint(0), wrapper.Limit())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_Size checks that ExecStateForkSuppressor is reporting the size of the wrapped mempool
func Test_Size(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, ejector mempool.OnEjection) {
		wrappedMempool.On("Size").Return(uint(139)).Once()
		require.Equal(t, uint(139), wrapper.Size())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_Limit checks that ExecStateForkSuppressor is reporting the capacity limit of the wrapped mempool
func Test_Limit(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, ejector mempool.OnEjection) {
		wrappedMempool.On("Limit").Return(uint(227)).Once()
		require.Equal(t, uint(227), wrapper.Limit())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_Clear checks that, when clearing the ExecStateForkSuppressor:
//   * the wrapper also clears the wrapped mempool;
//   * the reported mempool size, _after_ clearing should be zero
func Test_Clear(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, ejector mempool.OnEjection) {
		wrappedMempool.On("Clear").Return().Once()

		wrapper.Clear()
		wrappedMempool.On("Size").Return(uint(0))
		require.Equal(t, uint(0), wrapper.Size())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_All checks that ExecStateForkSuppressor.All() is returning the elements of the wrapped mempool
func Test_All(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, ejector mempool.OnEjection) {
		expectedSeals := unittest.IncorporatedResultSeal.Fixtures(7)
		wrappedMempool.On("All").Return(expectedSeals)
		retrievedSeals := wrapper.All()
		require.Equal(t, len(expectedSeals), len(retrievedSeals))
		for i := 0; i < len(expectedSeals); i++ {
			require.Equal(t, expectedSeals[i].ID(), retrievedSeals[i].ID())
		}
	})
}

// Test_Rem checks that ExecStateForkSuppressor.Rem()
//   * delegates the call to the underlying mempool
func Test_Rem(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, ejector mempool.OnEjection) {
		// element is in wrapped mempool: Rem should be called
		seal := unittest.IncorporatedResultSeal.Fixture()
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

// Test_Add adds IncorporatedResultSeals for
//   * 2 different blocks
//   * for each block, we generate one specific result,
//     for which we add 4 IncorporatedResultSeals
//      o IncorporatedResultSeal (1):
//        incorporated in block B1
//      o IncorporatedResultSeal (2):
//        incorporated in block B2
//      o IncorporatedResultSeal (3):
//        incorporated in block B2, but with a different seal than IncorporatedResult (1)
//      o IncorporatedResultSeal (4):
//        identical duplicate of IncorporatedResult (a)
//        which should be automatically de-duplicated
func Test_Add(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, ejector mempool.OnEjection) {
		for _, block := range unittest.BlockFixtures(2) {
			result := unittest.ExecutionResultFixture(unittest.WithBlock(block))

			// IncorporatedResultSeal (1):
			irSeal1 := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(result))
			wrappedMempool.On("Add", irSeal1).Return(true, nil).Once()
			added, err := wrappedMempool.Add(irSeal1)
			assert.NoError(t, err)
			assert.True(t, added)

			// IncorporatedResultSeal (2):
			irSeal2 := unittest.IncorporatedResultSeal.Fixture(unittest.IncorporatedResultSeal.WithResult(result))
			wrappedMempool.On("Add", irSeal2).Return(true, nil).Once()
			added, err = wrappedMempool.Add(irSeal2)
			assert.NoError(t, err)
			assert.True(t, added)

			// IncorporatedResultSeal (3):
			irSeal3 := unittest.IncorporatedResultSeal.Fixture(
				unittest.IncorporatedResultSeal.WithResult(result),
				unittest.IncorporatedResultSeal.WithIncorporatedBlockID(irSeal1.IncorporatedResult.IncorporatedBlockID),
			)
			assert.False(t, irSeal1.ID() == irSeal3.ID())
			wrappedMempool.On("Add", irSeal3).Return(true, nil).Once()
			added, err = wrappedMempool.Add(irSeal3)
			assert.NoError(t, err)
			assert.True(t, added)

			// IncorporatedResultSeal (4):
			irSeal4 := &flow.IncorporatedResultSeal{
				IncorporatedResult: irSeal1.IncorporatedResult,
				Seal:               irSeal1.Seal,
			}
			assert.True(t, irSeal1.ID() == irSeal4.ID())
			wrappedMempool.On("Add", irSeal4).Return(true, nil).Once()
			assert.False(t, irSeal1 == irSeal4)
			added, err = wrappedMempool.Add(irSeal4)
			assert.NoError(t, err)
			assert.True(t, added)

		}
	})
}

func Test_RejectInvalidSeals(t *testing.T) {
	// missing start state
	// missing end state

	assert.Fail(t, "test incomplete")
}

func Test_ConflictingResults(t *testing.T) {
	// missing start state
	// missing end state

	assert.Fail(t, "test incomplete")
}

// Test_Rem checks that ExecStateForkSuppressor.Rem()
//   * delegates the call to the underlying mempool
func Test_AddRem_SmokeTest(t *testing.T) {
	//WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *mempool.IncorporatedResultSeals) {
	//	rand.Seed(time.Now().UnixNano())
	//
	//	// smoke test
	//
	//	rand.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
	//
	//
	//	someSealID := unittest.IdentifierFixture()
	//
	//	for
	//
	//		wrappedMempool.On("Rem", someSealID).Return(true).Once()
	//	removed := wrapper.Rem(someSealID)
	//	require.Equal(t, removed, len(retrievedSeals))
	//})
}

// WithExecStateForkSuppressor
//  1. constructs a mock (aka `wrappedMempool`) of an IncorporatedResultSeals mempool
//  2. wrapps `wrappedMempool` in a ExecStateForkSuppressor
//  3. ensures that initializing the wrapper did not error
//  4. executes the `testLogic`
func WithExecStateForkSuppressor(t testing.TB, testLogic func(wrapper *ExecStateForkSuppressor, wrappedMempool *poolmock.IncorporatedResultSeals, ejector mempool.OnEjection)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		wrappedMempool := &poolmock.IncorporatedResultSeals{}
		var ejector mempool.OnEjection
		wrappedMempool.On("RegisterEjectionCallbacks", mock.Anything).
			Run(func(args mock.Arguments) {
				ejector = args[0].(mempool.OnEjection)
			}).Return()

		wrapper, err := NewExecStateForkSuppressor(wrappedMempool, db, zerolog.New(os.Stderr))
		require.NoError(t, err)
		require.NotNil(t, wrapper)
		testLogic(wrapper, wrappedMempool, ejector)
	})
}

//func withID(id flow.Identifier) interface{} {
//	return mock.MatchedBy(func(id flow.Identifier) bool { return expectedBlockID == block.BlockID })
//}

func shallowCopy(slice []*flow.IncorporatedResultSeal) []*flow.IncorporatedResultSeal {
	sliceCopy := make([]*flow.IncorporatedResultSeal, len(slice))
	copy(sliceCopy, slice)
	return sliceCopy
}
