// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/pkg/errors"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
)

// Account represents an account on the Flow network.
//
// An account can be an externally owned account or a contract account with code.
type Account struct {
	Address   Address
	Balance   uint64
	Keys      []AccountPublicKey
	Contracts map[string][]byte
}

// AccountPublicKey is a public key associated with an account.
//
// An account public key contains the public key, signing and hashing algorithms, and a key weight.
type AccountPublicKey struct {
	Index     int
	PublicKey crypto.PublicKey
	SignAlgo  crypto.SigningAlgorithm
	HashAlgo  hash.HashingAlgorithm
	SeqNumber uint64
	Weight    int
	Revoked   bool
}

func (a AccountPublicKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PublicKey []byte
		SignAlgo  crypto.SigningAlgorithm
		HashAlgo  hash.HashingAlgorithm
		SeqNumber uint64
		Weight    int
	}{
		a.PublicKey.Encode(),
		a.SignAlgo,
		a.HashAlgo,
		a.SeqNumber,
		a.Weight,
	})
}

func (a *AccountPublicKey) UnmarshalJSON(data []byte) error {
	temp := struct {
		PublicKey []byte
		SignAlgo  crypto.SigningAlgorithm
		HashAlgo  hash.HashingAlgorithm
		SeqNumber uint64
		Weight    int
	}{}
	err := json.Unmarshal(data, &temp)
	if err != nil {
		return err
	}
	if a == nil {
		a = new(AccountPublicKey)
	}
	a.PublicKey, err = crypto.DecodePublicKey(temp.SignAlgo, temp.PublicKey)
	if err != nil {
		return err
	}
	a.SignAlgo = temp.SignAlgo
	a.HashAlgo = temp.HashAlgo
	a.SeqNumber = temp.SeqNumber
	a.Weight = temp.Weight
	return nil
}

func (a AccountPublicKey) CompactEncodeWithDefaultVersion() ([]byte, error) {
	return a.compactEncodeV0()
}

func (a AccountPublicKey) CompactEncode(version uint8) ([]byte, error) {
	switch version {
	case 0:
		return a.compactEncodeV0()
	default:
		return nil, fmt.Errorf("cannot encode account public key: encoding version %d is not supported", version)
	}
}

func (a *AccountPublicKey) CompactDecode(data []byte) error {
	// read version and redirect
	// first 5 bits are encoding version
	b := uint8(data[0]) & byte(248)  // first 5 bits are encoding version
	encodingVersion := uint8(b) >> 3 // (mask of 11111000 and left shift 3)

	switch encodingVersion {
	case 0:
		return a.compactDecodeV0(data)
	default:
		return fmt.Errorf("cannot decode account public key: encoding version %d is not supported", encodingVersion)
	}
}

func (a *AccountPublicKey) compactDecodeV0(data []byte) error {

	dataSize := len(data)
	minDataSize := 15
	// we need at least minDataSize bytes
	if dataSize < minDataSize {
		return fmt.Errorf("cannot compact decode public key: data size is too small to be decodable (expected: at least %d, got: %d)", minDataSize, dataSize)
	}

	// first 5 bits were encoding version so we have already processed them

	// next bit is revoked flag
	a.Revoked = (data[0] & byte(4)) > 0 // (mask of 00000100 and equality check (might optimize in the future))

	// the last 2 remaining bits of first byte are the most significant bits of key weight
	a.Weight = int(uint16(data[0]&byte(3))<<8 + uint16(data[1]))

	// right now is omitted for speed reasons and since user has no control over these values

	// the next byte is the signing algorithm
	a.SignAlgo = crypto.SigningAlgorithm(uint8(data[2]))

	// the next byte is the hashing algorithm
	a.HashAlgo = hash.HashingAlgorithm(uint8(data[3]))

	// the next 8 bytes are sequence number
	a.SeqNumber = binary.BigEndian.Uint64(data[4:12])

	// the next 2 bytes captures the size of the encoded public key
	publicKeyLen := binary.BigEndian.Uint16(data[12:14])

	allowedDataSize := publicKeyLen + 14
	// to stay canonical if there are extra bytes we return error
	if dataSize != int(allowedDataSize) {
		return fmt.Errorf("cannot compact decode public key: encoded public key size doesn't match (expected: %d, got: %d)", allowedDataSize, dataSize)
	}

	PublicKey, err := crypto.DecodePublicKey(a.SignAlgo, data[14:allowedDataSize])
	if err != nil {
		return fmt.Errorf("cannot compact decode public key: %w", err)
	}

	a.PublicKey = PublicKey
	return nil
}

