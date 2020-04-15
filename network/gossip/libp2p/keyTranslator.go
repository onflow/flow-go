package libp2p

import (
	goecdsa "crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"math/big"

	lcrypto "github.com/libp2p/go-libp2p-core/crypto"
	lcrypto_pb "github.com/libp2p/go-libp2p-core/crypto/pb"

	"github.com/dapperlabs/flow-go/crypto"
	fcrypto "github.com/dapperlabs/flow-go/crypto"
)

// assigns a big.Int input to a Go ecdsa private key
func setPrKey(c elliptic.Curve, k *big.Int) *goecdsa.PrivateKey {
	priv := new(goecdsa.PrivateKey)
	priv.PublicKey.Curve = c
	priv.D = k
	priv.PublicKey.X, priv.PublicKey.Y = c.ScalarBaseMult(k.Bytes())
	return priv
}

// assigns two big.Int inputs to a Go ecdsa public key
func setPubKey(c elliptic.Curve, x *big.Int, y *big.Int) *goecdsa.PublicKey {
	pub := new(goecdsa.PublicKey)
	pub.Curve = c
	pub.X = x
	pub.Y = y
	return pub
}

// Both Flow and LibP2P define a crypto package with their own abstraction of Keys
// These utility functions convert a Flow crypto key to a LibP2P key (Flow --> LibP2P)

// PrivKey converts a Flow private key to a LibP2P Private key
func PrivKey(fpk fcrypto.PrivateKey) (lcrypto.PrivKey, error) {
	// get the signature algorithm
	keyType, err := keyType(fpk.Algorithm())
	if err != nil {
		return nil, err
	}

	// based on the signature algorithm, get the appropriate libp2p unmarshaller
	um, ok := lcrypto.PrivKeyUnmarshallers[keyType]
	if !ok {
		return nil, lcrypto.ErrBadKeyType
	}

	// get the raw dump of the flow key
	bytes := fpk.Encode()

	// in the case of NIST curves, the raw bytes need to be converted to x509 bytes
	// to accommodate libp2p unmarshaller
	if keyType == lcrypto_pb.KeyType_ECDSA {
		var k big.Int
		k.SetBytes(bytes)
		goKey := setPrKey(elliptic.P256(), &k)
		bytes, err = x509.MarshalECPrivateKey(goKey)
		if err != nil {
			return nil, lcrypto.ErrBadKeyType
		}
	}
	return um(bytes)
}

// PublicKey converts a Flow public key to a LibP2P public key
func PublicKey(fpk fcrypto.PublicKey) (lcrypto.PubKey, error) {
	keyType, err := keyType(fpk.Algorithm())
	if err != nil {
		return nil, err
	}
	um, ok := lcrypto.PubKeyUnmarshallers[keyType]
	if !ok {
		return nil, lcrypto.ErrBadKeyType
	}

	tempBytes := fpk.Encode()

	// at this point, keytype is either KeyType_ECDSA or KeyType_Secp256k1
	// and can't hold another value
	var bytes []byte
	if keyType == lcrypto_pb.KeyType_ECDSA {
		var x, y big.Int
		x.SetBytes(tempBytes[:len(tempBytes)/2])
		y.SetBytes(tempBytes[len(tempBytes)/2:])
		goKey := setPubKey(elliptic.P256(), &x, &y)
		bytes, err = x509.MarshalPKIXPublicKey(goKey)
		if err != nil {
			return nil, lcrypto.ErrBadKeyType
		}
	} else if keyType == lcrypto_pb.KeyType_Secp256k1 {
		bytes = make([]byte, crypto.PubKeyLenEcdsaSecp256k1+1) // libp2p requires an extra byte
		bytes[0] = 4                                           // magic number in libp2p to refer to an uncompressed key
		copy(bytes[1:], tempBytes)
	}

	return um(bytes)
}

// keyType translates Flow signing algorithm constants to the corresponding LibP2P constants
func keyType(sa fcrypto.SigningAlgorithm) (lcrypto_pb.KeyType, error) {
	switch sa {
	case fcrypto.EcdsaP256:
		return lcrypto_pb.KeyType_ECDSA, nil
	case fcrypto.EcdsaSecp256k1:
		return lcrypto_pb.KeyType_Secp256k1, nil
	default:
		return -1, lcrypto.ErrBadKeyType
	}
}
