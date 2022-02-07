package seed

import (
	"fmt"

	"github.com/onflow/flow-go/consensus/hotstuff/packer"
	"github.com/onflow/flow-go/crypto/hash"
)

// FromParentSignature reads the raw random seed from a main consensus QC sigData.
// The sigData is an RLP encoded structure that is part of QuorumCertificate.
// The indices can be used to generate task-specific seeds from the same signature.
func FromParentSignature(indices []byte, sigData []byte) ([]byte, error) {
	// unpack sig data to extract random beacon sig
	randomBeaconSig, err := packer.UnpackRandomBeaconSig(sigData)
	if err != nil {
		return nil, fmt.Errorf("could not unpack block signature: %w", err)
	}

	return FromRandomSource(indices, randomBeaconSig)
}

// FromRandomSource generates a task-specific seed (task is determined by indices).
func FromRandomSource(customizer []byte, sor []byte) ([]byte, error) {

	// create the key used for the KMAC by concatenating all indices
	keyLen := len(customizer)
	if keyLen < hash.KmacMinKeyLen {
		keyLen = hash.KmacMinKeyLen
	}
	key := make([]byte, keyLen)
	copy(key, customizer)

	// create a KMAC instance with our key and 32 bytes output size
	kmac, err := hash.NewKMAC_128(key, nil, 32)
	if err != nil {
		return nil, fmt.Errorf("could not create kmac: %w", err)
	}

	return kmac.ComputeHash(sor), nil
}
