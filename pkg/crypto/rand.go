package crypto

// Rand is a deterministic pseudo random number generator
type Rand struct {
	states     []uint64
	stateIndex int
}

// Next generates a new state given previous state
// Xorshift is used with these parameters (13, 9, 15)
func (x *Rand) next(index int) {
	x.states[index] ^= (x.states[index] << 13)
	x.states[index] ^= (x.states[index] >> 9)
	x.states[index] ^= (x.states[index] << 15)
}

// IntN returns an int random number between 0 and "to" (exclusive)
func (x *Rand) IntN(to int) int {
	res := x.states[x.stateIndex] % uint64(to)
	x.next(x.stateIndex)
	x.stateIndex = (x.stateIndex + 1) % len(x.states)
	return int(res)
}

// NewRand returns a new random generator (Rand)
func NewRand(seed []uint64) (*Rand, error) {
	size := len(seed)
	// seed slice can not include zeros
	for i := range seed {
		if seed[i] == uint64(0) {
			return nil, &cryptoError{"seed slice can not have a zero value"}
		}
	}
	// stateIndex initialized with first element of seed % rand size
	rand := &Rand{states: seed, stateIndex: int(seed[0] % uint64(size))}
	// initial next
	for i := range rand.states {
		rand.next(i)
	}
	return rand, nil
}
