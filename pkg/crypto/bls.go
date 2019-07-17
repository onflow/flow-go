package crypto

import (
	"strconv"
)

// Signature48 implements Signature
//----------------------
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
//-----------------------------------------------
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

// PrKeyBLS_BLS12381 is the private key of BLS using BLS12_381, it implements PrKey
//----------------------
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
//----------------------
type PubKey_BLS_BLS12381 struct {
	// public key data  will be entered here
	dummy int
}

func (pk *PubKey_BLS_BLS12381) KeySize() int {
	return PubKeyLengthBLS_BLS12381
}
