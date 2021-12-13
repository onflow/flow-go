package random

import (
	"encoding/binary"
	"fmt"
)

// Rand is a pseudo random number generator
type Rand interface {
	// Read fills the input slice with random bytes.
	Read([]byte)

	// UintN returns a random number between 0 and N (exclusive)
	UintN(uint64) uint64

	// Permutation returns a permutation of the set [0,n-1]
	// the theoretical output space grows very fast with (!n) so that input (n) should be chosen carefully
	// to make sure the function output space covers a big chunk of the theoretical outputs.
	// The returned error is non-nil if the parameter is a negative integer.
	Permutation(n int) ([]int, error)

	// SubPermutation returns the m first elements of a permutation of [0,n-1]
	// the theoretical output space can be large (n!/(n-m)!) so that the inputs should be chosen carefully
	// to make sure the function output space covers a big chunk of the theoretical outputs.
	// The returned error is non-nil if the parameter is a negative integer.
	SubPermutation(n int, m int) ([]int, error)

	// Shuffle permutes an ordered data structure of an arbitrary type in place. The main use-case is
	// permuting slice or array elements. (n) is the size of the data structure.
	// the theoretical output space grows very fast with the slice size (n!) so that input (n) should be chosen carefully
	// to make sure the function output space covers a big chunk of the theoretical outputs.
	// The returned error is non-nil if any of the parameters is a negative integer.
	Shuffle(n int, swap func(i, j int)) error

	// Samples picks (m) random ordered elements of a data structure of an arbitrary type of total size (n). The (m) elements are placed
	// in the indices 0 to (m-1) with in place swapping. The data structure ends up being a permutation of the initial (n) elements.
	// While the sampling of the (m) elements is pseudo-uniformly random, there is no guarantee about the uniformity of the permutation of
	// the (n) elements. The function Shuffle should be used in case the entire (n) elements need to be shuffled.
	// The main use-case of the data structure is a slice or array.
	// The theoretical output space grows very fast with the slice size (n!/(n-m)!) so that inputs should be chosen carefully
	// to make sure the function output space covers a big chunk of the theoretical outputs.
	// The returned error is non-nil if any of the parameters is a negative integer.
	Samples(n int, m int, swap func(i, j int)) error

	// State returns the internal state of the random generator.
	// The internal state can be used as a seed input for the function
	// Restore to restore an identical PRG (with the same internal state)
	State() []byte
}

// randCore is PRG providing the core Read function of a PRG.
// All other Rand methods use the core Read method.
//
// In order to add a new Rand implementation,
// it should be enough to implement randCore.
type randCore interface {
	// Read fills the input slice with random bytes.
	Read([]byte)
}

// genericPRG implements all the Rand methods using the embedded randCore method.
// All implementations of the Rand interface should embed the genericPRG struct.
type genericPRG struct {
	randCore
}

// UintN returns an uint64 pseudo-random number in [0,n-1],
// using `p` as an entropy source.
func (p *genericPRG) UintN(n uint64) uint64 {
	// TODO: use a single array per genericPRG  - precise concurrency assumptions in GoDoc
	// TODO: improve uniform distribution of UintN: for loop or higher sample
	bytes := make([]byte, 8)
	p.Read(bytes)

	random := binary.LittleEndian.Uint64(bytes)
	return random % n
}

// Permutation returns a permutation of the set [0,n-1].
// It implements Fisher-Yates Shuffle (inside-out variant) using `p` as a random source.
// The output space grows very fast with (!n) so that input `n` should be chosen carefully
// to guarantee a good uniformity of the output.
//
// O(n) space and O(n) time.
func (p *genericPRG) Permutation(n int) ([]int, error) {
	if n < 0 {
		return nil, fmt.Errorf("population size cannot be negative")
	}
	items := make([]int, n)
	for i := 0; i < n; i++ {
		j := p.UintN(uint64(i + 1))
		items[i] = items[j]
		items[j] = i
	}
	return items, nil
}

// SubPermutation returns the `m` first elements of a permutation of [0,n-1].
//
// It implements Fisher-Yates Shuffle using `p` as a source of randoms.
//
// O(n) space and O(n) time
func (p *genericPRG) SubPermutation(n int, m int) ([]int, error) {
	if m < 0 {
		return nil, fmt.Errorf("sample size cannot be negative")
	}
	if n < m {
		return nil, fmt.Errorf("sample size (%d) cannot be larger than entire population (%d)", m, n)
	}
	// condition n >= 0 is enforced by function Permutation(n)
	items, _ := p.Permutation(n)
	return items[:m], nil
}

// Shuffle permutes the given slice in place.
//
// It implements Fisher-Yates Shuffle using `p` as a source of randoms.
//
// O(1) space and O(n) time
func (p *genericPRG) Shuffle(n int, swap func(i, j int)) error {
	if n < 0 {
		return fmt.Errorf("population size cannot be negative")
	}
	for i := n - 1; i > 0; i-- {
		j := p.UintN(uint64(i + 1))
		swap(i, int(j))
	}
	return nil
}

// Samples picks randomly m elements out of n elemnts and places them
// in random order at indices [0,m-1], the swapping being implemented in place.
//
// It implements the first (m) elements of Fisher-Yates Shuffle using `p` as a source of randoms.
//
// O(1) space and O(m) time
func (p *genericPRG) Samples(n int, m int, swap func(i, j int)) error {
	if m < 0 {
		return fmt.Errorf("inputs cannot be negative")
	}
	if n < m {
		return fmt.Errorf("sample size (%d) cannot be larger than entire population (%d)", m, n)
	}
	for i := 0; i < m; i++ {
		j := p.UintN(uint64(n - i))
		swap(i, i+int(j))
	}
	return nil
}
