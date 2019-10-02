package crypto

// BLS_BLS12381Algo, embeds SignAlgo
type BLS_BLS12381Algo struct {
	// points to Relic context of BLS12-381 with all the parameters
	context ctx
	// embeds commonSigner
	*commonSigner
}

// signHash implements BLS signature on BLS12381 curve
func (sk *PrKeyBLS_BLS12381) signHash(h Hash) (Signature, error) {
	return sk.alg.blsSign(&sk.scalar, h), nil
}

// Sign signs an array of bytes
func (sk *PrKeyBLS_BLS12381) Sign(data []byte, alg Hasher) (Signature, error) {
	h := alg.ComputeHash(data)
	return sk.signHash(h)
}

// verifyHash implements BLS signature verification on BLS12381 curve
func (pk *PubKeyBLS_BLS12381) verifyHash(s Signature, h Hash) (bool, error) {
	return pk.alg.blsVerify(&pk.point, s, h), nil
}

// Verify verifies a signature of a byte array
func (pk *PubKeyBLS_BLS12381) Verify(s Signature, data []byte, alg Hasher) (bool, error) {
	if alg == nil {
		return false, cryptoError{"VerifyBytes requires a Hasher"}
	}
	h := alg.ComputeHash(data)
	return pk.verifyHash(s, h)
}

// GeneratePrKey generates a private key for BLS on BLS12381 curve
func (a *BLS_BLS12381Algo) generatePrKey(seed []byte) (PrivateKey, error) {
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

func (a *BLS_BLS12381Algo) decodePrKey([]byte) (PrivateKey, error) {
	// TODO: implement DecodePrKey
	return nil, nil
}

func (a *BLS_BLS12381Algo) decodePubKey([]byte) (PublicKey, error) {
	// TODO: implement DecodePubKey
	return nil, nil
}

// PrKeyBLS_BLS12381 is the private key of BLS using BLS12_381, it implements PrivateKey
type PrKeyBLS_BLS12381 struct {
	// the signature algo
	alg *BLS_BLS12381Algo
	// public key
	pk *PubKeyBLS_BLS12381
	// private key data
	scalar scalar
}

func (sk *PrKeyBLS_BLS12381) AlgoName() AlgoName {
	return sk.alg.name
}

func (sk *PrKeyBLS_BLS12381) KeySize() int {
	return sk.alg.prKeyLength
}

func (sk *PrKeyBLS_BLS12381) computePubKey() {
	newPk := &PubKeyBLS_BLS12381{
		alg: sk.alg,
	}
	// compute public key pk = g2^sk
	_G2scalarGenMult(&(newPk.point), &(sk.scalar))
	sk.pk = newPk
}

func (sk *PrKeyBLS_BLS12381) Pubkey() PublicKey {
	if sk.pk != nil {
		return sk.pk
	}
	sk.computePubKey()
	return sk.pk
}

func (a *PrKeyBLS_BLS12381) Encode() ([]byte, error) {
	// TODO: implement EncodePrKey
	return nil, nil
}

// PubKeyBLS_BLS12381 is the public key of BLS using BLS12_381, it implements PublicKey
type PubKeyBLS_BLS12381 struct {
	// the signature algo
	alg *BLS_BLS12381Algo
	// public key data
	point pointG2
}

func (pk *PubKeyBLS_BLS12381) AlgoName() AlgoName {
	return pk.alg.name
}

func (pk *PubKeyBLS_BLS12381) KeySize() int {
	return pk.alg.pubKeyLength
}

func (a *PubKeyBLS_BLS12381) Encode() ([]byte, error) {
	// TODO: implement EncodePubKey
	return nil, nil
}
