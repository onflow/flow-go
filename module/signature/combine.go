package signature

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
)

// Combiner creates a simple implementation for joining and splitting 2 signatures
// on a level above the cryptographic implementation. It simply concatenates
// signatures together and uses the stored information about signature lengths
// to split the concatenated bytes into its signature parts again.
type Combiner struct {
	lengthSig1 uint
	lengthSig2 uint
}

// NewCombiner creates a new combiner to join and split signatures.
func NewCombiner(lengthSig1, lengthSig2 uint) *Combiner {

	c := &Combiner{
		lengthSig1: lengthSig1,
		lengthSig2: lengthSig2,
	}
	return c
}

// Join will concatenate the provided 2 signatures into a common byte slice.
func (c *Combiner) Join(sig1, sig2 crypto.Signature) []byte {

	combined := make([]byte, 0, len(sig1)+len(sig2))
	combined = append(combined, sig1...)
	combined = append(combined, sig2...)
	return combined
}

// Split will split the given byte slice into its signature parts, using the
// embedded length information.
func (c *Combiner) Split(combined []byte) (crypto.Signature, crypto.Signature, error) {

	if uint(len(combined)) != c.lengthSig1+c.lengthSig2 {
		return nil, nil, fmt.Errorf("input length must be %d, got %d",
			c.lengthSig1+c.lengthSig2, len(combined))
	}

	sig1 := combined[:c.lengthSig1]
	sig2 := combined[c.lengthSig1:]

	return sig1, sig2, nil
}
