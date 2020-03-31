package signature

import (
	"encoding/binary"
	"fmt"

	"github.com/dapperlabs/flow-go/crypto"
)

// Combiner creates a simple implementation for joining and splitting signatures
// on a level above the cryptographic implementation. It simply concatenates
// signatures together with their length information and uses this information
// to split the concatenated bytes into its signature parts again.
type Combiner struct {
}

// NewCombiner creates a new combiner to join and split signatures.
func NewCombiner() *Combiner {
	c := &Combiner{}
	return c
}

// Join will concatenate the provided signatures into a common byte slice, with
// added length information. It will never fail.
func (c *Combiner) Join(sigs ...crypto.Signature) ([]byte, error) {
	var combined []byte
	for _, sig := range sigs {
		length := make([]byte, 4)
		binary.LittleEndian.PutUint32(length, uint32(len(sig)))
		combined = append(combined, length...)
		combined = append(combined, sig...)
	}
	return combined, nil
}

// Split will split the given byte slice into its signature parts, using the
// embedded length information. If any length is invalid, it will fail.
func (c *Combiner) Split(combined []byte) ([]crypto.Signature, error) {

	var sigs []crypto.Signature
	for next := 0; next < len(combined); {

		// check that we have at least 4 bytes of length information
		remaining := len(combined) - next
		if remaining < 4 {
			return nil, fmt.Errorf("insufficient remaining bytes for length information (remaining: %d, min: %d)", remaining, 4)
		}

		// get the next length information
		length := int(binary.LittleEndian.Uint32(combined[next : next+4]))

		// create the beginning marker for the signature
		from := next + 4
		if from >= len(combined) {
			return nil, fmt.Errorf("invalid from marker for next signature (from: %d, max: %d)", from, len(combined)-1)
		}

		// create the end marker for the signature
		to := from + length
		if to > len(combined) {
			return nil, fmt.Errorf("invalid to marker for next signature (to: %d, max: %d)", to, len(combined))
		}

		// get the signature
		sig := make([]byte, length)
		copy(sig[:], combined[from:to])
		sigs = append(sigs, sig)
		next = next + 4 + length
	}

	return sigs, nil
}
