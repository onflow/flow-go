// +build relic

package crypto

import (
	"fmt"
)

// BLS_BLS12381Algo, embeds SignAlgo
type BLS_BLS12381Algo struct {
	// points to Relic context of BLS12-381 with all the parameters
	context ctx

	// points to precomputed parameters of BLS12-381 not included in Relic context
	//precomputed BLSPrecParams
	// embeds commonSigner
	*commonSigner
}

// signHash implements BLS signature on BLS12381 curve
func (sk *PrKeyBLS_BLS12381) signHash(h Hash) (Signature, error) {
	return sk.alg.blsSign(&sk.scalar, h), nil
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
	return sk.signHash(h)
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

// verifyHash implements BLS signature verification on BLS12381 curve
func (pk *PubKeyBLS_BLS12381) verifyHash(s Signature, h Hash) (bool, error) {
	return pk.alg.blsVerify(&pk.point, s, h), nil
}

// Verify verifies a signature of a byte array
// This function does not modify the public key, even temporarily
// If the hasher used is KMAC128, it is not modified by the function, even temporarily
func (pk *PubKeyBLS_BLS12381) Verify(s Signature, data []byte, kmac Hasher) (bool, error) {
	if kmac == nil {
		return false, cryptoError{"VerifyBytes requires a Hasher"}
	}
	// hash the input to 128 bytes
	h := kmac.ComputeHash(data)
	return pk.verifyHash(s, h)
}

// GeneratePrKey generates a private key for BLS on BLS12381 curve
func (a *BLS_BLS12381Algo) generatePrivateKey(seed []byte) (PrivateKey, error) {
	sk := &PrKeyBLS_BLS12381{
		alg: a,
		// public key is not computed
		pk: nil,
	}
	// seed the RNG
	seedRelic(seed)
	// Generate private key here
	err := randZr(&(sk.scalar))
	if err != nil {
		return nil, err
	}
	return sk, nil
}

func (a *BLS_BLS12381Algo) decodePrivateKey(privateKeyBytes []byte) (PrivateKey, error) {
	if len(privateKeyBytes) != prKeyLengthBLS_BLS12381 {
		return nil, cryptoError{fmt.Sprintf("the input length has to be equal to %d", prKeyLengthBLS_BLS12381)}
	}
	sk := &PrKeyBLS_BLS12381{
		alg: a,
		pk:  nil,
	}
	readScalar(&sk.scalar, privateKeyBytes)
	return sk, nil
}

func (a *BLS_BLS12381Algo) decodePublicKey(publicKeyBytes []byte) (PublicKey, error) {
	if len(publicKeyBytes) != pubKeyLengthBLS_BLS12381 {
		return nil, cryptoError{fmt.Sprintf("the input length has to be equal to %d", pubKeyLengthBLS_BLS12381)}
	}
	pk := &PubKeyBLS_BLS12381{
		alg: a,
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
	dest := make([]byte, prKeyLengthBLS_BLS12381)
	writeScalar(dest, &a.scalar)
	return dest, nil
}

// PubKeyBLS_BLS12381 is the public key of BLS using BLS12_381, it implements PublicKey
type PubKeyBLS_BLS12381 struct {
	// the signature algo
	alg *BLS_BLS12381Algo
	// public key data
	point pointG2
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
