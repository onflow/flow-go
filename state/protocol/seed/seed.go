package seed

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/packer"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/crypto/random"
)

// PRGFromRandomSource returns a PRG seeded by the source of randomness of the protocol.
// The customizer is used to generate a task-specific PRG (customizer in this implementation
// is up to 12-bytes long).
//
// The function hashes the input random source to obtain the PRG seed.
// Hashing is required to uniformize the entropy over the output.
func PRGFromRandomSource(randomSource []byte, customizer []byte) (random.Rand, error) {
	// hash the source of randomness (signature) to uniformize the entropy
	var seed [hash.HashLenSHA3_256]byte
	hash.ComputeSHA3_256(&seed, randomSource)

	// create random number generator from the seed and customizer
	rng, err := random.NewChacha20PRG(seed[:], customizer)
	if err != nil {
		return nil, fmt.Errorf("could not create ChaCha20 PRG: %w", err)
	}
	return rng, nil
}

const RandomSourceLength = crypto.SignatureLenBLSBLS12381

// FromParentQCSignature extracts the source of randomness from the given QC sigData.
// The sigData is an RLP encoded structure that is part of QuorumCertificate.
func FromParentQCSignature(sigData []byte) ([]byte, error) {
	// unpack sig data to extract random beacon sig
	randomBeaconSig, err := packer.UnpackRandomBeaconSig(sigData)
	if err != nil {
		return nil, fmt.Errorf("could not unpack block signature: %w", err)
	}

	return randomBeaconSig, nil
}
