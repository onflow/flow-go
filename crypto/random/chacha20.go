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
// ChaCha is a stream cipher designed by Daniel J. Bernstein[^1], that we use as an RNG. It is
// an improved variant of the Salsa20 cipher family, which was selected as one of the "stream
// ciphers suitable for widespread adoption" by eSTREAM[^2].
//
// We use a 64-bit counter and 64-bit stream identifier as in Bernstein's implementation[^1]
// except that we use a stream identifier in place of a nonce. A 64-bit counter over 64-byte
// (16 word) blocks allows 1 ZiB of output before cycling, and the stream identifier allows
// 2<sup>64</sup> unique streams of output per seed.
//
// [^1]: D. J. Bernstein, [*ChaCha, a variant of Salsa20*](
//       https://cr.yp.to/chacha.html)
//
// [^2]: [eSTREAM: the ECRYPT Stream Cipher Project](
//       http://www.ecrypt.eu.org/stream/)
//
// a chacha20 has a 16 words (256 bits) of state (4 constant, 8 of seed, 1 of counter
// -incremented after each block- and 3 of streamID).

// TODO: update
type chacha20s struct {
	state chacha20.Cipher
	// Only used for State/Restore functionality
	counter      uint64 // TODO: update to 32
	initialBytes []byte
}

const (
	// TODO: private?
	KeySize   = chacha20.KeySize
	NonceSize = chacha20.NonceSize
	// public ?
	Chacha20SeedLen  = KeySize
	Chacha20MaxIDLen = NonceSize
)

// NewChacha20 returns a new PRG that is a set of ChaCha20 PRGs, seeded with
// the input seed and a stream identifier (12 bytes).
//
// The input seed is the initial state of the PRG, it is recommended to sample the
// seed uniformly at random.
//
// The length of the seed fixes the number of ChaCha20 instances to initialize:
// each 32 bytes of the seed initialize a ChaCha20 instance. The seed length
// has to be a multiple of 32 (the CSPRG state size).
func NewChacha20(seed []byte, streamID []byte) (*chacha20s, error) {

	// check the key size
	if len(seed) != KeySize {
		return nil, fmt.Errorf("chacha20 seed length should be %d, got %d", KeySize, len(seed))
	}

	// TODO: update by adding a maximum length and padding
	// check the nonce size
	if len(streamID) != NonceSize {
		return nil, fmt.Errorf("new Rand streamID should be %d bytes", NonceSize)
	}

	// create the Chacha20 state, initialized with the seed as a key, and the streamID as a nonce
	chacha, err := chacha20.NewUnauthenticatedCipher(seed, streamID)
	if err != nil {
		return nil, fmt.Errorf("chacha20 instance creation failed: %w", err)
	}

	// init the chacha20s
	rand := &chacha20s{
		state:        *chacha,
		initialBytes: append(seed, streamID...),
	}
	return rand, nil
}

// TODO : update GoDoc

// UintN returns an uint64 pseudo-random number in [0,n-1]
// using the chacha20 state.
func (x *chacha20s) UintN(n uint64) uint64 {
	// empty message to encrypt
	// TODO: use a single array per chacha  - precise concurrency assumptions in GoDoc
	// TODO: optimization: encrypt the entire block (512 bits) instead of of 64 bits
	bytes := make([]byte, 8)
	x.state.XORKeyStream(bytes, bytes)
	// increase the counter
	x.counter++

	random := binary.LittleEndian.Uint64(bytes)
	return random % n
}

// TODO: move to a generic PRG struct?

// Permutation returns a permutation of the set [0,n-1]
// it implements Fisher-Yates Shuffle (inside-out variant) using (x) as a random source
// the output space grows very fast with (!n) so that input (n) and the seed length
// (which fixes the internal state length of xorshifts ) should be chosen carefully
// O(n) space and O(n) time
func (x *chacha20s) Permutation(n int) ([]int, error) {
	if n < 0 {
		return nil, fmt.Errorf("population size cannot be negative")
	}
	items := make([]int, n)
	for i := 0; i < n; i++ {
		j := x.UintN(uint64(i + 1))
		items[i] = items[j]
		items[j] = i
	}
	return items, nil
}

