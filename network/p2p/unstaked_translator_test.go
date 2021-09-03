package p2p

import (
	"crypto/rand"
	"math"
	"testing"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/stretchr/testify/require"

	fcrypto "github.com/onflow/flow-go/crypto"
)

// For these test, refer to https://github.com/libp2p/specs/blob/master/peer-ids/peer-ids.md for libp2p
// PeerID specifications and how they relate to keys.

// This test shows we can't use ECDSA P-256 keys for libp2p and expect PeerID <=> PublicKey bijections
func TestIDTranslationP256(t *testing.T) {
	loops := 50
	for i := 0; i < loops; i++ {
		pID := createPeerIDFromAlgo(t, fcrypto.ECDSAP256)

		// check that we can not extract the public key back
		// This makes sense: the x509 serialization of ECDSA P-256 keys in uncompressed form is 64 + 2 bytes,
		// and libp2p uses multihash.IDENTITY only on serializations of less than 42 bytes
		_, err := pID.ExtractPublicKey()
		require.NotNil(t, err)

	}
}

// This test shows we can use ECDSA Secp256k1 keys for libp2p and expect PeerID <=> PublicKey bijections
func TestIDTranslationSecp256k1(t *testing.T) {
	loops := 50
	for i := 0; i < loops; i++ {
		pID := createPeerIDFromAlgo(t, fcrypto.ECDSASecp256k1)

		// check that we can extract the public key back
		// This makes sense: the compressed serialization of ECDSA Secp256k1 keys is 33 + 2 bytes,
		// and libp2p uses multihash.IDENTITY on serializations of less than 42 bytes
		_, err := pID.ExtractPublicKey()
		require.NoError(t, err)

	}
}

func TestUnstakedTranslationRoundTrip(t *testing.T) {
	max_iterations := 50
	unstakedTranslator := NewUnstakedNetworkIDTranslator()

	tested_vectors := 0

	for ok := true; ok; ok = tested_vectors < max_iterations {
		pID := createPeerIDFromAlgo(t, fcrypto.ECDSASecp256k1)

		pk, err := pID.ExtractPublicKey()
		require.NoError(t, err)

		// for a secp256k1 key, this is just the compressed representation
		pkBytes, err := pk.Raw()
		require.NoError(t, err)

		// key is positive, roundtrip should be possible
		if pkBytes[0] == 0x02 {
			tested_vectors++

			flowID, err := unstakedTranslator.GetFlowID(pID)
			require.NoError(t, err)
			retrievedPeerID, err := unstakedTranslator.GetPeerID(flowID)
			require.NoError(t, err)
			require.Equal(t, pID, retrievedPeerID)
		}

	}
}

func createPeerIDFromAlgo(t *testing.T, sa fcrypto.SigningAlgorithm) peer.ID {
	seed := createSeed(t)

	// this matches GenerateNetworkingKeys, and is intended to validate the choices in cmd/bootstrap
	key, err := fcrypto.GeneratePrivateKey(sa, seed)
	require.NoError(t, err)

	// get the public key
	pubKey := key.PublicKey()

	// extract the corresponding libp2p public Key
	libp2pPubKey, err := LibP2PPublicKeyFromFlow(pubKey)
	require.NoError(t, err)

	// obtain the PeerID based on libp2p's own rules
	pID, err := peer.IDFromPublicKey(libp2pPubKey)
	require.NoError(t, err)

	return pID
}

func createSeed(t *testing.T) []byte {
	seedLen := int(math.Max(fcrypto.KeyGenSeedMinLenECDSAP256, fcrypto.KeyGenSeedMinLenECDSASecp256k1))
	seed := make([]byte, seedLen)
	n, err := rand.Read(seed)
	require.NoError(t, err)
	require.Equal(t, n, seedLen)
	return seed
}
