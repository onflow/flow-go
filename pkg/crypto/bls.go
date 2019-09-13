package crypto

// BLS_BLS12381Algo, embeds SignAlgo
type BLS_BLS12381Algo struct {
	// points to Relic context of BLS12-381 with all the parameters
	context ctx
	// embeds SignAlgo
	*SignAlgo
}

// SignHash implements BLS signature on BLS12381 curve
func (a *BLS_BLS12381Algo) SignHash(sk PrKey, h Hash) (Signature, error) {
	hashBytes := h.Bytes()
	return a.SignBytes(sk, hashBytes, nil)
}

// SignBytes signs an array of bytes
// Hasher is not used in the specific case of BLS
func (a *BLS_BLS12381Algo) SignBytes(sk PrKey, data []byte, alg Hasher) (Signature, error) {
	blsPrKey, ok := sk.(*PrKeyBLS_BLS12381)
	if !ok {
		return nil, cryptoError{"BLS sigature can only be called using a BLS private key"}
	}
	skScalar := blsPrKey.scalar
	return a.blsSign(&skScalar, data), nil
}

// SignStruct signs a structure
// Hasher is not used in the specific case of BLS
func (a *BLS_BLS12381Algo) SignStruct(sk PrKey, data Encoder, alg Hasher) (Signature, error) {
	dataBytes := data.Encode()
	return a.SignBytes(sk, dataBytes, nil)
}

// VerifyHash implements BLS signature verification on BLS12381 curve
func (a *BLS_BLS12381Algo) VerifyHash(pk PubKey, s Signature, h Hash) (bool, error) {
	hashBytes := h.Bytes()
	return a.VerifyBytes(pk, s, hashBytes, nil)
}

// VerifyBytes verifies a signature of a byte array
func (a *BLS_BLS12381Algo) VerifyBytes(pk PubKey, s Signature, data []byte, alg Hasher) (bool, error) {
	blsPubKey, ok := pk.(*PubKeyBLS_BLS12381)
	if !ok {
		return false, cryptoError{"BLS signature verification can only be called using a BLS public key"}
	}

	pkPoint := &(blsPubKey.point)
	return a.blsVerify(pkPoint, s, data), nil
}

// VerifyStruct verifies a signature of a structure
func (a *BLS_BLS12381Algo) VerifyStruct(pk PubKey, s Signature, data Encoder, alg Hasher) (bool, error) {
	dataBytes := data.Encode()
	return a.VerifyBytes(pk, s, dataBytes, nil)
}

// GeneratePrKey generates a private key for BLS on BLS12381 curve
func (a *BLS_BLS12381Algo) GeneratePrKey(seed []byte) (PrKey, error) {
	var sk PrKeyBLS_BLS12381
	// seed the RNG
	seedRelic(seed)
	// Generate private key here
	err := randZr(&(sk.scalar))
	if err != nil {
		return nil, err
	}
	// public key is not computed (but this could be changed)
	sk.pk = nil
	// links the private key to the algo
	sk.alg = a
	return &sk, nil
}

// PrKeyBLS_BLS12381 is the private key of BLS using BLS12_381, it implements PrKey
type PrKeyBLS_BLS12381 struct {
	// the signature algo
	alg *BLS_BLS12381Algo
	// public key
	pk *PubKeyBLS_BLS12381
	// private key data
	scalar scalar
}

func (sk *PrKeyBLS_BLS12381) AlgoName() AlgoName {
	return sk.alg.Name()
}

func (sk *PrKeyBLS_BLS12381) Signer() Signer {
	return sk.alg
}

func (sk *PrKeyBLS_BLS12381) KeySize() int {
	return prKeyLengthBLS_BLS12381
}

func (sk *PrKeyBLS_BLS12381) computePubKey() {
	var newPk PubKeyBLS_BLS12381
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
type PubKeyBLS_BLS12381 struct {
	// public key data
	point pointG2
}

func (pk *PubKeyBLS_BLS12381) KeySize() int {
	return pubKeyLengthBLS_BLS12381
}
