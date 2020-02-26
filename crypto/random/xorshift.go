package random

import (
	"fmt"
)

// xorshift is a pseudo random number generator
type xorshift struct {
	states     []uint64
	stateIndex int
}

// Next generates a new state given previous state
// Xorshift is used with these parameters (13, 9, 15)
func (x *xorshift) next(index int) {
	x.states[index] ^= x.states[index] << 13
	x.states[index] ^= x.states[index] >> 9
	x.states[index] ^= x.states[index] << 15
}

// IntN returns an int random number between 0 and "to" (exclusive)
func (x *xorshift) IntN(to int) int {
	res := x.states[x.stateIndex] % uint64(to)
	x.next(x.stateIndex)
	x.stateIndex = (x.stateIndex + 1) % len(x.states)
	return int(res)
}

// NewRand returns a new random generator (xorshift)
func NewRand(seed []uint64) (*xorshift, error) {
	size := len(seed)
	// seed slice can not include zeros
	for i := range seed {
		if seed[i] == uint64(0) {
			return nil, fmt.Errorf("seed slice can not have a zero value")
		}
	}
	// stateIndex initialized with first element of seed % rand size
	states := make([]uint64, len(seed))
	copy(states, seed)
	rand := &xorshift{states: states, stateIndex: int(seed[0] % uint64(size))}
	// initial next
	for i := range rand.states {
		rand.next(i)
	}
	return rand, nil
}

// PermutateSubset implements Fisher-Yates Shuffle (inside-outside) on a slice of ints with
// size n using the given seed s and returns a subset m of it
func PermutateSubset(n int, m int, rng RandomGenerator) ([]int, error) {
	items, err := Permute(n, rng)
	if err != nil {
		return nil, err
	}
	return items[:m], nil
}

// Permute implements Fisher-Yates Shuffle (inside-outside) on a slice of ints with
// size n using the given seed s and returns the slice
func Permute(n int, rng RandomGenerator) ([]int, error) {
	items := make([]int, n)
	for i := 0; i < n; i++ {
		j := rng.IntN(i + 1)
		if j != i {
			items[i] = items[j]
		}
		items[j] = i
	}
	return items, nil
}