func (a AccountPublicKey) compactEncodeV0() ([]byte, error) {
	var encoded [14]byte

	// first 5 bits	are encoding version
	// you might set it initial int according to the version
	// e.g. uint8(8) for version 1, uint8(16) for v2, uint8(24) for v3, ...
	firstByte := uint8(0)
	// next bit is set flag for revoked key
	if a.Revoked {
		firstByte = firstByte | uint8(4)
	}

	if a.Weight >= 1024 {
		return nil, fmt.Errorf("cannot compact encode account public key: key weight larger than 1023")
	}
	// rest bits of first byte are the most significant bits of key weight
	encoded[0] = firstByte | uint8(uint16(a.Weight)>>8)
	encoded[1] = uint8(a.Weight)

	// TODO (maybe) add checks for enforcing uint8 of signing algo and hashing algo
	// right now is omitted for speed reasons and since user has no control over these values
	// the next byte is the signing algorithm
	encoded[2] = uint8(a.SignAlgo)

	// the next byte is the hashing algorithm
	encoded[3] = uint8(a.HashAlgo)

	// the next 8 bytes captures sequence number
	binary.BigEndian.PutUint64(encoded[4:12], a.SeqNumber)

	encodedPublicKey := a.PublicKey.Encode()

	if len(encodedPublicKey) >= 65536 {
		return nil, fmt.Errorf("cannot compact encode account public key: encoded public key size is larger than 65535 bytes")
	}
	binary.BigEndian.PutUint16(encoded[12:14], uint16(len(encodedPublicKey)))

	// and append the encoded public key as the rest of the bytes
	return append(encoded[:], encodedPublicKey...), nil
}

// Validate returns an error if this account key is invalid.
//
// An account key can be invalid for the following reasons:
// - It specifies an incompatible signature/hash algorithm pairing
// - (TODO) It specifies a negative key weight
func (a AccountPublicKey) Validate() error {
	if !CompatibleAlgorithms(a.SignAlgo, a.HashAlgo) {
		return errors.Errorf(
			"signing algorithm (%s) is incompatible with hashing algorithm (%s)",
			a.SignAlgo,
			a.HashAlgo,
		)
	}
	return nil
}

// AccountPrivateKey is a private key associated with an account.
type AccountPrivateKey struct {
	PrivateKey crypto.PrivateKey
	SignAlgo   crypto.SigningAlgorithm
	HashAlgo   hash.HashingAlgorithm
}

// PublicKey returns a weighted public key.
func (a AccountPrivateKey) PublicKey(weight int) AccountPublicKey {
	return AccountPublicKey{
		PublicKey: a.PrivateKey.PublicKey(),
		SignAlgo:  a.SignAlgo,
		HashAlgo:  a.HashAlgo,
		Weight:    weight,
	}
}

func (a AccountPrivateKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		PrivateKey []byte
		SignAlgo   crypto.SigningAlgorithm
		HashAlgo   hash.HashingAlgorithm
	}{
		a.PrivateKey.Encode(),
		a.SignAlgo,
		a.HashAlgo,
	})
}

// CompatibleAlgorithms returns true if the signature and hash algorithms are compatible.
func CompatibleAlgorithms(sigAlgo crypto.SigningAlgorithm, hashAlgo hash.HashingAlgorithm) bool {
	switch sigAlgo {
	case crypto.ECDSAP256, crypto.ECDSASecp256k1:
		switch hashAlgo {
		case hash.SHA2_256, hash.SHA3_256:
			return true
		}
	}
	return false
}
