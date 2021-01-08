package seed

import (
	"encoding/binary"
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/module/signature"
)

// FromParentSignature reads the raw random seed from a combined signature.
// the combinedSig must be from a QuorumCertificate. The indices can be used to
// generate task-specific seeds from the same signature.
func FromParentSignature(indices []uint32, combinedSig crypto.Signature) ([]byte, error) {
	// split the parent voter sig into staking & beacon parts
	combiner := signature.NewCombiner()
	sigs, err := combiner.Split(combinedSig)
	if err != nil {
		return nil, fmt.Errorf("could not split block signature: %w", err)
	}
	if len(sigs) != 2 {
		return nil, fmt.Errorf("invalid block signature split")
	}

	return FromRandomSource(indices, sigs[1])
}

// FromRandomSource generates a task-specific seed (task is determined by indices).
func FromRandomSource(indices []uint32, sor []byte) ([]byte, error) {

	// create the key used for the KMAC by concatenating all indices
	keyLen := 4 * len(indices)
	if keyLen < hash.KmacMinKeyLen {
		keyLen = hash.KmacMinKeyLen
	}
	key := make([]byte, keyLen)
	for i, index := range indices {
		binary.LittleEndian.PutUint32(key[4*i:4*i+4], index)
	}

	// create a KMAC instance with our key and 32 bytes output size
	kmac, err := hash.NewKMAC_128(key, nil, 32)
	if err != nil {
		return nil, fmt.Errorf("could not create kmac: %w", err)
	}

	return kmac.ComputeHash(sor), nil
}
