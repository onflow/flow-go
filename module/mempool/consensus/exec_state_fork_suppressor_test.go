package consensus

import (
	"os"
	"testing"

	"github.com/dgraph-io/badger/v2"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	mempool "github.com/onflow/flow-go/module/mempool/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

// Test_Construction verifies correctness of the initial size and limit values
func Test_Construction(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *mempool.IncorporatedResultSeals) {
		wrappedMempool.On("Size").Return(uint(0)).Once()
		require.Equal(t, uint(0), wrapper.Size())
		wrappedMempool.On("Limit").Return(uint(0)).Once()
		require.Equal(t, uint(0), wrapper.Limit())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_Size checks that ExecStateForkSuppressor is reporting the size of the wrapped mempool
func Test_Size(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *mempool.IncorporatedResultSeals) {
		wrappedMempool.On("Size").Return(uint(139)).Once()
		require.Equal(t, uint(139), wrapper.Size())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_Limit checks that ExecStateForkSuppressor is reporting the capacity limit of the wrapped mempool
func Test_Limit(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *mempool.IncorporatedResultSeals) {
		wrappedMempool.On("Limit").Return(uint(227)).Once()
		require.Equal(t, uint(227), wrapper.Limit())
		wrappedMempool.AssertExpectations(t)
	})
}

// Test_Add
// TODO add goDoc
func Test_Add(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *mempool.IncorporatedResultSeals) {
		irSeals := unittest.IncorporatedResultSeal.Fixtures(7)
		wrappedMempool.On("Size").Return(uint(0)).Once()
		// TODO FIX ME
		_, _ = wrappedMempool.Add(irSeals[0])

	})
}

// Test_Clear checks that, when clearing the ExecStateForkSuppressor:
//   * the wrapper also clears the wrapped mempool;
//   * the reported mempool size, _after_ clearing should be zero
func Test_Clear(t *testing.T) {
	WithExecStateForkSuppressor(t, func(wrapper *ExecStateForkSuppressor, wrappedMempool *mempool.IncorporatedResultSeals) {
		wrappedMempool.On("Size").Return(uint(139)).Once()
		wrappedMempool.On("Clear").Return().Once()

		wrapper.Clear()
		wrappedMempool.On("Limit").Return(uint(0)).Once()
		require.Equal(t, uint(227), wrapper.Size())
		wrappedMempool.AssertExpectations(t)
	})
}

// WithExecStateForkSuppressor
//  1. constructs a mock (aka `wrappedMempool`) of an IncorporatedResultSeals mempool
//  2. wrapps `wrappedMempool` in a ExecStateForkSuppressor
//  3. ensures that initializing the wrapper did not error
//  4. executes the `testLogic`
func WithExecStateForkSuppressor(t testing.TB, testLogic func(wrapper *ExecStateForkSuppressor, wrappedMempool *mempool.IncorporatedResultSeals)) {
	unittest.RunWithBadgerDB(t, func(db *badger.DB) {
		wrappedMempool := &mempool.IncorporatedResultSeals{}
		wrapper, err := NewExecStateForkSuppressor(wrappedMempool, db, zerolog.New(os.Stderr))
		require.NoError(t, err)
		require.NotNil(t, wrapper)
		testLogic(wrapper, wrappedMempool)
	})
}
