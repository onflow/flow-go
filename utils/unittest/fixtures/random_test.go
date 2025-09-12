package fixtures

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRandomGenerator(t *testing.T) {
	// Test with explicit seed for deterministic results
	rng := rand.New(rand.NewSource(12345))
	randomGen := NewRandomGenerator(rng)

	// Test RandomBytes
	bytes := randomGen.RandomBytes(32)
	require.Len(t, bytes, 32)
	assert.NotEqual(t, make([]byte, 32), bytes) // Should not be all zeros

	// Test RandomString
	str := randomGen.RandomString(16)
	require.Len(t, str, 16)
	assert.NotEmpty(t, str)

	// Test Uintn
	uintVal := randomGen.Uintn(100)
	assert.GreaterOrEqual(t, uintVal, uint(0))
	assert.Less(t, uintVal, uint(100))

	// Test Uint32n
	uint32Val := randomGen.Uint32n(100)
	assert.GreaterOrEqual(t, uint32Val, uint32(0))
	assert.Less(t, uint32Val, uint32(100))

	// Test Uint64n
	uint64Val := randomGen.Uint64n(100)
	assert.GreaterOrEqual(t, uint64Val, uint64(0))
	assert.Less(t, uint64Val, uint64(100))

	// Test IntInRange (positive ranges only)
	intInRange := randomGen.IntInRange(1, 50)
	assert.GreaterOrEqual(t, intInRange, 1)
	assert.LessOrEqual(t, intInRange, 50)

	// Test Int32InRange (positive ranges only)
	int32InRange := randomGen.Int32InRange(1, 25)
	assert.GreaterOrEqual(t, int32InRange, int32(1))
	assert.LessOrEqual(t, int32InRange, int32(25))

	// Test Int64InRange (positive ranges only)
	int64InRange := randomGen.Int64InRange(1, 100)
	assert.GreaterOrEqual(t, int64InRange, int64(1))
	assert.LessOrEqual(t, int64InRange, int64(100))

	// Test UintInRange
	uintInRange := randomGen.UintInRange(10, 90)
	assert.GreaterOrEqual(t, uintInRange, uint(10))
	assert.LessOrEqual(t, uintInRange, uint(90))

	// Test Uint32InRange
	uint32InRange := randomGen.Uint32InRange(5, 95)
	assert.GreaterOrEqual(t, uint32InRange, uint32(5))
	assert.LessOrEqual(t, uint32InRange, uint32(95))

	// Test Uint64InRange
	uint64InRange := randomGen.Uint64InRange(1, 1000)
	assert.GreaterOrEqual(t, uint64InRange, uint64(1))
	assert.LessOrEqual(t, uint64InRange, uint64(1000))
}

