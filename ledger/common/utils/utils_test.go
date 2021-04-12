package utils

import (
	"math/big"
	"math/bits"
	"math/rand"
	"time"

	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBitTools(t *testing.T) {
	seed := time.Now().UnixNano()
	t.Logf("rand seed is %d", seed)
	rand.Seed(seed)
	r := rand.NewSource(seed)

	const maxBits = 24 // upper bound of indices to test
	var b big.Int

	t.Run("Bit", func(t *testing.T) {
		var max big.Int
		// set max to 2^maxBits
		max.SetBit(&max, maxBits, 1)
		// random big int less that 2^maxBits
		b.Rand(rand.New(r), &max)
		maxBitsLen := (maxBits + 7) / 8 // length in bytes needed for maxbits
		bytes := make([]byte, maxBitsLen)
		copy(bytes[maxBitsLen-len(b.Bytes()):], b.Bytes())

		// reverse the endianness (both bits and bytes)
		for j := 0; j < len(bytes)/2; j++ {
			bytes[j], bytes[len(bytes)-j-1] = bytes[len(bytes)-j-1], bytes[j]
		}
		for j := 0; j < len(bytes); j++ {
			bytes[j] = bits.Reverse8(bytes[j])
		}
		// test bit reads are equal for all indices
		for i := 0; i < maxBits; i++ {
			bit := int(b.Bit(i))
			assert.Equal(t, bit, Bit(bytes, i))
		}
	})

	t.Run("SetBit", func(t *testing.T) {
		b.SetInt64(0)
		bytes := make([]byte, (maxBits+7)/8) // length in bytes needed for maxbits
		// build a random big bit by bit
		for i := 0; i < maxBits; i++ {
			bit := rand.Intn(2)
			// b = 2*b + bit
			b.Lsh(&b, 1)
			b.Add(&b, big.NewInt(int64(bit)))
			// sets the bit at index i
			if bit == 1 {
				SetBit(bytes, i)
			}
		}

		// get the big int from the random slice
		var randomBig big.Int
		randomBig.SetBytes(bytes)
		assert.Equal(t, 0, randomBig.Cmp(&b))
	})
}
