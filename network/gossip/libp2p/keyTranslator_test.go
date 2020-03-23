package libp2p

import (
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	fcrypto "github.com/dapperlabs/flow-go/crypto"
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

	// generate seed
	seed := k.createSeed()

	// test all the algorithms that are supported by the translator for private key conversion (currently only ECDSA on P-256)
	sa := []fcrypto.SigningAlgorithm{fcrypto.ECDSA_P256}

	for _, s := range sa {
		// generate a Flow private key
		fpk, err := fcrypto.GeneratePrivateKey(s, seed)
		require.NoError(k.T(), err)

		// convert it to a LibP2P private key
		lpk, err := PrivKey(fpk)
		require.NoError(k.T(), err)

		// get the raw bytes of both the keys
		fbytes, err := fpk.Encode()
		require.NoError(k.T(), err)

		lbytes, err := lpk.Raw()
		require.NoError(k.T(), err)

		// compare the raw bytes
		require.Equal(k.T(), fbytes, lbytes)

	}
}

// TestPublicKeyConversion tests that Public keys are successfully converted from Flow to LibP2P representation
func (k *KeyTranslatorTestSuite) TestPublicKeyConversion() {

	// generate seed
	seed := k.createSeed()

	// test the algorithms that are supported by the translator for public key conversion (currently only ECDSA 256)
	// ECDSA_SECp256k1 doesn't work and throws a 'invalid pub key length 64' error
	sa := []fcrypto.SigningAlgorithm{fcrypto.ECDSA_P256}

	for _, s := range sa {
		fpk, err := fcrypto.GeneratePrivateKey(s, seed)
		require.NoError(k.T(), err)

		// get the Flow public key
		fpublic := fpk.PublicKey()

		// convert the Flow public key to a Libp2p public key
		lpublic, err := PublicKey(fpublic)
		require.NoError(k.T(), err)

		// compare raw bytes of the public keys
		fbytes, err := fpublic.Encode()
		require.NoError(k.T(), err)

		lbytes, err := lpublic.Raw()
		require.NoError(k.T(), err)

		require.Equal(k.T(), fbytes, lbytes)
	}
}

func (k *KeyTranslatorTestSuite) createSeed() []byte {
	seed := make([]byte, 100)
	_, err := rand.Read(seed)
	require.NoError(k.T(), err)
	return seed
}
