package module

import (
	"github.com/onflow/flow-go/crypto"
)

// Merger is responsible for combining two signatures, but it must be done
// in a cryptographically unaware way (agnostic of the byte structure of the
// signatures). However, the merger has an internal notion of the expected
// length of each signature and errors in case the inputs have incompatible
// length.
type Merger interface {
	// Join concatenates the provided signatures. The merger has an internal notion
	// of the expected length of each signature. It returns the sentinel error
	// `verification.ErrInvalidFormat` if one of the signatures does not conform
	// to the expected byte length.
	Join(sig1, sig2 crypto.Signature) ([]byte, error)
	// Split separates the concatenated signature into its two components. The
	// merger has an internal notion of the expected byte length of each
	// signature. It returns the sentinel error `verification.ErrInvalidFormat`
	// if either signatures does not conform to the expected length.
	Split(combined []byte) (crypto.Signature, crypto.Signature, error)
}
