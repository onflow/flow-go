package fvm

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
)

func TestReusableCadanceRuntimePoolUnbuffered(t *testing.T) {
	pool := NewReusableCadenceRuntimePool(0)
	require.Nil(t, pool.pool)

	entry := pool.Borrow(nil)
	require.NotNil(t, entry)

	pool.Return(entry)

	entry2 := pool.Borrow(nil)
	require.NotNil(t, entry2)

	require.NotSame(t, entry, entry2)
}

func TestReusableCadanceRuntimePoolBuffered(t *testing.T) {
	pool := NewReusableCadenceRuntimePool(100)
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

func TestReusableCadanceRuntimePoolSharing(t *testing.T) {
	pool := NewReusableCadenceRuntimePool(100)
	require.NotNil(t, pool.pool)

	select {
	case <-pool.pool:
		require.True(t, false)
	default:
	}

	ctx := NewContext(zerolog.Logger{}, WithReusableCadenceRuntimePool(pool))

	entry := ctx.ReusableCadenceRuntimePool.Borrow(nil)
	require.NotNil(t, entry)

	select {
	case <-pool.pool:
		require.True(t, false)
	default:
	}

	ctx.ReusableCadenceRuntimePool.Return(entry)

	entry2 := pool.Borrow(nil)
	require.NotNil(t, entry2)

	require.Same(t, entry, entry2)
}
