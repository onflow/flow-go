package random

// Rand is a pseudo random number generator
type Rand interface {
	// IntN returns an uint64 random number between 0 and "to" (exclusive)
	IntN(int) int
	// Permutation returns a permutation of the set [0,n-1]
	// the theretical output space grows very fast with (!n) so that input (n) should be chosen carefully
	// to make sure the function output space covers a big chunk of the theortical outputs.
	Permutation(n int) []int
	// SubPermutation returns the m first elements of a permutation of [0,n-1]
	// the theretical output space can be large (!n/!(n-m)) so that the inputs should be chosen carefully
	// to make sure the function output space covers a big chunk of the theortical outputs.
	SubPermutation(n int, m int) []int
}
