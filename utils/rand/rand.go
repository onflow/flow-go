// Package rand is a wrapper around `crypto/rand` that uses the system RNG underneath
// to extract secure entropy.
//
// It implements useful tools that are not exported by the `crypto/rand` package.
// This package should be used instead of `math/rand` for any use-case requiring
// a secure randomness. It provides similar APIs to the ones provided by `math/rand`.
// This package does not implement any determinstic RNG (Pseudo-RNG) based on
// user input seeds. For the deterministic use-cases please use `flow-go/crypto/random`.
//
// Functions in this package may return an error if the underlying system implementation fails
// to read new randoms. When that happens, this package considers it an irrecoverable exception.
package rand

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

// Uint64 returns a random uint64.
//
// It returns:
//   - (0, exception) if crypto/rand fails to provide entropy which is likely a result of a system error.
//   - (random, nil) otherwise
func Uint64() (uint64, error) {
	// allocate a new memory at each call. Another possibility
	// is to use a global variable but that would make the package non thread safe
	buffer := make([]byte, 8)
	if _, err := rand.Read(buffer); err != nil { // checking err in crypto/rand.Read is enough
		return 0, fmt.Errorf("crypto/rand read failed: %w", err)
	}
	r := binary.LittleEndian.Uint64(buffer)
	return r, nil
}

// Uint64n returns a random uint64 strictly less than `n`.
// `n` has to be a strictly positive integer.
//
// It returns:
//   - (0, exception) if `n==0`
//   - (0, exception) if crypto/rand fails to provide entropy which is likely a result of a system error.
//   - (random, nil) otherwise
func Uint64n(n uint64) (uint64, error) {
	if n == 0 {
		return 0, fmt.Errorf("n should be strictly positive, got %d", n)
	}
	// the max returned random is n-1 > 0
	max := n - 1
	// count the bytes size of max
	size := 0
	for tmp := max; tmp != 0; tmp >>= 8 {
		size++
	}
	// get the bit size of max
	mask := uint64(0)
	for max&mask != max {
		mask = (mask << 1) | 1
	}

	// allocate a new memory at each call. Another possibility
	// is to use a global variable but that would make the package non thread safe
	buffer := make([]byte, 8)

	// Using 64 bits of random and reducing modulo n does not guarantee a high uniformity
	// of the result.
	// For a better uniformity, loop till a sample is less or equal to `max`.
	// This means the function might take longer time to output a random.
	// Using the size of `max` in bits helps the loop end earlier (the algo stops after one loop
	// with more than 50%)
	// a different approach would be to pull at least 128 bits from the random source
	// and use big number modular reduction by `n`.
	random := n
	for random > max {
		if _, err := rand.Read(buffer[:size]); err != nil { // checking err in crypto/rand.Read is enough
			return 0, fmt.Errorf("crypto/rand read failed: %w", err)
		}
		random = binary.LittleEndian.Uint64(buffer)
		random &= mask // adjust to the size of max in bits
	}
	return random, nil
}

// Uint32 returns a random uint32.
//
// It returns:
//   - (0, exception) if crypto/rand fails to provide entropy which is likely a result of a system error.
//   - (random, nil) otherwise
func Uint32() (uint32, error) {
	// for 64-bits machines, doing 64 bits operations and then casting
	// should be faster than dealing with 32 bits operations
	r, err := Uint64()
	return uint32(r), err
}

// Uint32n returns a random uint32 strictly less than `n`.
// `n` has to be a strictly positive integer.
//
// It returns an error:
//   - (0, exception) if `n==0`
//   - (0, exception) if crypto/rand fails to provide entropy which is likely a result of a system error.
//   - (random, nil) otherwise
func Uint32n(n uint32) (uint32, error) {
	r, err := Uint64n(uint64(n))
	return uint32(r), err
}

// Uint returns a random uint.
//
// It returns:
//   - (0, exception) if crypto/rand fails to provide entropy which is likely a result of a system error.
//   - (random, nil) otherwise
func Uint() (uint, error) {
	r, err := Uint64()
	return uint(r), err
}

// Uintn returns a random uint strictly less than `n`.
// `n` has to be a strictly positive integer.
//
// It returns an error:
//   - (0, exception) if `n==0`
//   - (0, exception) if crypto/rand fails to provide entropy which is likely a result of a system error.
//   - (random, nil) otherwise
func Uintn(n uint) (uint, error) {
	r, err := Uint64n(uint64(n))
	return uint(r), err
}

// Shuffle permutes a data structure in place
// based on the provided `swap` function.
// It is not deterministic.
//
// It implements Fisher-Yates Shuffle using crypto/rand as a source of randoms.
// It uses O(1) space and O(n) time
//
// It returns:
//   - (exception) if crypto/rand fails to provide entropy which is likely a result of a system error.
//   - (nil) otherwise
func Shuffle(n uint, swap func(i, j uint)) error {
	return Samples(n, n, swap)
}

// Samples picks randomly `m` elements out of `n` elements in a data structure
// and places them in random order at indices [0,m-1],
// the swapping being implemented in place. The data structure is defined
// by the `swap` function itself.
// Sampling is not deterministic like the other functions of the package.
//
// It implements the first `m` elements of Fisher-Yates Shuffle using
// crypto/rand as a source of randoms. `m` has to be less or equal to `n`.
// It uses O(1) space and O(m) time
//
// It returns:
//   - (exception) if `n < m`
//   - (exception) if crypto/rand fails to provide entropy which is likely a result of a system error.
//   - (nil) otherwise
func Samples(n uint, m uint, swap func(i, j uint)) error {
	if n < m {
		return fmt.Errorf("sample size (%d) cannot be larger than entire population (%d)", m, n)
	}
	for i := uint(0); i < m; i++ {
		j, err := Uintn(n - i)
		if err != nil {
			return err
		}
		swap(i, i+j)
	}
	return nil
}
