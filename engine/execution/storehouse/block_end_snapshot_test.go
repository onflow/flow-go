package storehouse_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

	executionMock "github.com/onflow/flow-go/engine/execution/mock"
	"github.com/onflow/flow-go/engine/execution/storehouse"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestBlockEndSnapshot(t *testing.T) {
	t.Run("Get register", func(t *testing.T) {
		header := unittest.BlockHeaderFixture()

		// create mock for storage
		store := &executionMock.RegisterStore{}
		reg := unittest.MakeOwnerReg("key", "value")
		store.On("GetRegister", header.Height, header.ID(), reg.Key).Return(reg.Value, nil).Once()
		snapshot := storehouse.NewBlockEndStateSnapshot(store, header.ID(), header.Height)

		// test get from storage
		value, err := snapshot.Get(reg.Key)
		require.NoError(t, err)
		require.Equal(t, reg.Value, value)

		// test get from cache
		value, err = snapshot.Get(reg.Key)
		require.NoError(t, err)
		require.Equal(t, reg.Value, value)

		// test get non existing register
		unknownReg := unittest.MakeOwnerReg("unknown", "unknown")
		store.On("GetRegister", header.Height, header.ID(), unknownReg.Key).
			Return(nil, fmt.Errorf("fail: %w", storage.ErrNotFound)).Once()

		value, err = snapshot.Get(unknownReg.Key)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound))
		require.Nil(t, value)

		// test get non existing register from cache
		_, err = snapshot.Get(unknownReg.Key)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrNotFound))

		// test getting storage.ErrHeightNotIndexed error
		heightNotIndexed := unittest.MakeOwnerReg("height not index", "height not index")
		store.On("GetRegister", header.Height, header.ID(), heightNotIndexed.Key).
			Return(nil, fmt.Errorf("fail: %w", storage.ErrHeightNotIndexed)).
			Twice() // to verify the result is not cached

		// verify getting the correct error
		_, err = snapshot.Get(heightNotIndexed.Key)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrHeightNotIndexed))

		// verify result is not cached
		_, err = snapshot.Get(heightNotIndexed.Key)
		require.Error(t, err)
		require.True(t, errors.Is(err, storage.ErrHeightNotIndexed))

		// test getting storage.ErrNotExecuted error
		heightNotExecuted := unittest.MakeOwnerReg("height not executed", "height not executed")
		counter := atomic.NewInt32(0)
		mocked := store.On("GetRegister", header.Height, header.ID(), heightNotExecuted.Key)
		mocked.Times(2) // the first time hit error, the second get value, the third time is cached
		mocked.RunFn = func(args mock.Arguments) {
			counter.Inc()
			// the first call should return error
			if counter.Load() == 1 {
				mocked.ReturnArguments = mock.Arguments{nil, fmt.Errorf("fail: %w", storehouse.ErrNotExecuted)}
				return
			}
			// the second call, it returns value
			mocked.ReturnArguments = mock.Arguments{heightNotExecuted.Value, nil}
		}

		// first time should return error
		_, err = snapshot.Get(heightNotExecuted.Key)
		require.Error(t, err)
		require.True(t, errors.Is(err, storehouse.ErrNotExecuted))

		// second time should return value
		value, err = snapshot.Get(heightNotExecuted.Key)
		require.NoError(t, err)
		require.Equal(t, heightNotExecuted.Value, value)

		// third time should be cached
		value, err = snapshot.Get(heightNotExecuted.Key)
		require.NoError(t, err)
		require.Equal(t, heightNotExecuted.Value, value)

		store.AssertExpectations(t)
	})

}