func TestInclusiveRange(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))
	randomGen := NewRandomGenerator(rng)

	// Test with different types (positive ranges only)
	tests := []struct {
		name string
		min  any
		max  any
	}{
		{"int", 1, 100},
		{"int32", int32(1), int32(100)},
		{"int64", int64(1), int64(100)},
		{"uint", uint(1), uint(100)},
		{"uint32", uint32(1), uint32(100)},
		{"uint64", uint64(1), uint64(100)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			switch min := tt.min.(type) {
			case int:
				max := tt.max.(int)
				result := InclusiveRange(randomGen, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			case int32:
				max := tt.max.(int32)
				result := InclusiveRange(randomGen, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			case int64:
				max := tt.max.(int64)
				result := InclusiveRange(randomGen, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			case uint:
				max := tt.max.(uint)
				result := InclusiveRange(randomGen, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			case uint32:
				max := tt.max.(uint32)
				result := InclusiveRange(randomGen, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			case uint64:
				max := tt.max.(uint64)
				result := InclusiveRange(randomGen, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			}
		})
	}
}

func TestRandomElement(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))
	randomGen := NewRandomGenerator(rng)

	// Test with string slice
	stringSlice := []string{"apple", "banana", "cherry", "date"}
	element := RandomElement(randomGen, stringSlice)
	assert.Contains(t, stringSlice, element)

	// Test with int slice
	intSlice := []int{1, 2, 3, 4, 5}
	intElement := RandomElement(randomGen, intSlice)
	assert.Contains(t, intSlice, intElement)

	// Test with empty slice
	emptySlice := []string{}
	emptyElement := RandomElement(randomGen, emptySlice)
	assert.Empty(t, emptyElement)

	// Test with single element slice
	singleSlice := []string{"only"}
	singleElement := RandomElement(randomGen, singleSlice)
	assert.Equal(t, "only", singleElement)
}

func TestRandomGeneratorDeterministic(t *testing.T) {
	// Test that generators produce same results with same seed
	rng1 := rand.New(rand.NewSource(42))
	rng2 := rand.New(rand.NewSource(42))
	randomGen1 := NewRandomGenerator(rng1)
	randomGen2 := NewRandomGenerator(rng2)

	const max = 100_000_000

	// Test all methods produce same results
	assert.Equal(t, randomGen1.RandomBytes(32), randomGen2.RandomBytes(32))
	assert.Equal(t, randomGen1.RandomString(32), randomGen2.RandomString(32))
	assert.Equal(t, randomGen1.Uintn(max), randomGen2.Uintn(max))
	assert.Equal(t, randomGen1.Uint32n(max), randomGen2.Uint32n(max))
	assert.Equal(t, randomGen1.Uint64n(max), randomGen2.Uint64n(max))
	assert.Equal(t, randomGen1.IntInRange(1, max), randomGen2.IntInRange(1, max))
	assert.Equal(t, randomGen1.Int32InRange(1, max), randomGen2.Int32InRange(1, max))
	assert.Equal(t, randomGen1.Int64InRange(1, max), randomGen2.Int64InRange(1, max))
	assert.Equal(t, randomGen1.UintInRange(1, max), randomGen2.UintInRange(1, max))
	assert.Equal(t, randomGen1.Uint32InRange(1, max), randomGen2.Uint32InRange(1, max))
	assert.Equal(t, randomGen1.Uint64InRange(1, max), randomGen2.Uint64InRange(1, max))

	// Test InclusiveRange
	assert.Equal(t, InclusiveRange(randomGen1, 1, max), InclusiveRange(randomGen2, 1, max))
	assert.Equal(t, InclusiveRange(randomGen1, uint32(1), max), InclusiveRange(randomGen2, uint32(1), max))

	// Test RandomElement
	slice := []string{"apple", "banana", "cherry"}
	assert.Equal(t, RandomElement(randomGen1, slice), RandomElement(randomGen2, slice))
}

func TestRandomGeneratorDifferentSeeds(t *testing.T) {
	// Test that generators produce different results with different seeds
	rng1 := rand.New(rand.NewSource(42))
	rng2 := rand.New(rand.NewSource(123))
	randomGen1 := NewRandomGenerator(rng1)
	randomGen2 := NewRandomGenerator(rng2)

	// Test that results are different (very high probability)
	assert.NotEqual(t, randomGen1.RandomBytes(32), randomGen2.RandomBytes(32))
	assert.NotEqual(t, randomGen1.RandomString(32), randomGen2.RandomString(32))
	assert.NotEqual(t, randomGen1.Uintn(100_000_000), randomGen2.Uintn(100_000_000))
	assert.NotEqual(t, randomGen1.Uint32n(100_000_000), randomGen2.Uint32n(100_000_000))
	assert.NotEqual(t, randomGen1.Uint64n(100_000_000), randomGen2.Uint64n(100_000_000))
}

func TestRandomGeneratorEdgeCases(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))
	randomGen := NewRandomGenerator(rng)

	// Test with single value range (positive only, as per current implementation)
	singleInt := randomGen.IntInRange(5, 5)
	assert.Equal(t, 5, singleInt)

	singleUint := randomGen.UintInRange(10, 10)
	assert.Equal(t, uint(10), singleUint)

	// Test with positive ranges only (as per current implementation constraints)
	positiveRange := randomGen.IntInRange(1, 10)
	assert.GreaterOrEqual(t, positiveRange, 1)
	assert.LessOrEqual(t, positiveRange, 10)

	// Test with positive range for int32
	positiveRange32 := randomGen.Int32InRange(5, 15)
	assert.GreaterOrEqual(t, positiveRange32, int32(5))
	assert.LessOrEqual(t, positiveRange32, int32(15))

	// Test with positive range for int64
	positiveRange64 := randomGen.Int64InRange(10, 50)
	assert.GreaterOrEqual(t, positiveRange64, int64(10))
	assert.LessOrEqual(t, positiveRange64, int64(50))

	// Test with positive range for unsigned types
	positiveUintRange := randomGen.UintInRange(5, 25)
	assert.GreaterOrEqual(t, positiveUintRange, uint(5))
	assert.LessOrEqual(t, positiveUintRange, uint(25))
}
