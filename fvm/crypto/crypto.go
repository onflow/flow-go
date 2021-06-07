package crypto

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
		tag string,
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
	tag string,
	message []byte,
	publicKey crypto.PublicKey,
	hashAlgo hash.HashingAlgorithm,
) (bool, error) {

	var hasher hash.Hasher

	switch hashAlgo {
	case hash.SHA2_256:
		fallthrough
	case hash.SHA3_256:
		var err error
		if hasher, err = NewPrefixedHashing(hashAlgo, tag); err != nil {
			return false, errors.NewValueErrorf(err.Error(), "verification failed")
		}
	case hash.KMAC128:
		hasher = crypto.NewBLSKMAC(tag)
	default:
		return false, errors.NewValueErrorf(hashAlgo.String(), "hashing algorithm type not found")
	}

	valid, err := publicKey.Verify(signature, message, hasher)
	if err != nil {
		return false, fmt.Errorf("failed to verify signature: %w", err)
	}

	return valid, nil
}

func HashWithTag(hashAlgo hash.HashingAlgorithm, tag string, data []byte) ([]byte, error) {
	var hasher hash.Hasher

	switch hashAlgo {
	case hash.SHA2_256:
		fallthrough
	case hash.SHA3_256:
		fallthrough
	case hash.SHA2_384:
		fallthrough
	case hash.SHA3_384:
		var err error
		if hasher, err = NewPrefixedHashing(hashAlgo, tag); err != nil {
			return nil, errors.NewValueErrorf(err.Error(), "verification failed")
		}
	case hash.KMAC128:
		hasher = crypto.NewBLSKMAC(tag)
	default:
		err := errors.NewValueErrorf(fmt.Sprint(hashAlgo), "hashing algorithm type not found")
		return nil, fmt.Errorf("hashing failed: %w", err)
	}

	return hasher.ComputeHash(data), nil
}

// RuntimeToCryptoSigningAlgorithm converts a runtime signature algorithm to a crypto signature algorithm.
func RuntimeToCryptoSigningAlgorithm(s runtime.SignatureAlgorithm) crypto.SigningAlgorithm {
	switch s {
	case runtime.SignatureAlgorithmECDSA_P256:
		return crypto.ECDSAP256
	case runtime.SignatureAlgorithmECDSA_secp256k1:
		return crypto.ECDSASecp256k1
	case runtime.SignatureAlgorithmBLS_BLS12_381:
		return crypto.BLSBLS12381
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
	case crypto.BLSBLS12381:
		return runtime.SignatureAlgorithmBLS_BLS12_381
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
	case runtime.HashAlgorithmKMAC128_BLS_BLS12_381:
		return hash.KMAC128
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
	case hash.KMAC128:
		return runtime.HashAlgorithmKMAC128_BLS_BLS12_381
	default:
		return runtime.HashAlgorithmUnknown
	}
}

// ValidatePublicKey returns true if public key is valid
func ValidatePublicKey(signAlgo runtime.SignatureAlgorithm, pk []byte) (valid bool, err error) {
	sigAlgo := RuntimeToCryptoSigningAlgorithm(signAlgo)

	_, err = crypto.DecodePublicKey(sigAlgo, pk)

	if err != nil {
		if crypto.IsInvalidInputsError(err) {
			return false, nil
		}
		return false, fmt.Errorf("validate public key failed: %w", err)
	}
	return true, nil
}

// VerifySignatureFromRuntime is an adapter that performs signature verification using
// raw values provided by the Cadence runtime.
func VerifySignatureFromRuntime(
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

	tag := rawTag
	// if the tag is `runtimeUserDomainTag` replace it with `flow.UserDomainTag`
	// this is for backwards compatibility. In the future cadence should send `flow.UserDomainTag` directly
	if tag == runtimeUserDomainTag {
		tag = string(flow.UserDomainTag[:])
	}

	if (sigAlgo == crypto.ECDSAP256 || sigAlgo == crypto.ECDSASecp256k1) && tag != string(flow.UserDomainTag[:]) {
		return false, errors.NewValueErrorf(signatureAlgorithm.Name(), "signature algorithm type %s not supported with tag different than %s", sigAlgo.String(), string(flow.UserDomainTag[:]))
	}

	// HashAlgorithmKMAC128_BLS_BLS12_381 and SignatureAlgorithmBLS_BLS12_381 only function with each other
	if (signatureAlgorithm == runtime.SignatureAlgorithmBLS_BLS12_381) != (hashAlgorithm == runtime.HashAlgorithmKMAC128_BLS_BLS12_381) {
		return false, errors.NewValueErrorf(hashAlgorithm.Name(), "cannot use hashing algorithm type %s with signature signature algorithm type %s", hashAlgorithm.String(), signatureAlgorithm.String())
	}

	hashAlgo := RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	if hashAlgo == hash.UnknownHashingAlgorithm {
		return false, errors.NewValueErrorf(hashAlgorithm.Name(), "hashing algorithm type not found")
	}

	publicKey, err := crypto.DecodePublicKey(sigAlgo, rawPublicKey)
	if err != nil {
		return false, errors.NewValueErrorf(string(rawPublicKey), "cannot decode public key: %w", err)
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
