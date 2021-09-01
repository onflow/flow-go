package p2p

import (
	"crypto/rand"
	"fmt"
	"math"
	"testing"

	"github.com/btcsuite/btcd/btcec"
	lcrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	fcrypto "github.com/onflow/flow-go/crypto"
)

// KeyTranslatorTestSuite tests key conversion from Flow keys to LibP2P keys
type KeyTranslatorTestSuite struct {
	suite.Suite
}

// TestKeyTranslatorTestSuite runs all the test methods in this test suite
func TestKeyTranslatorTestSuite(t *testing.T) {
	suite.Run(t, new(KeyTranslatorTestSuite))
}

// TestPrivateKeyConversion tests that Private keys are successfully converted from Flow to LibP2P representation
func (k *KeyTranslatorTestSuite) TestPrivateKeyConversion() {

	// test all the ECDSA curves that are supported by the translator for private key conversion
	sa := []fcrypto.SigningAlgorithm{fcrypto.ECDSAP256, fcrypto.ECDSASecp256k1}
	prKeyLen := []int{fcrypto.PrKeyLenECDSAP256, fcrypto.PrKeyLenECDSASecp256k1}
	// offset of the key raw bytes in the libp2p encoding
	offset := []int{7, 0}

	loops := 50
	for j, s := range sa {
		for i := 0; i < loops; i++ {
			// generate seed
			seed := k.createSeed()
			// generate a Flow private key
			fpk, err := fcrypto.GeneratePrivateKey(s, seed)
			require.NoError(k.T(), err)

			// convert it to a LibP2P private key
			lpk, err := LibP2PPrivKeyFromFlow(fpk)
			require.NoError(k.T(), err)

			// get the raw bytes of both the keys
			fbytes := fpk.Encode()
			lbytes, err := lpk.Raw()
			lbytes = lbytes[offset[j] : offset[j]+prKeyLen[j]]
			require.NoError(k.T(), err)

			// compare the raw bytes
			require.Equal(k.T(), fbytes, lbytes)
		}
	}
}

// RawUncompressed returns the bytes of the key in an uncompressed format (like Flow library)
// This function is added to the test since Raw function from libp2p only returns the compressed format
func rawUncompressed(key lcrypto.PubKey) ([]byte, error) {
	k, ok := key.(*lcrypto.Secp256k1PublicKey)
	if !ok {
		return nil, fmt.Errorf("libp2p public key must be of type Secp256k1PublicKey")
	}
	return (*btcec.PublicKey)(k).SerializeUncompressed()[1:], nil
}

// TestPublicKeyConversion tests that Public keys are successfully converted from Flow to LibP2P representation
func (k *KeyTranslatorTestSuite) TestPublicKeyConversion() {

	// test the algorithms that are supported by the translator for public key conversion (currently only ECDSA 256)
	// ECDSASecp256k1 doesn't work and throws a 'invalid pub key length 64' error
	sa := []fcrypto.SigningAlgorithm{fcrypto.ECDSAP256, fcrypto.ECDSASecp256k1}
	// the offset of the public key bytes in the x509 encoding of an ECDSA P-256 public key
	x509offset := 27

	loops := 50
	for _, s := range sa {
		for i := 0; i < loops; i++ {
			// generate seed
			seed := k.createSeed()
			fpk, err := fcrypto.GeneratePrivateKey(s, seed)
			require.NoError(k.T(), err)

			// get the Flow public key
			fpublic := fpk.PublicKey()

			// convert the Flow public key to a Libp2p public key
			lpublic, err := LibP2PPublicKeyFromFlow(fpublic)
			require.NoError(k.T(), err)

			// compare raw bytes of the public keys
			fbytes := fpublic.Encode()
			var lbytes []byte
			if s == fcrypto.ECDSAP256 {
				lbytes, err = lpublic.Raw()
				lbytes = lbytes[x509offset:]
			} else if s == fcrypto.ECDSASecp256k1 {
				lbytes, err = rawUncompressed(lpublic)
			}
			require.NoError(k.T(), err)

			require.Equal(k.T(), fbytes, lbytes)
		}
	}
}

func (k *KeyTranslatorTestSuite) TestPublicKeyRoundTrip() {
	sa := []fcrypto.SigningAlgorithm{fcrypto.ECDSAP256, fcrypto.ECDSASecp256k1}
	loops := 50
	for _, s := range sa {
		for i := 0; i < loops; i++ {

			// generate seed
			seed := k.createSeed()
			fpk, err := fcrypto.GeneratePrivateKey(s, seed)
			require.NoError(k.T(), err)

			// get the Flow public key
			fpublic := fpk.PublicKey()

			// convert the Flow public key to a Libp2p public key
			lpublic, err := LibP2PPublicKeyFromFlow(fpublic)
			require.NoError(k.T(), err)

			fpublic2, err := FlowPublicKeyFromLibP2P(lpublic)
			require.NoError(k.T(), err)
			require.Equal(k.T(), fpublic, fpublic2)

		}
	}

}

// TestLibP2PIDGenerationIsConsistent tests that a LibP2P peer ID generated using Flow ECDSA key is deterministic
func (k *KeyTranslatorTestSuite) TestPeerIDGenerationIsConsistent() {
	// generate a seed which will be used for both - Flow keys and Libp2p keys
	seed := k.createSeed()

	// generate a Flow private key
	fpk, err := fcrypto.GeneratePrivateKey(fcrypto.ECDSAP256, seed)
	require.NoError(k.T(), err)

	// get the Flow public key
	fpublic := fpk.PublicKey()

	// convert it to the Libp2p Public key
	lconverted, err := LibP2PPublicKeyFromFlow(fpublic)
	require.NoError(k.T(), err)

	// check that the LibP2P Id generation is deterministic
	var prev peer.ID
	for i := 0; i < 100; i++ {

		// generate a Libp2p Peer ID from the converted public key
		fpeerID, err := peer.IDFromPublicKey(lconverted)
		require.NoError(k.T(), err)

		if i > 0 {
			err = prev.Validate()
			require.NoError(k.T(), err)
			assert.Equal(k.T(), prev, fpeerID, "peer ID generation is not deterministic")
		}
		prev = fpeerID
	}
}

func (k *KeyTranslatorTestSuite) createSeed() []byte {
	seedLen := int(math.Max(fcrypto.KeyGenSeedMinLenECDSAP256, fcrypto.KeyGenSeedMinLenECDSASecp256k1))
	seed := make([]byte, seedLen)
	n, err := rand.Read(seed)
	require.NoError(k.T(), err)
	require.Equal(k.T(), n, seedLen)
	return seed
}
