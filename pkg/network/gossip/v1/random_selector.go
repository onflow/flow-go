package gnode

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// uniqueRand is a type that returns unique random numbers in a given range
type uniqueRand struct {
	n         int
	generated map[int]bool
}

// newUniqSelector returns a new instance of uniqueRand, which is set to return unique random variables
// in the range [0,n-1]
// n: range of numbers from which the selector returns unique numbers
func newUniqSelector(n int) *uniqueRand {
	return &uniqueRand{generated: make(map[int]bool), n: n}
}

// Int returns a unique random integer that has not been returned by this instance
// of uniqueRand before.
func (u *uniqueRand) Int() (int, error) {

	// Declaration outside loop for performance concerns
	var (
		r   *big.Int
		i   = 0
		err error
		n64 = int64(u.n)
	)

	max := big.NewInt(n64)

	for {
		r, err = rand.Int(rand.Reader, max)
		if err != nil {
			return 0, fmt.Errorf("could not generate random number: %v", err)
		}

		//making sure that the number is in the range of indice
		i = int(r.Int64() % n64)

		if !u.generated[i] {
			u.generated[i] = true
			return i, nil
		}
	}
}
