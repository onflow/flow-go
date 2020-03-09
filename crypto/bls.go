// +build relic

package crypto

// BLS_BLS12381Algo, embeds SignAlgo
type BLS_BLS12381Algo struct {
	// points to Relic context of BLS12-381 with all the parameters
	context ctx
	// embeds commonSigner
	*commonSigner
}

// Sign signs an array of bytes
// This function does not modify the private key, even temporarily
// If the hasher used is KMAC128, it is not modified by the function, even temporarily
func (sk *PrKeyBLS_BLS12381) Sign(data []byte, kmac Hasher) (Signature, error) {
	if kmac == nil {
		return nil, cryptoError{"Sign requires a Hasher"}
	}
	// hash the input to 128 bytes
	h := kmac.ComputeHash(data)
	return sk.alg.blsSign(&sk.scalar, h), nil
}

const BLS_KMACFunction = "H2C"

// NewBLS_KMAC returns a new KMAC128 instance with the right parameters
// chosen for BLS signatures and verifications
// tag is the domain separation tag
func NewBLS_KMAC(tag string) Hasher {
	// the error is ignored as the parameter lengths are in the correct range of kmac
	kmac, _ := NewKMAC_128([]byte(tag), []byte("BLS_KMACFunction"), OpSwUInputLenBLS_BLS12381)
	return kmac
}

// Verify verifies a signature of a byte array
// This function does not modify the public key, even temporarily
// If the hasher used is KMAC128, it is not modified by the function, even temporarily
func (pk *PubKeyBLS_BLS12381) Verify(s Signature, data []byte, kmac Hasher) (bool, error) {
	if kmac == nil {
		return false, cryptoError{"VerifyBytes requires a Hasher"}
	}
	if pk.check != valid {
		return false, cryptoError{"verification must use a verified public key."}
	}
	// hash the input to 128 bytes
	h := kmac.ComputeHash(data)
	return pk.alg.blsVerify(&pk.point, s, h), nil
}

// GeneratePrKey generates a private key for BLS on BLS12381 curve
func (a *BLS_BLS12381Algo) generatePrivateKey(seed []byte) (PrivateKey, error) {
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

func (a *BLS_BLS12381Algo) decodePrivateKey([]byte) (PrivateKey, error) {
	// TODO: implement DecodePrKey
	return nil, nil
}

func (a *BLS_BLS12381Algo) decodePublicKey(publicKeyBytes []byte) (PublicKey, error) {
	pk := &PubKeyBLS_BLS12381{
		alg:   a,
		check: unchecked,
	}
	readPointG2(&pk.point, publicKeyBytes)
	return pk, nil
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

func (sk *PrKeyBLS_BLS12381) Algorithm() SigningAlgorithm {
	return sk.alg.algo
}

func (sk *PrKeyBLS_BLS12381) KeySize() int {
	return sk.alg.prKeyLength
}

func (sk *PrKeyBLS_BLS12381) computePublicKey() {
	newPk := &PubKeyBLS_BLS12381{
		alg: sk.alg,
	}
	// compute public key pk = g2^sk
	_G2scalarGenMult(&(newPk.point), &(sk.scalar))
	newPk.check = valid
	sk.pk = newPk
}

func (sk *PrKeyBLS_BLS12381) PublicKey() PublicKey {
	if sk.pk != nil {
		return sk.pk
	}
	sk.computePublicKey()
	return sk.pk
}

func (a *PrKeyBLS_BLS12381) Encode() ([]byte, error) {
	// TODO: implement EncodePrKey
	return nil, nil
}

// memCheckState defines the state of the public key
// membership in G2
type memCheckState int

const (
	// states of the public key membership
	unchecked memCheckState = iota
	valid
	invalid
)

// PubKeyBLS_BLS12381 is the public key of BLS using BLS12_381,
// it implements PublicKey
type PubKeyBLS_BLS12381 struct {
	// the signature algo
	alg *BLS_BLS12381Algo
	// public key data
	point pointG2
	// membership check in G2
	// this flag forces validating the public key
	// before it is used for signature verifcations
	check memCheckState
}

func (pk *PubKeyBLS_BLS12381) Algorithm() SigningAlgorithm {
	return pk.alg.algo
}

func (pk *PubKeyBLS_BLS12381) KeySize() int {
	return pk.alg.pubKeyLength
}

func (a *PubKeyBLS_BLS12381) Encode() ([]byte, error) {
	dest := make([]byte, pubKeyLengthBLS_BLS12381)
	writePointG2(dest, &a.point)
	return dest, nil
}

// ValidityCheck runs a membership check of BLS public keys on BLS12-381 curve.
// Returns true if the public key is on the correct subgroup of the curve
// and false otherwise
// Itr is necessary to run this test once for every public key before
// it is used to verify BLS signatures
func (a *PubKeyBLS_BLS12381) ValidityCheck() (bool, error) {
	a.check = valid
	return a.check == valid, nil
}
