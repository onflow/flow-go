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
	random := NewRandomGenerator(rng)

	// Test RandomBytes
	bytes := random.RandomBytes(32)
	require.Len(t, bytes, 32)
	assert.NotEqual(t, make([]byte, 32), bytes) // Should not be all zeros

	// Test RandomString
	str := random.RandomString(16)
	require.Len(t, str, 16)
	assert.NotEmpty(t, str)

	// Test Uintn
	uintVal := random.Uintn(100)
	assert.GreaterOrEqual(t, uintVal, uint(0))
	assert.Less(t, uintVal, uint(100))

	// Test Uint32n
	uint32Val := random.Uint32n(100)
	assert.GreaterOrEqual(t, uint32Val, uint32(0))
	assert.Less(t, uint32Val, uint32(100))

	// Test Uint64n
	uint64Val := random.Uint64n(100)
	assert.GreaterOrEqual(t, uint64Val, uint64(0))
	assert.Less(t, uint64Val, uint64(100))

	// Test IntInRange (positive ranges only)
	intInRange := random.IntInRange(1, 50)
	assert.GreaterOrEqual(t, intInRange, 1)
	assert.LessOrEqual(t, intInRange, 50)

	// Test Int32InRange (positive ranges only)
	int32InRange := random.Int32InRange(1, 25)
	assert.GreaterOrEqual(t, int32InRange, int32(1))
	assert.LessOrEqual(t, int32InRange, int32(25))

	// Test Int64InRange (positive ranges only)
	int64InRange := random.Int64InRange(1, 100)
	assert.GreaterOrEqual(t, int64InRange, int64(1))
	assert.LessOrEqual(t, int64InRange, int64(100))

	// Test UintInRange
	uintInRange := random.UintInRange(10, 90)
	assert.GreaterOrEqual(t, uintInRange, uint(10))
	assert.LessOrEqual(t, uintInRange, uint(90))

	// Test Uint32InRange
	uint32InRange := random.Uint32InRange(5, 95)
	assert.GreaterOrEqual(t, uint32InRange, uint32(5))
	assert.LessOrEqual(t, uint32InRange, uint32(95))

	// Test Uint64InRange
	uint64InRange := random.Uint64InRange(1, 1000)
	assert.GreaterOrEqual(t, uint64InRange, uint64(1))
	assert.LessOrEqual(t, uint64InRange, uint64(1000))
}

func TestInclusiveRange(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))
	random := NewRandomGenerator(rng)

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
				result := InclusiveRange(random, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			case int32:
				max := tt.max.(int32)
				result := InclusiveRange(random, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			case int64:
				max := tt.max.(int64)
				result := InclusiveRange(random, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			case uint:
				max := tt.max.(uint)
				result := InclusiveRange(random, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			case uint32:
				max := tt.max.(uint32)
				result := InclusiveRange(random, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			case uint64:
				max := tt.max.(uint64)
				result := InclusiveRange(random, min, max)
				assert.GreaterOrEqual(t, result, min)
				assert.LessOrEqual(t, result, max)
			}
		})
	}
}

func TestRandomElement(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))
	random := NewRandomGenerator(rng)

	// Test with string slice
	stringSlice := []string{"apple", "banana", "cherry", "date"}
	element := RandomElement(random, stringSlice)
	assert.Contains(t, stringSlice, element)

	// Test with int slice
	intSlice := []int{1, 2, 3, 4, 5}
	intElement := RandomElement(random, intSlice)
	assert.Contains(t, intSlice, intElement)

	// Test with empty slice
	emptySlice := []string{}
	emptyElement := RandomElement(random, emptySlice)
	assert.Empty(t, emptyElement)

	// Test with single element slice
	singleSlice := []string{"only"}
	singleElement := RandomElement(random, singleSlice)
	assert.Equal(t, "only", singleElement)
}

