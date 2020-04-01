package libp2p

import (
	lcrypto "github.com/libp2p/go-libp2p-core/crypto"
	lcrypto_pb "github.com/libp2p/go-libp2p-core/crypto/pb"

	"github.com/dapperlabs/flow-go/crypto"
	fcrypto "github.com/dapperlabs/flow-go/crypto"
)

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
	byte, err := fpk.Encode()
	if err != nil {
		return nil, err
	}
	// unmarshal the raw dump
	return um(byte)
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

	// at this point, keytype is either KeyType_ECDSA or KeyType_Secp256k1
	// and can't hold another value
	var bytes []byte
	if keyType == lcrypto_pb.KeyType_ECDSA {
		bytes, err = fpk.Encode()
	} else if keyType == lcrypto_pb.KeyType_Secp256k1 {
		bytes = make([]byte, crypto.PubKeyLenECDSA_SECp256k1+1) // libp2p requires an extra byte
		bytes[0] = 4                                            // magic number in libp2p to refer to an uncompressed key
		var tempBytes []byte
		tempBytes, err = fpk.Encode()
		copy(bytes[1:], tempBytes)
	}

	if err != nil {
		return nil, err
	}
	return um(bytes)
}

// keyType translates Flow signing algorithm constants to the corresponding LibP2P constants
func keyType(sa fcrypto.SigningAlgorithm) (lcrypto_pb.KeyType, error) {
	switch sa {
	case fcrypto.ECDSA_P256:
		return lcrypto_pb.KeyType_ECDSA, nil
	case fcrypto.ECDSA_SECp256k1:
		return lcrypto_pb.KeyType_Secp256k1, nil
	default:
		return -1, lcrypto.ErrBadKeyType
	}
}
