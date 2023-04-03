package atomic

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/require"
	"go.uber.org/atomic"

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

func BenchmarkValue(b *testing.B) {
	val := NewValue[*flow.Header]()
	for i := 0; i < b.N; i++ {
		val.Set(&flow.Header{Height: uint64(i)})
		x, _ := val.Get()
		if x.Height != uint64(i) {
			b.Fail()
		}
	}
}

// Compare implementation to raw atomic.UnsafePointer.
// Generics and supporting zero values incurs cost of ~30ns/op (~30%)
func BenchmarkNoGenerics(b *testing.B) {
	val := atomic.NewUnsafePointer(unsafe.Pointer(nil))
	for i := 0; i < b.N; i++ {
		val.Store((unsafe.Pointer)(&flow.Header{Height: uint64(i)}))
		x := (*flow.Header)(val.Load())
		if x.Height != uint64(i) {
			b.Fail()
		}
	}
}
