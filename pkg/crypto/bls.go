package crypto

// BLS_BLS12381Algo, embeds SignAlgo
type BLS_BLS12381Algo struct {
	// points to Relic context of BLS12-381 with all the parameters
	context ctx
	// embeds SignAlgo
	*SignAlgo
}

// SignHash implements BLS signature on BLS12381 curve
func (a *BLS_BLS12381Algo) SignHash(sk PrKey, h Hash) Signature {
	hashBytes := h.ToBytes()
	return a.SignBytes(sk, hashBytes, nil)
}

// SignBytes signs an array of bytes
// Hasher is not used in the specific case of BLS
func (a *BLS_BLS12381Algo) SignBytes(sk PrKey, data []byte, alg Hasher) Signature {
	skScalar := (sk.(*PrKeyBLS_BLS12381)).sk
	return a.blsSign(&skScalar, data)
}

// SignStruct signs a structure
// Hasher is not used in the specific case of BLS
func (a *BLS_BLS12381Algo) SignStruct(sk PrKey, data Encoder, alg Hasher) Signature {
	dataBytes := data.Encode()
	return a.SignBytes(sk, dataBytes, nil)
}

// VerifyHash implements BLS signature verification on BLS12381 curve
func (a *BLS_BLS12381Algo) VerifyHash(pk PubKey, s Signature, h Hash) bool {
	hashBytes := h.ToBytes()
	return a.VerifyBytes(pk, s, hashBytes, nil)
}

// VerifyBytes verifies a signature of a byte array
func (a *BLS_BLS12381Algo) VerifyBytes(pk PubKey, s Signature, data []byte, alg Hasher) bool {
	pkPoint := &((pk.(*PubKey_BLS_BLS12381)).pk)
	return a.blsVerify(pkPoint, s, data)
}

// VerifyStruct verifies a signature of a structure
func (a *BLS_BLS12381Algo) VerifyStruct(pk PubKey, s Signature, data Encoder, alg Hasher) bool {
	dataBytes := data.Encode()
	return a.VerifyBytes(pk, s, dataBytes, nil)
}

// GeneratePrKey generates a private key for BLS on BLS12381 curve
func (a *BLS_BLS12381Algo) GeneratePrKey(seed []byte) PrKey {
	var sk PrKeyBLS_BLS12381
	// Generate private key here
	randZr(&(sk.sk), seed)
	(&(sk.sk)).setInt(7)
	// public key is not computed (but this could be changed)
	sk.pk = nil
	// links the private key to the algo
	sk.alg = a
	return &sk
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
	return prKeyLengthBLS_BLS12381()
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
	return pubKeyLengthBLS_BLS12381()
}
