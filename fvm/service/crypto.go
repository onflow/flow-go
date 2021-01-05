package service

import (
	"fmt"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
)

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
		return false, fmt.Errorf("invalid hashing algorithm: %s", hashAlgo)
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
	case crypto.ECDSAP256.String():
		return crypto.ECDSAP256
	case crypto.ECDSASecp256k1.String():
		return crypto.ECDSASecp256k1
	default:
		return crypto.UnknownSigningAlgorithm
	}
}

// StringToHashingAlgorithm converts a string to a HashingAlgorithm.
func StringToHashingAlgorithm(s string) hash.HashingAlgorithm {
	switch s {
	case hash.SHA2_256.String():
		return hash.SHA2_256
	case hash.SHA3_256.String():
		return hash.SHA3_256
	default:
		return hash.UnknownHashingAlgorithm
	}
}

// TODO Ramtin (make this part of the service and pass to env)
// verifySignatureFromRuntime is an adapter that performs signature verification using
// raw values provided by the Cadence runtime.
func (verifier DefaultSignatureVerifier) VerifySignatureFromRuntime(
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
