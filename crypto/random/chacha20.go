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
// -incremented after each block- and 3 of stream_id).

// TODO: update
type chacha20s struct {
	state chacha20.Cipher
	// Only used for State/Restore functionality
	counter      uint64
	initialBytes []byte
}

const (
	KeySize   = chacha20.KeySize
	NonceSize = chacha20.NonceSize
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
	if len(stream_id) != NonceSize {
		return nil, fmt.Errorf("new Rand stream_id should be %d bytes", NonceSize)
	}

	// create the Chacha20 state, initialized with the seed as a key, and the streamID as a nonce
	chacha, err := chacha20.NewUnauthenticatedCipher(seed, streamID)
	if err != nil {
		return nil, fmt.Errorf("chacha20 instance creation failed: %w", err)
	}

	// init the chacha20s
	rand := &chacha20s{
		state:        chacha,
		initialBytes: append(seed, stream_id...),
	}
	return rand, nil
}

// UintN returns an uint64 pseudo-random number in [0,n-1]
// using the chacha20 of the current index. The index is updated
// to use another chacha20 at the next round
func (x *chacha20s) UintN(n uint64) uint64 {
	bytes := make([]byte, 8)
	x.states[x.stateIndex].XORKeyStream(bytes, bytes)
	x.counters[x.stateIndex]++
	res := binary.LittleEndian.Uint64(bytes) % n
	// update the index
	x.stateIndex = (x.stateIndex + 1) % len(x.states)
	return res
}

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

// State returns the internal state of the concatenated Chacha20s (this is used
// for serde purposes)
func (x *chacha20s) State() []byte {
	bytes := x.initialBytes
	for i := 0; i < len(x.counters); i++ {
		b := make([]byte, 8)
		binary.LittleEndian.PutUint64(b, x.counters[i])
		bytes = append(bytes, b...)
	}
	idx := make([]byte, 4)
	binary.LittleEndian.PutUint32(idx, uint32(x.stateIndex))
	bytes = append(bytes, idx...)
	length := make([]byte, 4)
	binary.LittleEndian.PutUint32(length, uint32(len(x.states)))
	bytes = append(bytes, length...)
	return bytes
}

func Restore(stateBytes []byte) (*chacha20s, error) {
	// we expect k*32 bytes of seed, 12 bytes of stream_id, k*8 bytes of
	// counters, 4 bytes of index, 4 bytes of length (== k)
	n := len(stateBytes)

	// safety check
	// minimum (k==1) 46 bytes
	if n < 46 {
		return nil, fmt.Errorf("Rand state length should be of the form k * %d + %d + k * 8 + 8 bytes", KeySize, NonceSize)
	}
	k := binary.LittleEndian.Uint32(stateBytes[n-4:])
	if uint32(n) != (k*KeySize + NonceSize + k*8 + 4 + 4) {
		return nil, fmt.Errorf("Rand state length should be of the form k * %d + %d + k * 8 + 8 bytes", KeySize, NonceSize)
	}

	// create the Chacha20 instances
	states := make([]chacha20.Cipher, 0, k)
	counters := make([]uint64, 0, k)

	// initialize the Chacha20s with the state, initialize the counters
	stream_id := stateBytes[k*KeySize : k*KeySize+NonceSize]
	postInitialBytesOffset := int(k*KeySize + NonceSize)
	for i := 0; i < int(k); i++ {
		chacha, err := chacha20.NewUnauthenticatedCipher(stateBytes[i*KeySize:(i+1)*KeySize], stream_id)
		if err != nil {
			return nil, fmt.Errorf("CSPRNG instance creation failed at index %d", i)
		}
		// retrieve and set the counter, both in the chacha20 instance and the
		// secondary index
		counter := binary.LittleEndian.Uint64(stateBytes[postInitialBytesOffset+8*i : postInitialBytesOffset+8*(i+1)])
		counters = append(counters, counter)

		// counters indicate the # of queried uint64s, i.e. they count in 8 bytes increments
		// internal chacha counters count in 64 bytes increments, so we might have to
		// query the remainder
		fullCounts := uint32(counter / 8)
		remainingInts := counter % 8
		chacha.SetCounter(fullCounts)
		// query the remaining bits and discard the result
		remainderStream := make([]byte, 8*remainingInts)
		chacha.XORKeyStream(remainderStream, remainderStream)

		states = append(states, *chacha)
	}
	// initialize the state index
	idx := int(binary.LittleEndian.Uint32(stateBytes[n-8 : n-4]))
	rand := &chacha20s{
		states:       states,
		stateIndex:   idx,
		counters:     counters,
		initialBytes: stateBytes[:k*KeySize+NonceSize],
	}
	return rand, nil
}
