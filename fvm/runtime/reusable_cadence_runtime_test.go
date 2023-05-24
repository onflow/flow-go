package runtime

import (
	"testing"

	"github.com/onflow/cadence/runtime"
	"github.com/stretchr/testify/require"
)

func TestReusableCadenceRuntimePoolUnbuffered(t *testing.T) {
	pool := NewReusableCadenceRuntimePool(0, runtime.Config{})
	require.Nil(t, pool.pool)

	entry := pool.Borrow(nil)
	require.NotNil(t, entry)

	pool.Return(entry)

	entry2 := pool.Borrow(nil)
	require.NotNil(t, entry2)

	require.NotSame(t, entry, entry2)
}

func TestReusableCadenceRuntimePoolBuffered(t *testing.T) {
	pool := NewReusableCadenceRuntimePool(100, runtime.Config{})
	require.NotNil(t, pool.pool)

	select {
	case <-pool.pool:
		require.True(t, false)
	default:
	}

	entry := pool.Borrow(nil)
	require.NotNil(t, entry)

	select {
	case <-pool.pool:
		require.True(t, false)
	default:
	}

	pool.Return(entry)

	entry2 := pool.Borrow(nil)
	require.NotNil(t, entry2)

	require.Same(t, entry, entry2)
}

func TestReusableCadenceRuntimePoolSharing(t *testing.T) {
	pool := NewReusableCadenceRuntimePool(100, runtime.Config{})
	require.NotNil(t, pool.pool)

	select {
	case <-pool.pool:
		require.True(t, false)
	default:
	}

	var otherPool ReusableCadenceRuntimePool = pool

	entry := otherPool.Borrow(nil)
	require.NotNil(t, entry)

	select {
	case <-pool.pool:
		require.True(t, false)
	default:
	}

	otherPool.Return(entry)

	entry2 := pool.Borrow(nil)
	require.NotNil(t, entry2)

	require.Same(t, entry, entry2)
}
