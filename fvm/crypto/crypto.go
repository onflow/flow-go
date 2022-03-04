package crypto

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/cadence/runtime"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
)

func HashWithTag(hashAlgo hash.HashingAlgorithm, tag string, data []byte) ([]byte, error) {
	var hasher hash.Hasher

	switch hashAlgo {
	case hash.SHA2_256, hash.SHA3_256, hash.SHA2_384, hash.SHA3_384, hash.Keccak_256:
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
	case runtime.HashAlgorithmKECCAK_256:
		return hash.Keccak_256
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
	case hash.Keccak_256:
		return runtime.HashAlgorithmKECCAK_256
	default:
		return runtime.HashAlgorithmUnknown
	}
}

// ValidatePublicKey returns :
// - nil if key is valid and no exception occurred.
// - crypto.invalidInputsError if key is invalid and no exception occurred.
// - panics if an exception occurred.
func ValidatePublicKey(signAlgo runtime.SignatureAlgorithm, pk []byte) error {
	sigAlgo := RuntimeToCryptoSigningAlgorithm(signAlgo)

	_, err := crypto.DecodePublicKey(sigAlgo, pk)

	if err != nil {
		if crypto.IsInvalidInputsError(err) {
			return err
		}
		panic(fmt.Errorf("validate public key failed with unexpected error %w", err))
	}
	return nil
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
		return false, errors.NewValueErrorf(fmt.Sprintf("%d", signatureAlgorithm), "signature algorithm type not found")
	}

	hashAlgo := RuntimeToCryptoHashingAlgorithm(hashAlgorithm)
	if hashAlgo == hash.UnknownHashingAlgorithm {
		return false, errors.NewValueErrorf(fmt.Sprintf("%d", hashAlgorithm), "hashing algorithm type not found")
	}

	// check ECDSA compatibilites
	if sigAlgo == crypto.ECDSAP256 || sigAlgo == crypto.ECDSASecp256k1 {
		// hashing compatibility
		if hashAlgo != hash.SHA2_256 && hashAlgo != hash.SHA3_256 && hashAlgo != hash.Keccak_256 {
			return false, errors.NewValueErrorf(sigAlgo.String(), "cannot use hashing algorithm type %s with signature signature algorithm type %s",
				hashAlgo, sigAlgo)
		}

		// tag length compatibility
		if len(tag) > flow.DomainTagLength {
			return false, errors.NewValueErrorf(tag, "tag length (%d) is larger than max length allowed (%d bytes).", len(tag), flow.DomainTagLength)
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
		return false, errors.NewValueErrorf(hex.EncodeToString(rawPublicKey), "cannot decode public key: %w", err)
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
	case hash.SHA2_256, hash.SHA3_256, hash.Keccak_256:
		var err error
		if hasher, err = NewPrefixedHashing(hashAlgo, tag); err != nil {
			return false, errors.NewValueErrorf(err.Error(), "verification failed")
		}
	case hash.KMAC128:
		hasher = crypto.NewBLSKMAC(tag)
	default:
		return false, errors.NewValueErrorf(fmt.Sprint(hashAlgo), "hashing algorithm type not found")
	}

	valid, err := publicKey.Verify(signature, message, hasher)
	if err != nil {
		// All inputs are guaranteed to be valid at this stage.
		// The check for crypto.InvalidInputs is only a sanity check
		if crypto.IsInvalidInputsError(err) {
			return false, err
		}
		panic(fmt.Errorf("verify signature failed with unexpected error %w", err))
	}

	return valid, nil
}

// VerifyPOP verifies a proof of possession (PoP) for the receiver public key; currently only works for BLS
func VerifyPOP(pk *runtime.PublicKey, s crypto.Signature) (bool, error) {

	key, err := crypto.DecodePublicKey(crypto.BLSBLS12381, pk.PublicKey)
	if err != nil {
		// at this stage, the runtime public key is valid and there are no possible user value errors
		panic(fmt.Errorf("verify PoP failed: runtime BLS public key should be valid %x", pk.PublicKey))
	}

	valid, err := crypto.BLSVerifyPOP(key, s)
	if err != nil {
		// no user errors possible at this stage
		panic(fmt.Errorf("verify PoP failed with unexpected error %w", err))
	}
	return valid, nil
}

// AggregateSignatures aggregate multiple signatures into one; currently only works for BLS
func AggregateSignatures(sigs [][]byte) (crypto.Signature, error) {
	s := make([]crypto.Signature, 0, len(sigs))
	for _, sig := range sigs {
		s = append(s, sig)
	}

	aggregatedSignature, err := crypto.AggregateBLSSignatures(s)
	if err != nil {
		// check for a user error
		if crypto.IsInvalidInputsError(err) {
			return nil, err
		}
		panic(fmt.Errorf("aggregate BLS signatures failed with unexpected error %w", err))
	}
	return aggregatedSignature, nil
}

// AggregatePublicKeys aggregate multiple public keys into one; currently only works for BLS
func AggregatePublicKeys(keys []*runtime.PublicKey) (*runtime.PublicKey, error) {
	pks := make([]crypto.PublicKey, 0, len(keys))
	for _, key := range keys {
		// TODO: avoid validating the public keys again since Cadence makes sure runtime keys have been validated.
		// This requires exporting an unsafe function in the crypto package.
		pk, err := crypto.DecodePublicKey(crypto.BLSBLS12381, key.PublicKey)
		if err != nil {
			// at this stage, the runtime public key is valid and there are no possible user value errors
			panic(fmt.Errorf("aggregate BLS public keys failed: runtime public key should be valid %x", key.PublicKey))
		}
		pks = append(pks, pk)
	}

	pk, err := crypto.AggregateBLSPublicKeys(pks)
	if err != nil {
		// check for a user error
		if crypto.IsInvalidInputsError(err) {
			return nil, err
		}
		panic(fmt.Errorf("aggregate BLS public keys failed with unexpected error %w", err))
	}

	return &runtime.PublicKey{
		PublicKey: pk.Encode(),
		SignAlgo:  CryptoToRuntimeSigningAlgorithm(crypto.BLSBLS12381),
	}, nil
}
