package flow

// TODO: these functions will be moved to a separate `encoding` package in a future PR

import (
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/onflow/cadence"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
)

// accountPublicKeyWrapper is used for encoding and decoding.
type accountPublicKeyWrapper struct {
	PublicKey []byte
	SignAlgo  uint
	HashAlgo  uint
	Weight    uint
	SeqNumber uint64
	Revoked   bool
}

// legacyAccountPublicKeyWrapper is an RLP wrapper for the old public key format that does
// not contain a Revoked field.
type legacyAccountPublicKeyWrapper struct {
	PublicKey []byte
	SignAlgo  uint
	HashAlgo  uint
	Weight    uint
	SeqNumber uint64
}

// runtimeAccountPublicKeyWrapper must match format used in Transaction
// currently should match SDK AccountKey encoded format
type runtimeAccountPublicKeyWrapper struct {
	PublicKey []byte
	SignAlgo  uint
	HashAlgo  uint
	Weight    uint
}

// accountPrivateKeyWrapper is used for encoding and decoding.
type accountPrivateKeyWrapper struct {
	PrivateKey []byte
	SignAlgo   uint
	HashAlgo   uint
}

func EncodeAccountPublicKey(a AccountPublicKey) ([]byte, error) {
	w := accountPublicKeyWrapper{
		PublicKey: a.PublicKey.Encode(),
		SignAlgo:  uint(a.SignAlgo),
		HashAlgo:  uint(a.HashAlgo),
		Weight:    uint(a.Weight),
		SeqNumber: a.SeqNumber,
		Revoked:   a.Revoked,
	}

	return rlp.EncodeToBytes(&w)
}

func EncodeRuntimeAccountPublicKeys(keys []AccountPublicKey) ([]cadence.Value, error) {
	encodedKeys := make([]cadence.Value, len(keys))
	for i, key := range keys {
		k, err := EncodeRuntimeAccountPublicKey(key)
		if err != nil {
			return nil, err
		}

		values := make([]cadence.Value, len(k))
		for j, v := range k {
			values[j] = cadence.NewUInt8(v)
		}
		encodedKeys[i] = cadence.NewArray(values)
	}

	return encodedKeys, nil
}

func EncodeRuntimeAccountPublicKey(a AccountPublicKey) ([]byte, error) {
	publicKey := a.PublicKey.Encode()

	w := runtimeAccountPublicKeyWrapper{
		PublicKey: publicKey,
		SignAlgo:  uint(a.SignAlgo),
		HashAlgo:  uint(a.HashAlgo),
		Weight:    uint(a.Weight),
	}

	return rlp.EncodeToBytes(&w)
}

func decodeAccountPublicKeyWrapper(b []byte) (accountPublicKeyWrapper, error) {
	var wrapper accountPublicKeyWrapper

	err := rlp.DecodeBytes(b, &wrapper)
	if err != nil {
		// public key data may be stored in legacy format, so convert
		var legacyWrapper legacyAccountPublicKeyWrapper

		err := rlp.DecodeBytes(b, &legacyWrapper)
		if err != nil {
			return accountPublicKeyWrapper{}, err
		}

		return accountPublicKeyWrapper{
			PublicKey: legacyWrapper.PublicKey,
			SignAlgo:  legacyWrapper.SignAlgo,
			HashAlgo:  legacyWrapper.HashAlgo,
			Weight:    legacyWrapper.Weight,
			SeqNumber: legacyWrapper.SeqNumber,
			Revoked:   false, // all legacy keys are not revoked
		}, nil
	}

	return wrapper, nil
}

func DecodeAccountPublicKey(b []byte, index uint64) (AccountPublicKey, error) {
	w, err := decodeAccountPublicKeyWrapper(b)
	if err != nil {
		return AccountPublicKey{}, err
	}

	signAlgo := crypto.SigningAlgorithm(w.SignAlgo)
	hashAlgo := hash.HashingAlgorithm(w.HashAlgo)

	publicKey, err := crypto.DecodePublicKey(signAlgo, w.PublicKey)
	if err != nil {
		return AccountPublicKey{}, err
	}

	return AccountPublicKey{
		Index:     int(index),
		PublicKey: publicKey,
		SignAlgo:  signAlgo,
		HashAlgo:  hashAlgo,
		Weight:    int(w.Weight),
		SeqNumber: w.SeqNumber,
		Revoked:   w.Revoked,
	}, nil
}

// DecodeRuntimeAccountPublicKey decode bytes into AccountPublicKey
// but it is designed to accept byte-format used by Cadence runtime
// (currently same as SDK, but we don't want to keep explicit dependency
// on SDK)
func DecodeRuntimeAccountPublicKey(b []byte, seqNumber uint64) (AccountPublicKey, error) {
	var w runtimeAccountPublicKeyWrapper

	err := rlp.DecodeBytes(b, &w)
	if err != nil {
		return AccountPublicKey{}, err
	}

	signAlgo := crypto.SigningAlgorithm(w.SignAlgo)
	hashAlgo := hash.HashingAlgorithm(w.HashAlgo)

	publicKey, err := crypto.DecodePublicKey(signAlgo, w.PublicKey)
	if err != nil {
		return AccountPublicKey{}, err
	}

	return AccountPublicKey{
		PublicKey: publicKey,
		SignAlgo:  signAlgo,
		HashAlgo:  hashAlgo,
		Weight:    int(w.Weight),
		SeqNumber: seqNumber,
	}, nil
}

func EncodeAccountPrivateKey(a AccountPrivateKey) ([]byte, error) {
	privateKey := a.PrivateKey.Encode()

	w := accountPrivateKeyWrapper{
		PrivateKey: privateKey,
		SignAlgo:   uint(a.SignAlgo),
		HashAlgo:   uint(a.HashAlgo),
	}

	return rlp.EncodeToBytes(&w)
}

func DecodeAccountPrivateKey(b []byte) (AccountPrivateKey, error) {
	var w accountPrivateKeyWrapper

	err := rlp.DecodeBytes(b, &w)
	if err != nil {
		return AccountPrivateKey{}, err
	}

	signAlgo := crypto.SigningAlgorithm(w.SignAlgo)
	hashAlgo := hash.HashingAlgorithm(w.HashAlgo)

	privateKey, err := crypto.DecodePrivateKey(signAlgo, w.PrivateKey)
	if err != nil {
		return AccountPrivateKey{}, err
	}

	return AccountPrivateKey{
		PrivateKey: privateKey,
		SignAlgo:   signAlgo,
		HashAlgo:   hashAlgo,
	}, nil
}
