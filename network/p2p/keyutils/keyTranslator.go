package keyutils

import (
	goecdsa "crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"fmt"
	"math/big"

	lcrypto "github.com/libp2p/go-libp2p-core/crypto"
	lcrypto_pb "github.com/libp2p/go-libp2p-core/crypto/pb"
	"github.com/libp2p/go-libp2p-core/peer"

	"github.com/onflow/flow-go/crypto"
	fcrypto "github.com/onflow/flow-go/crypto"
)

// This module is meant to help libp2p <-> flow public key conversions
// Flow's Network Public Keys and LibP2P's public keys are a marshalling standard away from each other and inter-convertible.
// Libp2p supports keys as ECDSA public Keys on either the NIST P-256 curve or the "Bitcoin" secp256k1 curve, see https://github.com/libp2p/specs/blob/master/peer-ids/peer-ids.md#peer-ids
// libp2p represents the P-256 keys as ASN1-DER and the secp256k1 keys as X9.62 encodings in compressed format
//
// While Flow's key types supports both secp256k1 and P-256 keys (under crypto/ecdsa), note that Flow's networking keys are always P-256 keys.
// Flow represents both the P-256 keys and the secp256k1 keys in uncompressed representation, but their byte serializations (Encode) do not include the X9.62 compression bit
// Flow also makes a X9.62 compressed format (with compression bit) accessible (EncodeCompressed)

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

// PeerIDFromFlowPublicKey converts a Flow public key to a LibP2P peer ID.
func PeerIDFromFlowPublicKey(networkPubKey fcrypto.PublicKey) (pid peer.ID, err error) {
	pk, err := LibP2PPublicKeyFromFlow(networkPubKey)
	if err != nil {
		err = fmt.Errorf("failed to convert Flow key to LibP2P key: %w", err)
		return
	}

	pid, err = peer.IDFromPublicKey(pk)
	if err != nil {
		err = fmt.Errorf("failed to convert LibP2P key to peer ID: %w", err)
		return
	}

	return
}

// PrivKey converts a Flow private key to a LibP2P Private key
func LibP2PPrivKeyFromFlow(fpk fcrypto.PrivateKey) (lcrypto.PrivKey, error) {
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
func LibP2PPublicKeyFromFlow(fpk fcrypto.PublicKey) (lcrypto.PubKey, error) {
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
		bytes = make([]byte, crypto.PubKeyLenECDSASecp256k1+1) // libp2p requires an extra byte
		bytes[0] = 4                                           // signals uncompressed form as specified in section 4.3.6/7 of ANSI X9.62.
		copy(bytes[1:], tempBytes)
	}

	return um(bytes)
}

// This converts some libp2p PubKeys to a flow PublicKey
// - the supported key types are ECDSA P-256 and ECDSA Secp256k1 public keys,
// - libp2p also supports RSA and Ed25519 keys, which Flow doesn't, their conversion will return an error.
func FlowPublicKeyFromLibP2P(lpk lcrypto.PubKey) (fcrypto.PublicKey, error) {

	switch ktype := lpk.Type(); ktype {
	case lcrypto_pb.KeyType_ECDSA:
		pubB, err := lpk.Raw()
		if err != nil {
			return nil, lcrypto.ErrBadKeyType
		}
		key, err := x509.ParsePKIXPublicKey(pubB)
		if err != nil {
			// impossible to decode from ASN1.DER
			return nil, lcrypto.ErrBadKeyType
		}
		cryptoKey, ok := key.(*goecdsa.PublicKey)
		if !ok {
			// not recognized as crypto.P-256
			return nil, lcrypto.ErrNotECDSAPubKey
		}
		// ferrying through DecodePublicKey to get the curve checks
		pk_uncompressed := elliptic.Marshal(cryptoKey, cryptoKey.X, cryptoKey.Y)
		// the first bit is the compression bit of X9.62
		pubKey, err := crypto.DecodePublicKey(crypto.ECDSAP256, pk_uncompressed[1:])
		if err != nil {
			return nil, lcrypto.ErrNotECDSAPubKey
		}
		return pubKey, nil
	case lcrypto_pb.KeyType_Secp256k1:
		// libp2p uses the compressed representation, flow the uncompressed one
		lpk_secp256k1, ok := lpk.(*lcrypto.Secp256k1PublicKey)
		if !ok {
			return nil, lcrypto.ErrBadKeyType
		}
		secpBytes, err := lpk_secp256k1.Raw()
		if err != nil { // this never errors
			return nil, lcrypto.ErrBadKeyType
		}
		pk, err := crypto.DecodePublicKeyCompressed(crypto.ECDSASecp256k1, secpBytes)
		if err != nil {
			return nil, lcrypto.ErrNotECDSAPubKey
		}
		return pk, nil
	default:
		return nil, lcrypto.ErrBadKeyType
	}

}

// keyType translates Flow signing algorithm constants to the corresponding LibP2P constants
func keyType(sa fcrypto.SigningAlgorithm) (lcrypto_pb.KeyType, error) {
	switch sa {
	case fcrypto.ECDSAP256:
		return lcrypto_pb.KeyType_ECDSA, nil
	case fcrypto.ECDSASecp256k1:
		return lcrypto_pb.KeyType_Secp256k1, nil
	default:
		return -1, lcrypto.ErrBadKeyType
	}
}
