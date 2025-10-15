package fixtures

import (
	"math/rand"
)

const (
	// charset is the set of characters used to generate random strings.
	charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// RandomGenerator provides random value generation with consistent randomness.
// Exposes all methods from *rand.Rand, as well as additional methods to reduce
type RandomGenerator struct {
	*rand.Rand
}

func NewRandomGenerator(rng *rand.Rand) *RandomGenerator {
	return &RandomGenerator{
		Rand: rng,
	}
}

// RandomBytes generates n random bytes.
func (g *RandomGenerator) RandomBytes(n int) []byte {
	bytes := make([]byte, n)
	read, err := g.Read(bytes)
	NoError(err)
	Assertf(read == n, "expected to read %d bytes, got %d", n, read)

	return bytes
}

// RandomString generates a random string of the specified length.
func (g *RandomGenerator) RandomString(length uint) string {
	result := make([]byte, length)
	for i := range length {
		result[i] = charset[g.Intn(len(charset))]
	}
	return string(result)
}

// Uint64n generates a random uint64 strictly less than `n`.
// Uses rand.Int63n to generate a random int64 and then casts it to uint64. n MUST be > 0.
func (g *RandomGenerator) Uint64n(n uint64) uint64 {
	return uint64(g.Int63n(int64(n)))
}

// Uint32n generates a random uint32 strictly less than `n`.
// Uses rand.Int31n to generate a random int32 and then casts it to uint32. n MUST be > 0.
func (g *RandomGenerator) Uint32n(n uint32) uint32 {
	return uint32(g.Int31n(int32(n)))
}

// Uint16n generates a random uint16 strictly less than `n`.
// Uses rand.Int31n to generate a random int32 and then casts it to uint16. n MUST be > 0.
func (g *RandomGenerator) Uint16n(n uint16) uint16 {
	return uint16(g.Int31n(int32(n)))
}

// Uint8n generates a random uint8 strictly less than `n`.
// Uses rand.Int31n to generate a random int32 and then casts it to uint8. n MUST be > 0.
func (g *RandomGenerator) Uint8n(n uint8) uint8 {
	return uint8(g.Int31n(int32(n)))
}

// Uintn generates a random uint strictly less than `n`.
// Uses rand.Int63n to generate a random int64 and then casts it to uint. n MUST be > 0.
func (g *RandomGenerator) Uintn(n uint) uint {
	return uint(g.Int63n(int64(n)))
}

// Uint64InRange generates a random uint64 in the inclusive range [min, max].
// `max` must be strictly greater than `min` or the method will panic.
func (g *RandomGenerator) Uint64InRange(min, max uint64) uint64 {
	return InclusiveRange(g, min, max)
}

// Uint32InRange generates a random uint32 in the inclusive range [min, max].
// `max` must be strictly greater than `min` or the method will panic.
func (g *RandomGenerator) Uint32InRange(min, max uint32) uint32 {
	return InclusiveRange(g, min, max)
}

// UintInRange generates a random uint in the inclusive range [min, max].
// `max` must be strictly greater than `min` or the method will panic.
func (g *RandomGenerator) UintInRange(min, max uint) uint {
	return InclusiveRange(g, min, max)
}

// Int64InRange generates a random int64 in the inclusive range [min, max].
// Min and max MUST be positive numbers and `max` must be strictly greater than `min` or the method
// will panic.
func (g *RandomGenerator) Int64InRange(min, max int64) int64 {
	return InclusiveRange(g, min, max)
}

// Int32InRange generates a random int32 in the inclusive range [min, max].
// Min and max MUST be positive numbers and `max` must be strictly greater than `min` or the method
// will panic.
func (g *RandomGenerator) Int32InRange(min, max int32) int32 {
	return InclusiveRange(g, min, max)
}

// IntInRange generates a random int in the inclusive range [min, max].
// Min and max MUST be positive numbers and `max` must be strictly greater than `min` or the method
// will panic.
func (g *RandomGenerator) IntInRange(min, max int) int {
	return InclusiveRange(g, min, max)
}

// Bool generates a random bool.
func (g *RandomGenerator) Bool() bool {
	return g.Intn(2) == 0
}

// InclusiveRange generates a random number of type T in the inclusive range [min, max].
// Min and max MUST be positive numbers and `max` must be strictly greater than `min` or the method
// will panic.
func InclusiveRange[T ~uint64 | ~uint32 | ~uint | ~int64 | ~int32 | ~int](g *RandomGenerator, min, max T) T {
	return min + T(g.Intn(int(max)+1-int(min)))
}

// RandomElement selects a random element from the provided slice.
// Returns the zero value of T if the slice is empty.
func RandomElement[T any](g *RandomGenerator, slice []T) T {
	if len(slice) == 0 {
		var zero T
		return zero
	}
	return slice[g.Intn(len(slice))]
}
