package random

import (
	"encoding/binary"
	"fmt"
)

// xorshifts is a set of xorshift128+ pseudo random number generators
// each xorshift+ has a 128-bits state
// using a set of xorshift128+ allows initializing the set with a larger
// seed than 128 bits
type xorshifts struct {
	states     []xorshiftp
	stateIndex int
}

// xorshiftp is a single xorshift128+ PRG
// the internal state is 128 bits
// http://vigna.di.unimi.it/ftp/papers/xorshiftplus.pdf
type xorshiftp struct {
	a, b uint64
}

// at least a 16 bytes constant
var zeroSeed = []byte("NothingUpMySleeve")

// NewRand returns a new PRG that is a set of xorshift128+ PRGs, seeded with the input seed
// the input seed is the initial state of the PRG.
// the length of the seed fixes the number of xorshift128+ to initialize:
// each 16 bytes of the seed initilize an xorshift128+ instance
// To make sure the seed entropy is optimal, the function checks that len(seed)
// is a multiple 16 (PRG state size)
func NewRand(seed []byte) (*xorshifts, error) {
	// safety check
	if len(seed) == 0 || len(seed)%16 != 0 {
		return nil, fmt.Errorf("new Rand seed length should be a non-zero multiple of 16")
	}
	// create the xorshift128+ instances
	states := make([]xorshiftp, 0, len(seed)/16)
	// initialize the xorshift128+ with the seed
	for i := 0; i < cap(states); i++ {
		states = append(states, xorshiftp{
			a: binary.BigEndian.Uint64(seed[i*16 : i*16+8]),
			b: binary.BigEndian.Uint64(seed[i*16+8 : (i+1)*16]),
		})
	}
	// check states are not zeros
	// replace the zero seed by a nothing-up-my-sleeve constant seed
	// the bias introduced is nigligible as the seed space is 2^128
	for _, x := range states {
		if x.a|x.b == 0 {
			x.a = binary.BigEndian.Uint64(zeroSeed[:8])
			x.b = binary.BigEndian.Uint64(zeroSeed[8:])
		}
	}
	// init the xorshifts
	rand := &xorshifts{
		states:     states,
		stateIndex: 0,
	}
	return rand, nil
}

// next generates updates the state of a single xorshift128+
func (x *xorshiftp) next() {
	// the xorshift+ shift parameters chosen for this instance
	const shift0 = 23
	const shift1 = 17
	const shift2 = 26
	var tmp uint64 = x.a
	x.a = x.b
	tmp ^= tmp << shift0
	tmp ^= tmp >> shift1
	tmp ^= x.b ^ (x.b >> shift2)
	x.b = tmp
}

// prn generated a Pseudo-random number out of the current state of
// xorshift128+ at index (index)
// prn does not change any prg state
func (x *xorshiftp) prn() uint64 {
	return x.a + x.b
}

// UintN returns an uint64 pseudo-random number in [0,n-1]
// using the xorshift+ of the current index. The index is updated
// to use another xorshift+ at the next round
func (x *xorshifts) UintN(n uint64) uint64 {
	res := x.states[x.stateIndex].prn() % n
	// update the state
	x.states[x.stateIndex].next()
	// update the index
	x.stateIndex = (x.stateIndex + 1) % len(x.states)
	return res
}

// Permutation returns a permutation of the set [0,n-1]
// it implements Fisher-Yates Shuffle (inside-out variant) using (x) as a random source
// the output space grows very fast with (!n) so that input (n) and the seed length
// (which fixes the internal state length of xorshifts ) should be chosen carefully
// O(n) space and O(n) time
func (x *xorshifts) Permutation(n int) ([]int, error) {
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
func (x *xorshifts) SubPermutation(n int, m int) ([]int, error) {
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
func (x *xorshifts) Shuffle(n int, swap func(i, j int)) error {
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
func (x *xorshifts) Samples(n int, m int, swap func(i, j int)) error {
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

// state returns the internal state of an xorshift128+
func (x *xorshiftp) state() []byte {
	state := make([]byte, 16)
	binary.BigEndian.PutUint64(state, x.a)
	binary.BigEndian.PutUint64(state[8:], x.b)
	return state
}

// State returns the internal state of the concatenated xorshifts
func (x *xorshifts) State() []byte {
	state := make([]byte, 0, 16*len(x.states))
	j := x.stateIndex
	for i := 0; i < len(x.states); i++ {
		xorshift := x.states[j%len(x.states)]
		state = append(state, xorshift.state()...)
		j++
	}
	return state
}
