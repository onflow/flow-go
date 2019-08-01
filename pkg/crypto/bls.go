package crypto

// BLS_BLS12381Algo, embeds SignAlgo
type BLS_BLS12381Algo struct {
	// points to Relic context of BLS12-381 with all the parameters
	context ctx
	// embeds SignAlgo
	*SignAlgo
}

// SignHash implements BLS signature on BLS12381 curve
func (a *BLS_BLS12381Algo) SignHash(k PrKey, h Hash) Signature {
	var s Signature
	s = make([]byte, a.SignatureLength)
	// signature computation here
	return s
}

// VerifyHash implements BLS signature verification on BLS12381 curve
func (a *BLS_BLS12381Algo) VerifyHash(pk PubKey, s Signature, h Hash) bool {
	// signature verification here
	return true
}

// GeneratePrKey generates a private key for BLS on BLS12381 curve
func (a *BLS_BLS12381Algo) GeneratePrKey(seed []byte) PrKey {
	var sk PrKeyBLS_BLS12381
	// Generate private key here
	randZr(&(sk.sk), seed)
	//(&(sk.sk)).setInt(1)
	// public key is not computed (but this could be changed)
	sk.pk = nil
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
	// private key data
	sk scalar
}

func (sk *PrKeyBLS_BLS12381) AlgoName() AlgoName {
	return sk.alg.Name()
}

func (sk *PrKeyBLS_BLS12381) KeySize() int {
	return PrKeyLengthBLS_BLS12381
}

func (sk *PrKeyBLS_BLS12381) ComputePubKey() {
	var newPk PubKey_BLS_BLS12381
	// compute public key pk = g2^sk
	_G2scalarGenMult(&(newPk.pk), &(sk.sk))
	sk.pk = &newPk
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
	// public key data will be entered here
	pk pointG2
}

func (pk *PubKey_BLS_BLS12381) KeySize() int {
	return PubKeyLengthBLS_BLS12381
}
