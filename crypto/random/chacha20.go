package random

import (
	"encoding/binary"
	"fmt"

	"golang.org/x/crypto/chacha20"
)

// TODO: update description, RFC and lengths

// We use Chacha20, to build a cryptographically secure random number generator
// that uses the ChaCha algorithm.
//
// ChaCha is a stream cipher designed by Daniel J. Bernstein[^1], that we use as an PRG. It is
// an improved variant of the Salsa20 cipher family.
//
// We use Chacha20 with a 256-bit key, a 192-bit stream identifier and a 32-bit counter as
// as specified in RFC 8439 [^2].
// The encryption key is used as the PRG seed while the stream identifer is used as a nonce
// to customize the PRG. The PRG outputs are the successive encryptions of a constant message.
//
// A 32-bit counter over 64-byte blocks allows 256 GiB of output before cycling,
// and the stream identifier allows 2^192 unique streams of output per seed.
// It is the caller's responsibility to avoid the PRG output cycling.
//
// [^1]: D. J. Bernstein, [*ChaCha, a variant of Salsa20*](
//       https://cr.yp.to/chacha.html)
//
// [^2]: [RFC 8439: ChaCha20 and Poly1305 for IETF Protocols](
//       https://datatracker.ietf.org/doc/html/rfc8439)

type state struct {
	cipher chacha20.Cipher

	// Only used for State/Restore functionality

	// Counter of bytes encrypted so far by the sream cipher.
	// Note this is different than the internal counter of the chacha state
	// that counts the encrypted blocks of 512 bits.
	bytesCounter uint64
	// TODO change to key and nonce.
	initialBytes []byte
}

const (
	keySize   = chacha20.KeySize
	nonceSize = chacha20.NonceSize

	// Chacha20SeedLen is the seed length of the Chacha based PRG, it is fixed to 32 bytes.
	Chacha20SeedLen = keySize
	// Chacha20CustomizerMaxLen is the maximum length of the nonce used as a PRG customizer, it is fixed to 24 bytes.
	// Shorter customizers are padded by zeros to 24 bytes.
	Chacha20CustomizerMaxLen = nonceSize
)

// NewChacha20 returns a new Chacha20-based PRG, seeded with
// the input seed (32 bytes) and a customizer (up to 12 bytes).
//
// It is recommended to sample the seed uniformly at random.
// The function errors if the the seed is different than 32 bytes,
// or if the customizer is larger than 12 bytes.
func NewChacha20(seed []byte, customizer []byte) (*state, error) {

	// check the key size
	if len(seed) != Chacha20SeedLen {
		return nil, fmt.Errorf("chacha20 seed length should be %d, got %d", Chacha20SeedLen, len(seed))
	}

	// TODO: update by adding a maximum length and padding
	// check the nonce size
	if len(customizer) != Chacha20CustomizerMaxLen {
		return nil, fmt.Errorf("new Rand streamID should be %d bytes", Chacha20CustomizerMaxLen)
	}

	// create the Chacha20 state, initialized with the seed as a key, and the customizer as a streamID.
	chacha, err := chacha20.NewUnauthenticatedCipher(seed, customizer)
	if err != nil {
		return nil, fmt.Errorf("chacha20 instance creation failed: %w", err)
	}

	// init the state
	rand := &state{
		cipher:       *chacha,
		bytesCounter: 0,
		initialBytes: append(seed, customizer...),
	}
	return rand, nil
}

// TODO : update GoDoc

// UintN returns an uint64 pseudo-random number in [0,n-1]
// using the chacha20-based PRG.
func (c *state) UintN(n uint64) uint64 {
	// empty message to encrypt
	// TODO: use a single array per chacha  - precise concurrency assumptions in GoDoc
	// TODO: improve unioform distribution of UintN: for loop or higher sample
	bytes := make([]byte, 8)
	c.cipher.XORKeyStream(bytes, bytes)
	// increase the counter
	c.bytesCounter += 8

	random := binary.LittleEndian.Uint64(bytes)
	return random % n
}

// TODO: move to a generic PRG struct?