func TestRandomGeneratorDeterministic(t *testing.T) {
	// Test that generators produce same results with same seed
	rng1 := rand.New(rand.NewSource(42))
	rng2 := rand.New(rand.NewSource(42))
	random1 := NewRandomGenerator(rng1)
	random2 := NewRandomGenerator(rng2)

	const max = 100_000_000

	// Test all methods produce same results
	assert.Equal(t, random1.RandomBytes(32), random2.RandomBytes(32))
	assert.Equal(t, random1.RandomString(32), random2.RandomString(32))
	assert.Equal(t, random1.Uintn(max), random2.Uintn(max))
	assert.Equal(t, random1.Uint32n(max), random2.Uint32n(max))
	assert.Equal(t, random1.Uint64n(max), random2.Uint64n(max))
	assert.Equal(t, random1.IntInRange(1, max), random2.IntInRange(1, max))
	assert.Equal(t, random1.Int32InRange(1, max), random2.Int32InRange(1, max))
	assert.Equal(t, random1.Int64InRange(1, max), random2.Int64InRange(1, max))
	assert.Equal(t, random1.UintInRange(1, max), random2.UintInRange(1, max))
	assert.Equal(t, random1.Uint32InRange(1, max), random2.Uint32InRange(1, max))
	assert.Equal(t, random1.Uint64InRange(1, max), random2.Uint64InRange(1, max))

	// Test InclusiveRange
	assert.Equal(t, InclusiveRange(random1, 1, max), InclusiveRange(random2, 1, max))
	assert.Equal(t, InclusiveRange(random1, uint32(1), max), InclusiveRange(random2, uint32(1), max))

	// Test RandomElement
	slice := []string{"apple", "banana", "cherry"}
	assert.Equal(t, RandomElement(random1, slice), RandomElement(random2, slice))
}

func TestRandomGeneratorDifferentSeeds(t *testing.T) {
	// Test that generators produce different results with different seeds
	rng1 := rand.New(rand.NewSource(42))
	rng2 := rand.New(rand.NewSource(123))
	random1 := NewRandomGenerator(rng1)
	random2 := NewRandomGenerator(rng2)

	// Test that results are different (very high probability)
	assert.NotEqual(t, random1.RandomBytes(32), random2.RandomBytes(32))
	assert.NotEqual(t, random1.RandomString(32), random2.RandomString(32))
	assert.NotEqual(t, random1.Uintn(100_000_000), random2.Uintn(100_000_000))
	assert.NotEqual(t, random1.Uint32n(100_000_000), random2.Uint32n(100_000_000))
	assert.NotEqual(t, random1.Uint64n(100_000_000), random2.Uint64n(100_000_000))
}

func TestRandomGeneratorEdgeCases(t *testing.T) {
	rng := rand.New(rand.NewSource(12345))
	random := NewRandomGenerator(rng)

	// Test with single value range (positive only, as per current implementation)
	singleInt := random.IntInRange(5, 5)
	assert.Equal(t, 5, singleInt)

	singleUint := random.UintInRange(10, 10)
	assert.Equal(t, uint(10), singleUint)

	// Test with positive ranges only (as per current implementation constraints)
	positiveRange := random.IntInRange(1, 10)
	assert.GreaterOrEqual(t, positiveRange, 1)
	assert.LessOrEqual(t, positiveRange, 10)

	// Test with positive range for int32
	positiveRange32 := random.Int32InRange(5, 15)
	assert.GreaterOrEqual(t, positiveRange32, int32(5))
	assert.LessOrEqual(t, positiveRange32, int32(15))

	// Test with positive range for int64
	positiveRange64 := random.Int64InRange(10, 50)
	assert.GreaterOrEqual(t, positiveRange64, int64(10))
	assert.LessOrEqual(t, positiveRange64, int64(50))

	// Test with positive range for unsigned types
	positiveUintRange := random.UintInRange(5, 25)
	assert.GreaterOrEqual(t, positiveUintRange, uint(5))
	assert.LessOrEqual(t, positiveUintRange, uint(25))
}
