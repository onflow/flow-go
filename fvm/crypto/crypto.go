package crypto

import (
	"fmt"
	"strings"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

func HashWithTag(hashAlgo hash.HashingAlgorithm, tag string, data []byte) ([]byte, error) {
	var hasher hash.Hasher

	switch hashAlgo {
	case hash.SHA2_256, hash.SHA3_256, hash.SHA2_384, hash.SHA3_384:
		var err error
		if hasher, err = NewPrefixedHashing(hashAlgo, tag); err != nil {
			return nil, errors.NewValueErrorf(err.Error(), "verification failed")
		}
	case hash.KMAC128:
		hasher = NewBLSKMAC(tag)
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
	tag string,
	message []byte,
	rawPublicKey []byte,
	signatureAlgorithm runtime.SignatureAlgorithm,
	hashAlgorithm runtime.HashAlgorithm,
) (bool, error) {

	sigAlgo := RuntimeToCryptoSigningAlgorithm(signatureAlgorithm)
	if sigAlgo == crypto.UnknownSigningAlgorithm {
		return false, errors.NewValueErrorf(signatureAlgorithm.Name(), "signature algorithm type not found")
	}

	hashAlgo := RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	if hashAlgo == hash.UnknownHashingAlgorithm {
		return false, errors.NewValueErrorf(hashAlgorithm.Name(), "hashing algorithm type not found")
	}

	// check ECDSA compatibilites
	if sigAlgo == crypto.ECDSAP256 || sigAlgo == crypto.ECDSASecp256k1 {
		// hashing compatibility
		if hashAlgo != hash.SHA2_256 && hashAlgo != hash.SHA3_256 {
			return false, errors.NewValueErrorf(sigAlgo.String(), "cannot use hashing algorithm type %s with signature signature algorithm type %s",
				hashAlgo, sigAlgo)
		}

		// tag compatibility
		if !tagECDSACheck(tag) {
			return false, errors.NewValueErrorf(sigAlgo.String(), "tag %s is not supported", tag)
		}

		// check BLS compatibilites
	} else if sigAlgo == crypto.BLSBLS12381 && hashAlgo != hash.KMAC128 {
		// hashing compatibility
		return false, errors.NewValueErrorf(sigAlgo.String(), "cannot use hashing algorithm type %s with signature signature algorithm type %s",
			hashAlgo, sigAlgo)
		// there are no tag constraints
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

// check compatible tags with ECDSA
//
// Only tags with a prefix flow.UserTagString and zero paddings are accepted.
func tagECDSACheck(tag string) bool {

	if len(tag) > flow.DomainTagLength ||
		!strings.HasPrefix(tag, flow.UserTagString) {

		return false
	}

	// check the remaining bytes are zeros
	remaining := tag[len(flow.UserTagString):]
	for _, b := range []byte(remaining) {
		if b != 0 {
			return false
		}
	}

	return true
}

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
	case hash.SHA2_256, hash.SHA3_256:
		var err error
		if hasher, err = NewPrefixedHashing(hashAlgo, tag); err != nil {
			return false, errors.NewValueErrorf(err.Error(), "verification failed")
		}
	case hash.KMAC128:
		hasher = NewBLSKMAC(tag)
	default:
		return false, errors.NewValueErrorf(fmt.Sprint(hashAlgo), "hashing algorithm type not found")
	}

	valid, err := publicKey.Verify(signature, message, hasher)
	if err != nil {
		return false, fmt.Errorf("failed to verify signature: %w", err)
	}

	return valid, nil
}
