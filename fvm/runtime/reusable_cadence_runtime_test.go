package runtime

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/model/flow"
)

func TestReusableCadenceRuntimePoolUnbuffered(t *testing.T) {
	pool := NewReusableCadenceRuntimePool(0, flow.Mainnet.Chain(), runtime.Config{})
	require.Nil(t, pool.pool)

	entry := pool.Borrow(nil, environment.CadenceTransactionRuntime).(reusableCadenceTransactionRuntime)
	require.NotNil(t, entry)

	pool.Return(entry)

	entry2 := pool.Borrow(nil, environment.CadenceTransactionRuntime).(reusableCadenceTransactionRuntime)
	require.NotNil(t, entry2)

	require.NotSame(t, entry.reusableCadenceRuntime, entry2.reusableCadenceRuntime)
}

func TestReusableCadenceRuntimePoolBuffered(t *testing.T) {
	pool := NewReusableCadenceRuntimePool(100, flow.Mainnet.Chain(), runtime.Config{})
	require.NotNil(t, pool.pool)

	select {
	case <-pool.pool:
		require.True(t, false)
	default:
	}

	entry := pool.Borrow(nil, environment.CadenceTransactionRuntime).(reusableCadenceTransactionRuntime)
	require.NotNil(t, entry)

	select {
	case <-pool.pool:
		require.True(t, false)
	default:
	}

	pool.Return(entry)

	entry2 := pool.Borrow(nil, environment.CadenceTransactionRuntime).(reusableCadenceTransactionRuntime)
	require.NotNil(t, entry2)

	require.Same(t, entry.reusableCadenceRuntime, entry2.reusableCadenceRuntime)
}

func TestReusableCadenceRuntimePoolSharing(t *testing.T) {
	pool := NewReusableCadenceRuntimePool(100, flow.Mainnet.Chain(), runtime.Config{})
	require.NotNil(t, pool.pool)

	select {
	case <-pool.pool:
		require.True(t, false)
	default:
	}

	var otherPool = pool

	entry := otherPool.Borrow(nil, environment.CadenceTransactionRuntime).(reusableCadenceTransactionRuntime)
	require.NotNil(t, entry)

	select {
	case <-pool.pool:
		require.True(t, false)
	default:
	}

	otherPool.Return(entry)

	entry2 := pool.Borrow(nil, environment.CadenceTransactionRuntime).(reusableCadenceTransactionRuntime)
	require.NotNil(t, entry2)

	require.Same(t, entry.reusableCadenceRuntime, entry2.reusableCadenceRuntime)
}
