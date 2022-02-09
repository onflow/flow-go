package seed

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/packer"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/crypto/random"
)

// FromParentSignature extracts a seed from the given QC sigData.
// The sigData is an RLP encoded structure that is part of QuorumCertificate.
//
// The function extracts first the source of randomness from sigData and then hashes
// it to obtain the outout seed.
// Since the source of randmoness is a cryptographic signature, it is required to
// hash it in order to uniformize the entropy over the output.
func FromParentSignature(sigData []byte) ([]byte, error) {
	// unpack sig data to extract random beacon sig
	randomBeaconSig, err := packer.UnpackRandomBeaconSig(sigData)
	if err != nil {
		return nil, fmt.Errorf("could not unpack block signature: %w", err)
	}

	return FromRandomSource(randomBeaconSig), nil
}

// FromRandomSource generates a seed from source of randomness.
//
// The source of randomness is a cryptographic signature, it is therefore required to
// hash the data in order to uniformize the entropy over the output seed.
func FromRandomSource(randomSource []byte) []byte {
	var seed [hash.HashLenSHA3_256]byte
	hash.ComputeSHA3_256(&seed, randomSource)
	return seed[:]
}

func PRGFromRandomSource(randomSource []byte, customizer []byte) (random.Rand, error) {
	// hash the source of randomness (signature) to uniformize the entropy
	var seed [hash.HashLenSHA3_256]byte
	hash.ComputeSHA3_256(&seed, randomSource)

	// create random number generator from the seed and customizer
	rng, err := random.NewChacha20PRG(seed[:], customizer)
	if err != nil {
		return nil, fmt.Errorf("could not create rng: %w", err)
	}
	return rng, nil
}
