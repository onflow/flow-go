package fvm

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

// NOTE: these constant *must* match the constants used in the Cadence Crypto contract!
//
const (
	ECDSA_P256      = "ECDSA_P256"
	ECDSA_Secp256k1 = "ECDSA_Secp256k1"
	SHA2_256        = "SHA2_256"
	SHA3_256        = "SHA3_256"
)

type SignatureVerifier interface {
	Verify(
		signature []byte,
		tag []byte,
		message []byte,
		publicKey crypto.PublicKey,
		hashAlgo hash.HashingAlgorithm,
	) (bool, error)
}

type DefaultSignatureVerifier struct{}

func NewDefaultSignatureVerifier() DefaultSignatureVerifier {
	return DefaultSignatureVerifier{}
}

func (DefaultSignatureVerifier) Verify(
	signature []byte,
	tag []byte,
	message []byte,
	publicKey crypto.PublicKey,
	hashAlgo hash.HashingAlgorithm,
) (bool, error) {
	hasher := newHasher(hashAlgo)
	if hasher == nil {
		return false, ErrInvalidHashAlgorithm
	}

	message = append(tag, message...)

	valid, err := publicKey.Verify(signature, message, hasher)
	if err != nil {
		return false, fmt.Errorf("failed to verify signature: %w", err)
	}

	return valid, nil
}

func newHasher(hashAlgo hash.HashingAlgorithm) hash.Hasher {
	switch hashAlgo {
	case hash.SHA2_256:
		return hash.NewSHA2_256()
	case hash.SHA3_256:
		return hash.NewSHA3_256()
	}
	return nil
}

// StringToSigningAlgorithm converts a string to a SigningAlgorithm.
func StringToSigningAlgorithm(s string) crypto.SigningAlgorithm {
	switch s {
	case ECDSA_P256:
		return crypto.ECDSAP256
	case ECDSA_Secp256k1:
		return crypto.ECDSASecp256k1
	default:
		return crypto.UnknownSigningAlgorithm
	}
}

// StringToHashingAlgorithm converts a string to a HashingAlgorithm.
func StringToHashingAlgorithm(s string) hash.HashingAlgorithm {
	switch s {
	case SHA2_256:
		return hash.SHA2_256
	case SHA3_256:
		return hash.SHA3_256
	default:
		return hash.UnknownHashingAlgorithm
	}
}

// verifySignatureFromRuntime is an adapter that performs signature verification using
// raw values provided by the Cadence runtime.
func verifySignatureFromRuntime(
	verifier SignatureVerifier,
	signature []byte,
	rawTag string,
	message []byte,
	rawPublicKey []byte,
	rawSigAlgo string,
	rawHashAlgo string,
) (bool, error) {
	sigAlgo := StringToSigningAlgorithm(rawSigAlgo)
	hashAlgo := StringToHashingAlgorithm(rawHashAlgo)

	publicKey, err := crypto.DecodePublicKey(sigAlgo, rawPublicKey)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return false, err
	}

	tag := parseRuntimeDomainTag(rawTag)
	if tag == nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return false, fmt.Errorf("invalid domain tag")
	}

	valid, err := verifier.Verify(
		signature,
		tag,
		message,
		publicKey,
		hashAlgo,
	)
	if err != nil {
		// TODO: improve error passing https://github.com/onflow/cadence/issues/202
		return false, err
	}

	return valid, nil
}

const runtimeUserDomainTag = "user"

func parseRuntimeDomainTag(tag string) []byte {
	if tag == runtimeUserDomainTag {
		return flow.UserDomainTag[:]
	}

	return nil
}
