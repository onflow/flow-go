package random

// Rand is a pseudo random number generator
type Rand interface {
	// IntN returns an uint64 random number between 0 and "to" (exclusive)
	IntN(int) (int, error)
	// Permutation returns a permutation of the set [0,n-1]
	// the theoretical output space grows very fast with (!n) so that input (n) should be chosen carefully
	// to make sure the function output space covers a big chunk of the theortical outputs.
	Permutation(n int) ([]int, error)
	// SubPermutation returns the m first elements of a permutation of [0,n-1]
	// the theoretical output space can be large (!n/!(n-m)) so that the inputs should be chosen carefully
	// to make sure the function output space covers a big chunk of the theortical outputs.
	SubPermutation(n int, m int) ([]int, error)
	// Shuffle permutes an ordered data structure of an aribtrary type in place. The main use-case is
	// permuting slice elements. (n) is the size of the data structure.
	// the theoretical output space grows very fast with the slice size (!n) so that input (n) should be chosen carefully
	// to make sure the function output space covers a big chunk of the theortical outputs.
	Shuffle(n int, swap func(i, j int))
}