// Permutation returns a permutation of the set [0,n-1]
// it implements Fisher-Yates Shuffle (inside-out variant) using (x) as a random source
// the output space grows very fast with (!n) so that input (n) and the seed length
// (which fixes the internal state length of xorshifts ) should be chosen carefully
// O(n) space and O(n) time
func (c *state) Permutation(n int) ([]int, error) {
	if n < 0 {
		return nil, fmt.Errorf("population size cannot be negative")
	}
	items := make([]int, n)
	for i := 0; i < n; i++ {
		j := c.UintN(uint64(i + 1))
		items[i] = items[j]
		items[j] = i
	}
	return items, nil
}

// TODO: move to a generic PRG struct?

// SubPermutation returns the m first elements of a permutation of [0,n-1]
// It implements Fisher-Yates Shuffle using x as a source of randoms
// O(n) space and O(n) time
func (c *state) SubPermutation(n int, m int) ([]int, error) {
	if m < 0 {
		return nil, fmt.Errorf("sample size cannot be negative")
	}
	if n < m {
		return nil, fmt.Errorf("sample size (%d) cannot be larger than entire population (%d)", m, n)
	}
	// condition n >= 0 is enforced by function Permutation(n)
	items, _ := c.Permutation(n)
	return items[:m], nil
}

// TODO: move to a generic PRG struct?

// Shuffle permutes the given slice in place
// It implements Fisher-Yates Shuffle using x as a source of randoms
// O(1) space and O(n) time
func (c *state) Shuffle(n int, swap func(i, j int)) error {
	if n < 0 {
		return fmt.Errorf("population size cannot be negative")
	}
	for i := n - 1; i > 0; i-- {
		j := c.UintN(uint64(i + 1))
		swap(i, int(j))
	}
	return nil
}

// TODO: move to a generic PRG struct?

// Samples picks randomly m elements out of n elemnts and places them
// in random order at indices [0,m-1]. The swapping is done in place
// It implements the first (m) elements of Fisher-Yates Shuffle using x as a source of randoms
// O(1) space and O(m) time
func (c *state) Samples(n int, m int, swap func(i, j int)) error {
	if m < 0 {
		return fmt.Errorf("inputs cannot be negative")
	}
	if n < m {
		return fmt.Errorf("sample size (%d) cannot be larger than entire population (%d)", m, n)
	}
	for i := 0; i < m; i++ {
		j := c.UintN(uint64(n - i))
		swap(i, i+int(j))
	}
	return nil
}

// State returns the internal state of the concatenated Chacha20s
// (this is used for serde purposes)
// TODO: update the name (serialize, encode, marshall ?)
func (c *state) State() []byte {
	counter := make([]byte, 8)
	binary.LittleEndian.PutUint64(counter, c.bytesCounter)
	bytes := append(c.initialBytes, counter...)
	// output is seed || streamID || counter
	return bytes
}

func Restore(stateBytes []byte) (*state, error) {
	// inpout should be seed (32 bytes) || streamID (12 bytes) || bytesCounter (8 bytes)
	const expectedLen = keySize + nonceSize + 8

	// check input length
	if len(stateBytes) != expectedLen {
		return nil, fmt.Errorf("Rand state length should be of %d bytes, got %d", expectedLen, len(stateBytes))
	}

	seed := stateBytes[:keySize]
	streamID := stateBytes[keySize : keySize+nonceSize]
	bytesCounter := binary.LittleEndian.Uint64(stateBytes[keySize+nonceSize:])

	// create the Chacha20 instance with seed and streamID
	chacha, err := chacha20.NewUnauthenticatedCipher(seed, streamID)
	if err != nil {
		return nil, fmt.Errorf("Chacha20 instance creation failed: %w", err)
	}
	// set the bytes counter

	// each chacha internal block is 512 bits
	const bytesPerBlock = 512 >> 3
	blockCount := uint32(bytesCounter / bytesPerBlock)
	remainingBytes := bytesCounter % bytesPerBlock
	chacha.SetCounter(blockCount)
	// query the remaining bytes and to catch the stored chacha state
	remainderStream := make([]byte, remainingBytes)
	chacha.XORKeyStream(remainderStream, remainderStream)

	rand := &state{
		cipher:       *chacha,
		bytesCounter: bytesCounter,
		initialBytes: stateBytes[:keySize+nonceSize],
	}
	return rand, nil
}
