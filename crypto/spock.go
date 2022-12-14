package crypto

// SPoCK design based on the BLS signature scheme.
// BLS is using BLS12-381 curve and the same settings in bls.go.

import (
	"github.com/onflow/flow-go/crypto/hash"
)

// SPOCKProve generates a spock poof for data under the private key sk.
//
// The function returns:
//   - (false, nilHasherError) if the hasher is nil
//   - (false, invalidHasherSiseError) if hasher's output size is not 128 bytes
//   - (nil, notBLSKeyError) if input key is not a BLS key
//   - (nil, error) if an unexpected error occurs
//   - (proof, nil) otherwise
func SPOCKProve(sk PrivateKey, data []byte, kmac hash.Hasher) (Signature, error) {
	if sk.Algorithm() != BLSBLS12381 {
		return nil, notBLSKeyError
	}

	// BLS signature of data
	return sk.Sign(data, kmac)
}

// SPOCKVerifyAgainstData verifies a SPoCK proof is generated from the given data
// and the prover's public key.
//
// This is a simple BLS signature verifictaion of the proof under the input data
// and public key.
//
// The function returns:
//   - (false, notBLSKeyError) if input key is not a BLS key
//   - (false, nilHasherError) if the hasher is nil
//   - (false, invalidHasherSiseError) if hasher's output size is not 128 bytes
//   - (false, error) if an unexpected error occurs
//   - (validity, nil) otherwise
func SPOCKVerifyAgainstData(pk PublicKey, proof Signature, data []byte, kmac hash.Hasher) (bool, error) {
	if pk.Algorithm() != BLSBLS12381 {
		return false, notBLSKeyError
	}
	// BLS verification of data
	return pk.Verify(proof, data, kmac)
}
