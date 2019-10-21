package crypto

// Rand is a deterministic random number generator
type Rand struct {
	states     []uint64
	stateIndex int
}

// Next generates a new state given previous state
// Xorshift is used with these parameters (13, 9, 15)
func (x *Rand) Next() {
	for i := range x.states {
		x.states[i] ^= (x.states[i] << 13)
		x.states[i] ^= (x.states[i] >> 9)
		x.states[i] ^= (x.states[i] << 15)
	}
}

// UIntN returns an uint random number between 0 and "to" (exclusive)
func (x *Rand) UIntN(to uint64) uint64 {
	res := x.states[x.stateIndex] % to
	x.Next()
	x.stateIndex = (x.stateIndex + 1) % len(x.states)
	return res
}

// IntN returns an int random number between 0 and "to" (exclusive)
func (x *Rand) IntN(to int) int {
	res := x.states[x.stateIndex] % uint64(to)
	x.Next()
	x.stateIndex = (x.stateIndex + 1) % len(x.states)
	return int(res)
}

// NewRand returns a new random generator (Rand)
func NewRand(seed []uint64) *Rand {
	size := len(seed)
	// making sure the seed is not zero
	for i, s := range seed {
		seed[i] = s | uint64(1)
	}
	// stateIndex initialized with first element of seed % rand size
	rand := &Rand{states: seed, stateIndex: int(seed[0] % uint64(size))}
	rand.Next()
	return rand
}