// TODO: move to a generic PRG struct?

// SubPermutation returns the m first elements of a permutation of [0,n-1]
// It implements Fisher-Yates Shuffle using x as a source of randoms
// O(n) space and O(n) time
func (x *chacha20s) SubPermutation(n int, m int) ([]int, error) {
	if m < 0 {
		return nil, fmt.Errorf("sample size cannot be negative")
	}
	if n < m {
		return nil, fmt.Errorf("sample size (%d) cannot be larger than entire population (%d)", m, n)
	}
	// condition n >= 0 is enforced by function Permutation(n)
	items, _ := x.Permutation(n)
	return items[:m], nil
}

// TODO: move to a generic PRG struct?

// Shuffle permutes the given slice in place
// It implements Fisher-Yates Shuffle using x as a source of randoms
// O(1) space and O(n) time
func (x *chacha20s) Shuffle(n int, swap func(i, j int)) error {
	if n < 0 {
		return fmt.Errorf("population size cannot be negative")
	}
	for i := n - 1; i > 0; i-- {
		j := x.UintN(uint64(i + 1))
		swap(i, int(j))
	}
	return nil
}

// TODO: move to a generic PRG struct?

// Samples picks randomly m elements out of n elemnts and places them
// in random order at indices [0,m-1]. The swapping is done in place
// It implements the first (m) elements of Fisher-Yates Shuffle using x as a source of randoms
// O(1) space and O(m) time
func (x *chacha20s) Samples(n int, m int, swap func(i, j int)) error {
	if m < 0 {
		return fmt.Errorf("inputs cannot be negative")
	}
	if n < m {
		return fmt.Errorf("sample size (%d) cannot be larger than entire population (%d)", m, n)
	}
	for i := 0; i < m; i++ {
		j := x.UintN(uint64(n - i))
		swap(i, i+int(j))
	}
	return nil
}

// State returns the internal state of the concatenated Chacha20s
// (this is used for serde purposes)
// TODO: update the name (serialize, encode, marshall ?)
func (x *chacha20s) State() []byte {
	counterBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(counterBytes, x.counter)
	bytes := append(x.initialBytes, counterBytes...)
	// output is seed || streamID || counter
	return bytes
}

func Restore(stateBytes []byte) (*chacha20s, error) {
	// inpout should be seed (32 bytes) || streamID (12 bytes) || counter (8 bytes)
	const expectedLen = KeySize + NonceSize + 8

	// check input length
	if len(stateBytes) != expectedLen {
		return nil, fmt.Errorf("Rand state length should be of %d bytes, got %d", expectedLen, len(stateBytes))
	}

	seed := stateBytes[:KeySize]
	streamID := stateBytes[KeySize : KeySize+NonceSize]
	counterBytes := stateBytes[KeySize+NonceSize:]

	// create the Chacha20 instance with seed and streamID
	chacha, err := chacha20.NewUnauthenticatedCipher(seed, streamID)
	if err != nil {
		return nil, fmt.Errorf("Chacha20 instance creation failed: %w", err)
	}
	// set the counter
	counter := binary.LittleEndian.Uint64(counterBytes)

	// TODO : replace 8 by constants
	full512Count := uint32(counter / 8)
	remaining64Count := counter % 8
	chacha.SetCounter(full512Count)
	// query the remaining bits and discard the result to catch the stored chacha state
	remainderStream := make([]byte, remaining64Count*8)
	chacha.XORKeyStream(remainderStream, remainderStream)

	rand := &chacha20s{
		state:        *chacha,
		counter:      counter,
		initialBytes: stateBytes[:KeySize+NonceSize],
	}
	return rand, nil
}
