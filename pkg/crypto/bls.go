package crypto

import (
	"strconv"
)

// Signature48 implements Signature

func (s *Signature48) ToBytes() []byte {
	return s[:]
}

func (s *Signature48) String() string {
	var res string
	for i := 0; i < len(s); i++ {
		res = strconv.FormatUint(uint64(s[i]), 16) + res
	}
	return "0x" + res
}

// BLS_BLS12381Algo, embeds SignAlgo
type BLS_BLS12381Algo struct {
	// G1 and G2 curves will be added here
	//...
	//...
	*SignAlgo
}

// SignHash implements BLS signature on BLS12381 curve
func (a *BLS_BLS12381Algo) SignHash(k PrKey, h Hash) Signature {
	var s Signature
	return s
}

// VerifyHash implements BLS signature verification on BLS12381 curve
func (a *BLS_BLS12381Algo) VerifyHash(pk PubKey, s Signature, h Hash) bool {
	return true
}

// GeneratePrKey generates a private key for BLS on BLS12381 curve
func (a *BLS_BLS12381Algo) GeneratePrKey() PrKey {
	var sk PrKeyBLS_BLS12381
	// links the private key to the algo
	sk.alg = a
	return &sk
}

// SignBytes signs an array of bytes
func (a *BLS_BLS12381Algo) SignBytes(sk PrKey, data []byte, alg Hasher) Signature {
	h := alg.ComputeBytesHash(data)
	return a.SignHash(sk, h)
}

// SignStruct signs a structure
func (a *BLS_BLS12381Algo) SignStruct(sk PrKey, data Encoder, alg Hasher) Signature {
	h := alg.ComputeStructHash(data)
	return a.SignHash(sk, h)
}

// VerifyBytes verifies a signature of a byte array
func (a *BLS_BLS12381Algo) VerifyBytes(pk PubKey, s Signature, data []byte, alg Hasher) bool {
	h := alg.ComputeBytesHash(data)
	return a.VerifyHash(pk, s, h)
}

// VerifyStruct verifies a signature of a structure
func (a *BLS_BLS12381Algo) VerifyStruct(pk PubKey, s Signature, data Encoder, alg Hasher) bool {
	h := alg.ComputeStructHash(data)
	return a.VerifyHash(pk, s, h)
}

// PrKeyBLS_BLS12381 is the private key of BLS using BLS12_381, it implements PrKey
type PrKeyBLS_BLS12381 struct {
	// the signature algo
	alg *BLS_BLS12381Algo
	// public key
	pk *PubKey_BLS_BLS12381
	// private key data will be entered here
	//...
}

func (sk *PrKeyBLS_BLS12381) AlgoName() AlgoName {
	return sk.alg.Name()
}

func (sk *PrKeyBLS_BLS12381) KeySize() int {
	return PrKeyLengthBLS_BLS12381
}

func (sk *PrKeyBLS_BLS12381) ComputePubKey() {
	// compute sk.pk here
	sk.pk = nil
}

func (sk *PrKeyBLS_BLS12381) Pubkey() PubKey {
	if sk.pk != nil {
		return sk.pk
	}
	sk.ComputePubKey()
	return sk.pk
}

// PubKey_BLS_BLS12381 is the public key of BLS using BLS12_381, it implements PubKey
type PubKey_BLS_BLS12381 struct {
	// public key data  will be entered here
	dummy int
}

func (pk *PubKey_BLS_BLS12381) KeySize() int {
	return PubKeyLengthBLS_BLS12381
}
