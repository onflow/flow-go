package atomic

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestValue_Basic tests storing a basic builtin type (int)
func TestValue_Basic(t *testing.T) {
	val := NewValue[int]()
	// should return zero value and ok=false initially
	x, ok := val.Get()
	require.False(t, ok)
	require.Equal(t, 0, x)

	// should return stored value
	val.Set(1)
	x, ok = val.Get()
	require.True(t, ok)
	require.Equal(t, 1, x)

	// should be able to store and retrieve zero value
	val.Set(0)
	x, ok = val.Get()
	require.True(t, ok)
	require.Equal(t, 0, x)
}

// TestValue_Struct tests storing a struct type.
func TestValue_Struct(t *testing.T) {
	val := NewValue[flow.Header]()
	// should return zero value and ok=false initially
	x, ok := val.Get()
	require.False(t, ok)
	require.Zero(t, x)

	// should return stored value
	header := unittest.BlockHeaderFixture()
	val.Set(*header)
	x, ok = val.Get()
	require.True(t, ok)
	require.Equal(t, *header, x)

	// should be able to store and retrieve zero value
	val.Set(flow.Header{})
	x, ok = val.Get()
	require.True(t, ok)
	require.Equal(t, flow.Header{}, x)
}

// TestValue_Ptr tests storing a pointer type.
func TestValue_Ptr(t *testing.T) {
	val := NewValue[*flow.Header]()
	// should return zero value and ok=false initially
	x, ok := val.Get()
	require.False(t, ok)
	require.Nil(t, x)

	// should return stored value
	header := unittest.BlockHeaderFixture()
	val.Set(header)
	x, ok = val.Get()
	require.True(t, ok)
	require.Equal(t, header, x)

	// should be able to store and retrieve zero value
	val.Set(nil)
	x, ok = val.Get()
	require.True(t, ok)
	require.Nil(t, x)
}
