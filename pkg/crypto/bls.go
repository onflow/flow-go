package crypto

// BLS_BLS12381Algo, embeds SignAlgo
type BLS_BLS12381Algo struct {
	// points to Relic context of BLS12-381 with all the parameters
	context ctx
	// embeds SignAlgo
	*SignAlgo
}

func (a *BLS_BLS12381Algo) SignHash(sk PrKey, h Hash) (Signature, error) {
	blsPrKey, ok := sk.(*PrKeyBLS_BLS12381)
	if !ok {
		return nil, cryptoError{"BLS sigature can only be called using a BLS private key"}
	}
	skScalar := blsPrKey.scalar
	return a.blsSign(&skScalar, h), nil
}

// SignBytes signs an array of bytes
// Hasher is not used in the specific case of BLS
func (a *BLS_BLS12381Algo) SignBytes(sk PrKey, data []byte, alg Hasher) (Signature, error) {
	h := alg.ComputeBytesHash(data)
	return a.SignHash(sk, h)
}

// SignStruct signs a structure
// Hasher is not used in the specific case of BLS
func (a *BLS_BLS12381Algo) SignStruct(sk PrKey, data Encoder, alg Hasher) (Signature, error) {
	h := alg.ComputeStructHash(data)
	return a.SignHash(sk, h)
}

// VerifyHash implements BLS signature verification on BLS12381 curve
func (a *BLS_BLS12381Algo) VerifyHash(pk PubKey, s Signature, h Hash) (bool, error) {
	blsPubKey, ok := pk.(*PubKey_BLS_BLS12381)
	if !ok {
		return false, cryptoError{"BLS signature verification can only be called using a BLS public key"}
	}

	pkPoint := &(blsPubKey.point)
	return a.blsVerify(pkPoint, s, h), nil
}

// VerifyBytes verifies a signature of a byte array
func (a *BLS_BLS12381Algo) VerifyBytes(pk PubKey, s Signature, data []byte, alg Hasher) (bool, error) {
	h := alg.ComputeBytesHash(data)
	return a.VerifyHash(pk, s, h)
}

// VerifyStruct verifies a signature of a structure
func (a *BLS_BLS12381Algo) VerifyStruct(pk PubKey, s Signature, data Encoder, alg Hasher) (bool, error) {
	h := alg.ComputeStructHash(data)
	return a.VerifyHash(pk, s, h)
}

// GeneratePrKey generates a private key for BLS on BLS12381 curve
func (a *BLS_BLS12381Algo) GeneratePrKey(seed []byte) (PrKey, error) {
	var sk PrKeyBLS_BLS12381
	// Generate private key here
	err := randZr(&(sk.scalar), seed)
	if err != nil {
		return nil, err
	}
	// public key is not computed (but this could be changed)
	sk.pk = nil
	// links the private key to the algo
	sk.alg = a
	return &sk, nil
}

func (a *BLS_BLS12381Algo) EncodePrKey(PrKey) ([]byte, error) {
	// TODO: implement EncodePrKey
	return nil, nil
}

func (a *BLS_BLS12381Algo) DecodePrKey([]byte) (PrKey, error) {
	// TODO: implement DecodePrKey
	return nil, nil
}

func (a *BLS_BLS12381Algo) EncodePubKey(sk PubKey) ([]byte, error) {
	// TODO: implement EncodePubKey
	return nil, nil
}

func (a *BLS_BLS12381Algo) DecodePubKey([]byte) (PubKey, error) {
	// TODO: implement DecodePubKey
	return nil, nil
}

// PrKeyBLS_BLS12381 is the private key of BLS using BLS12_381, it implements PrKey
type PrKeyBLS_BLS12381 struct {
	// the signature algo
	alg *BLS_BLS12381Algo
	// public key
	pk *PubKey_BLS_BLS12381
	// private key data
	scalar scalar
}

func (sk *PrKeyBLS_BLS12381) AlgoName() AlgoName {
	return sk.alg.Name()
}

func (sk *PrKeyBLS_BLS12381) KeySize() int {
	return prKeyLengthBLS_BLS12381
}

func (sk *PrKeyBLS_BLS12381) computePubKey() {
	var newPk PubKey_BLS_BLS12381
	// compute public key pk = g2^sk
	_G2scalarGenMult(&(newPk.point), &(sk.scalar))
	sk.pk = &newPk
}

func (sk *PrKeyBLS_BLS12381) Pubkey() PubKey {
	if sk.pk != nil {
		return sk.pk
	}
	sk.computePubKey()
	return sk.pk
}

// PubKey_BLS_BLS12381 is the public key of BLS using BLS12_381, it implements PubKey
type PubKey_BLS_BLS12381 struct {
	// public key data
	point pointG2
}

func (pk *PubKey_BLS_BLS12381) KeySize() int {
	return pubKeyLengthBLS_BLS12381
}
