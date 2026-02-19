// Package rand is a wrapper around `crypto/rand` that uses the system RNG underneath
// to extract secure entropy.
//
// It implements useful tools that are not exported by the `crypto/rand` package.
// This package should be used instead of `math/rand` for any use-case requiring
// a secure randomness. It provides similar APIs to the ones provided by `math/rand`.
// This package does not implement any determinstic RNG (Pseudo-RNG) based on
// user input seeds. For the deterministic use-cases please use `github.com/onflow/crypto/random`.
//
// Functions in this package may return an error if the underlying system implementation fails
// to read new randoms. When that happens, this package considers it an irrecoverable exception.
package rand

import (
	"crypto/rand"
	"encoding/base64"
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
	// 64 bits are sampled but only 32 bits are used. This does not affect the uniformity of the output
	// assuming that the 64-bits distribution is uniform
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
	return uint32(r), err // `r` is less than `n` and necessarily fits in 32 bits
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
	for i := range m {
		j, err := Uintn(n - i)
		if err != nil {
			return err
		}
		swap(i, i+j)
	}
	return nil
}

// GenerateRandomString generates a cryptographically secure random string of size n.
// n must be > 0
func GenerateRandomString(length int) (string, error) {
	if length <= 0 {
		return "", fmt.Errorf("length should greater than 0, got %d", length)
	}

	// The base64 encoding uses 64 different characters to represent data in
	// strings, which makes it possible to represent 6 bits of data with each
	// character (as 2^6 is 64). This means that every 3 bytes (24 bits) of
	// input data will be represented by 4 characters (4 * 6 bits) in the
	// base64 encoding. Consequently, base64 encoding increases the size of
	// the data by approximately 1/3 compared to the original input data.
	//
	// 1. (n+3) / 4 - This calculates how many groups of 4 characters are needed
	//    in the base64 encoded output to represent at least 'n' characters.
	//    The +3 ensures rounding up, as integer division truncates the result.
	//
	// 2. ... * 3 - Each group of 4 base64 characters represents 3 bytes
	//    of input data. This multiplication calculates the number of bytes
	//    needed to produce the required length of the base64 string.
	byteSlice := make([]byte, (length+3)/4*3)
	_, err := rand.Read(byteSlice)
	if err != nil {
		return "", fmt.Errorf("failed to generate random string: %w", err)
	}

	encodedString := base64.URLEncoding.EncodeToString(byteSlice)
	return encodedString[:length], nil
}
