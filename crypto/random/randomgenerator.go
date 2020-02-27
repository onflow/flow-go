package random

// RandomGenerator is a stateful pseudo random number generator
type RandomGenerator interface {
	// IntN returns an int random number between 0 and "to" (exclusive)
	IntN(int) int
}
