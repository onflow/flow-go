package crypto

import (
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/sema"

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
	var hasher hash.Hasher

	switch hashAlgo {
	case hash.SHA2_256:
		hasher = hash.NewSHA2_256()
	case hash.SHA3_256:
		hasher = hash.NewSHA3_256()
	case hash.SHA2_384:
		hasher = hash.NewSHA2_384()
	case hash.SHA3_384:
		hasher = hash.NewSHA3_384()
	case hash.KMAC128:
		hasher = crypto.NewBLSKMAC(string(tag))
	default:
		return false, errors.NewValueErrorf(hashAlgo.String(), "hashing algorithm type not found")
	}

	if hashAlgo != hash.KMAC128 {
		message = append(tag, message...)
	}

	valid, err := publicKey.Verify(signature, message, hasher)
	if err != nil {
		return false, fmt.Errorf("failed to verify signature: %w", err)
	}

	return valid, nil
}

func HashWithTag(hashAlgo hash.HashingAlgorithm, tag string, data []byte) ([]byte, error) {
	var hashFunc func(tag string, data []byte) hash.Hash

	shaHashWrapper := func(f func([]byte) hash.Hash) func(tag string, data []byte) hash.Hash {
		return func(tag string, data []byte) hash.Hash {
			message := append([]byte(tag), data...)
			return f(message)
		}
	}

	switch hashAlgo {
	case hash.SHA2_256:
		hashFunc = shaHashWrapper(hash.NewSHA2_256().ComputeHash)
	case hash.SHA3_256:
		hashFunc = shaHashWrapper(hash.NewSHA3_256().ComputeHash)
	case hash.SHA2_384:
		hashFunc = shaHashWrapper(hash.NewSHA2_384().ComputeHash)
	case hash.SHA3_384:
		hashFunc = shaHashWrapper(hash.NewSHA3_384().ComputeHash)
	case hash.KMAC128:
		hashFunc = func(tag string, data []byte) hash.Hash {
			return crypto.NewBLSKMAC(tag).ComputeHash(data)
		}
	default:
		err := errors.NewValueErrorf(hashAlgo.String(), "hashing algorithm type not found")
		return nil, fmt.Errorf("hashing failed: %w", err)
	}

	return hashFunc(tag, data), nil
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

	tag := parseRuntimeDomainTag(rawTag)
	if tag == nil {
		return false, errors.NewValueErrorf(rawTag, "invalid domain tag")
	}

	if len(tag) > 0 && sigAlgo != crypto.BLSBLS12381 && string(tag) != string(flow.UserDomainTag[:]) {
		return false, errors.NewValueErrorf(signatureAlgorithm.Name(), "signature algorithm type %s not supported with tag different than %s", sigAlgo.String(), string(flow.UserDomainTag[:]))
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

//  NewAccountPublicKey construct an account public key given a runtime public key.
func NewAccountPublicKey(publicKey *runtime.PublicKey,
	hashAlgo sema.HashAlgorithm,
	keyIndex int,
	weight int,
) (*flow.AccountPublicKey, error) {
	var err error
	signAlgorithm := RuntimeToCryptoSigningAlgorithm(publicKey.SignAlgo)
	if signAlgorithm == crypto.UnknownSigningAlgorithm {
		err = errors.NewValueErrorf(publicKey.SignAlgo.Name(), "signature algorithm type not found")
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	hashAlgorithm := RuntimeToCryptoHashingAlgorithm(hashAlgo)
	if hashAlgorithm == hash.UnknownHashingAlgorithm {
		err = errors.NewValueErrorf(hashAlgo.Name(), "hashing algorithm type not found")
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	decodedPublicKey, err := crypto.DecodePublicKey(signAlgorithm, publicKey.PublicKey)
	if err != nil {
		err = errors.NewValueErrorf(string(publicKey.PublicKey), "cannot decode public key: %w", err)
		return nil, fmt.Errorf("adding account key failed: %w", err)
	}

	return &flow.AccountPublicKey{
		Index:     keyIndex,
		PublicKey: decodedPublicKey,
		SignAlgo:  signAlgorithm,
		HashAlgo:  hashAlgorithm,
		SeqNumber: 0,
		Weight:    weight,
		Revoked:   false,
	}, nil
}

func parseRuntimeDomainTag(tag string) []byte {
	if tag == runtimeUserDomainTag {
		return flow.UserDomainTag[:]
	}

	return nil
}
