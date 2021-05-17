package fvm

import (
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

const runtimeUserDomainTag = "user"

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
		return false, errors.NewValueErrorf(hashAlgo.String(), "hashing algorithm type not found")
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
	case hash.SHA2_384:
		return hash.NewSHA2_384()
	case hash.SHA3_384:
		return hash.NewSHA3_384()
	}
	return nil
}

// RuntimeToCryptoSigningAlgorithm converts a runtime signature algorithm to a crypto signature algorithm.
func RuntimeToCryptoSigningAlgorithm(s runtime.SignatureAlgorithm) crypto.SigningAlgorithm {
	switch s {
	case runtime.SignatureAlgorithmECDSA_P256:
		return crypto.ECDSAP256
	case runtime.SignatureAlgorithmECDSA_secp256k1:
		return crypto.ECDSASecp256k1
	default:
		return crypto.UnknownSigningAlgorithm
	}
}

// CryptoToRuntimeSigningAlgorithm converts a crypto signature algorithm to a runtime signature algorithm.
func CryptoToRuntimeSigningAlgorithm(s crypto.SigningAlgorithm) runtime.SignatureAlgorithm {
	switch s {
	case crypto.ECDSAP256:
		return runtime.SignatureAlgorithmECDSA_P256
	case crypto.ECDSASecp256k1:
		return runtime.SignatureAlgorithmECDSA_secp256k1
	default:
		return runtime.SignatureAlgorithmUnknown
	}
}

// RuntimeToCryptoHashingAlgorithm converts a runtime hash algorithm to a crypto hashing algorithm.
func RuntimeToCryptoHashingAlgorithm(s runtime.HashAlgorithm) hash.HashingAlgorithm {
	switch s {
	case runtime.HashAlgorithmSHA2_256:
		return hash.SHA2_256
	case runtime.HashAlgorithmSHA3_256:
		return hash.SHA3_256
	case runtime.HashAlgorithmSHA2_384:
		return hash.SHA2_384
	case runtime.HashAlgorithmSHA3_384:
		return hash.SHA3_384
	default:
		return hash.UnknownHashingAlgorithm
	}
}

// CryptoToRuntimeHashingAlgorithm converts a crypto hashing algorithm to a runtime hash algorithm.
func CryptoToRuntimeHashingAlgorithm(h hash.HashingAlgorithm) runtime.HashAlgorithm {
	switch h {
	case hash.SHA2_256:
		return runtime.HashAlgorithmSHA2_256
	case hash.SHA3_256:
		return runtime.HashAlgorithmSHA3_256
	case hash.SHA2_384:
		return runtime.HashAlgorithmSHA2_384
	case hash.SHA3_384:
		return runtime.HashAlgorithmSHA3_384
	default:
		return runtime.HashAlgorithmUnknown
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
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (bool, error) {
	sigAlgo := RuntimeToCryptoSigningAlgorithm(signatureAlgorithm)
	if sigAlgo == crypto.UnknownSigningAlgorithm {
		return false, errors.NewValueErrorf(signatureAlgorithm.Name(), "signature algorithm type not found")
	}
	if sigAlgo == crypto.BLSBLS12381 {
		return false, errors.NewValueErrorf(signatureAlgorithm.Name(), "signature algorithm type %s not supported", crypto.BLSBLS12381.String())
	}

	hashAlgo := RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	if hashAlgo == hash.UnknownHashingAlgorithm {
		return false, errors.NewValueErrorf(hashAlgorithm.Name(), "hashing algorithm type not found")
	}
	if hashAlgo == hash.KMAC128 {
		return false, errors.NewValueErrorf(signatureAlgorithm.Name(), "hashing algorithm %s not supported", hash.KMAC128.String())
	}

	publicKey, err := crypto.DecodePublicKey(sigAlgo, rawPublicKey)
	if err != nil {
		return false, errors.NewValueErrorf(string(rawPublicKey), "cannot decode public key: %w", err)
	}

	tag := parseRuntimeDomainTag(rawTag)
	if tag == nil {
		return false, errors.NewValueErrorf(string(rawTag), "invalid domain tag")
	}

	valid, err := verifier.Verify(
		signature,
		tag,
		message,
		publicKey,
		hashAlgo,
	)
	if err != nil {
		return false, err
	}

	return valid, nil
}

func parseRuntimeDomainTag(tag string) []byte {
	if tag == runtimeUserDomainTag {
		return flow.UserDomainTag[:]
	}

	return nil
}
