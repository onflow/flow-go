// +build relic

package crypto

// This file contains tests against the library BLST (https://github.com/supranational/blst).
// The purpose of these tests is to detect differences with a different implementation of BLS on the BLS12-381
// curve since the BLS IRTF draft (https://datatracker.ietf.org/doc/draft-irtf-cfrg-bls-signature/) doesn't
// provide extensive test vectors.
// A detected difference with BLST library doesn't necessary mean a bug or a non-standard implementation since
// both libraries might have made different choices. It is nevertheless a good flag for possible bugs or deviations
// from the standard as both libraries are being developed.

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	blst "github.com/supranational/blst/bindings/go"
	"pgregory.net/rapid"

	"github.com/onflow/flow-go/crypto/hash"
)

// validPrivateKeyBytes generates bytes of a valid private key
func validPrivateKeyBytes(t *rapid.T) []byte {
	seed := rapid.SliceOfN(rapid.Byte(), KeyGenSeedMinLenBLSBLS12381, KeyGenSeedMaxLenBLSBLS12381).Draw(t, "seed").([]byte)
	sk, _ := GeneratePrivateKey(BLSBLS12381, seed)
	// TODO: require.NoError(t, err) seems to mess with rapid
	return sk.Encode()
}

// validPublicKeyBytes generates bytes of a valid public key
func validPublicKeyBytes(t *rapid.T) []byte {
	seed := rapid.SliceOfN(rapid.Byte(), KeyGenSeedMinLenBLSBLS12381, KeyGenSeedMaxLenBLSBLS12381).Draw(t, "seed").([]byte)
	sk, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.NoError(t, err)
	return sk.PublicKey().Encode()
}

// validSignature generates bytes of a valid signature
func validSignature(t *rapid.T) []byte {
	seed := rapid.SliceOfN(rapid.Byte(), KeyGenSeedMinLenBLSBLS12381, KeyGenSeedMaxLenBLSBLS12381).Draw(t, "seed").([]byte)
	sk, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.NoError(t, err)
	hasher := NewBLSKMAC("random_tag")
	message := rapid.SliceOfN(rapid.Byte(), 1, 1000).Draw(t, "msg").([]byte)
	signature, err := sk.Sign(message, hasher)
	require.NoError(t, err)
	return signature
}

// testEncodeDecodePrivateKeyCrossBLST tests encoding and decoding of private keys are consistent with BLST.
// This test assumes private key serialization is identical to the one in BLST.
func testEncodeDecodePrivateKeyCrossBLST(t *rapid.T) {
	randomSlice := rapid.SliceOfN(rapid.Byte(), prKeyLengthBLSBLS12381, prKeyLengthBLSBLS12381)
	validSlice := rapid.Custom(validPrivateKeyBytes)
	// skBytes are bytes of either a valid or a random private key
	skBytes := rapid.OneOf(randomSlice, validSlice).Example().([]byte)

	// check decoding results are consistent
	skBLS, err := DecodePrivateKey(BLSBLS12381, skBytes)
	var skBLST blst.Scalar
	res := skBLST.Deserialize(skBytes)

	blsPass := err == nil
	blstPass := res != nil
	require.Equal(t, blsPass, blstPass, "deserialization of the private key %x differs", skBytes)

	// check private keys are equal
	if blstPass && blsPass {
		skBLSOutBytes := skBLS.Encode()
		skBLSTOutBytes := skBLST.Serialize()

		assert.Equal(t, skBLSOutBytes, skBLSTOutBytes)
	}
}

// testEncodeDecodePublicKeyCrossBLST tests encoding and decoding of public keys keys are consistent with BLST.
// This test assumes public key serialization is identical to the one in BLST.
func testEncodeDecodePublicKeyCrossBLST(t *rapid.T) {
	randomSlice := rapid.SliceOfN(rapid.Byte(), PubKeyLenBLSBLS12381, PubKeyLenBLSBLS12381)
	validSliceBLS := rapid.Custom(validPublicKeyBytes)
	// pkBytes are bytes of either a valid or a random public key
	pkBytes := rapid.OneOf(randomSlice, validSliceBLS).Example().([]byte)

	// check decoding results are consistent
	pkBLS, err := DecodePublicKey(BLSBLS12381, pkBytes)
	var pkBLST blst.P2Affine
	res := pkBLST.Deserialize(pkBytes)
	pkValidBLST := pkBLST.KeyValidate()

	blsPass := err == nil
	blstPass := res != nil && pkValidBLST
	require.Equal(t, blsPass, blstPass, "deserialization of pubkey %x differs", pkBytes)

	// check public keys are equal
	if blsPass && blstPass {
		pkBLSOutBytes := pkBLS.Encode()
		pkBLSTOutBytes := pkBLST.Compress()

		assert.Equal(t, pkBLSOutBytes, pkBLSTOutBytes)
	}
}

// testEncodeDecodeSignatureCrossBLST tests encoding and decoding of signatures are consistent with BLST.
// This test assumes signature serialization is identical to the one in BLST.
func testEncodeDecodeSignatureCrossBLST(t *rapid.T) {
	randomSlice := rapid.SliceOfN(rapid.Byte(), SignatureLenBLSBLS12381, SignatureLenBLSBLS12381)
	validSignature := rapid.Custom(validSignature)
	// sigBytes are bytes of either a valid or a random signature
	sigBytes := rapid.OneOf(randomSlice, validSignature).Example().([]byte)

	// check decoding results are consistent
	var sigBLSPt pointG1
	// here we test readPointG1 rather than the simple Signature type alias
	err := readPointG1(&sigBLSPt, sigBytes)
	blsPass := (err == nil) && checkInG1Test(&sigBLSPt)

	var sigBLST blst.P1Affine
	res := sigBLST.Uncompress(sigBytes)
	// our validation has no infinity rejection for G1
	blstPass := (res != nil) && sigBLST.SigValidate(false)

	require.Equal(t, blsPass, blstPass, "deserialization of signature %x differs", sigBytes)

	// check both signatures (G1 points) are are equal
	if blsPass && blstPass {
		sigBLSOutBytes := make([]byte, signatureLengthBLSBLS12381)
		writePointG1(sigBLSOutBytes, &sigBLSPt)
		sigBLSTOutBytes := sigBLST.Compress()

		assert.Equal(t, sigBLSOutBytes, sigBLSTOutBytes)
	}

}

// testSignWithRelicMapCrossBLST tests signing a hashed G1 point is consistent with BLST.
// The tests assumes the used hash-to-field and map-to-curve are identical, by using the relic
// hash-to-curve, which is tested to be identical to the one used in BLST.
// The test only checks the consistency of the remaining steps of the signature with BLST.
// The test also assumes signature serialization is identical to the one in BLST.
func testSignWithRelicMapCrossBLST(t *rapid.T) {
	blsCipher := []byte("BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_")
	message := rapid.SliceOfN(rapid.Byte(), 1, 1000).Example().([]byte)

	// generate two private keys from the same seed
	skBytes := rapid.Custom(validPrivateKeyBytes).Example().([]byte)

	sk, err := DecodePrivateKey(BLSBLS12381, skBytes)
	require.NoError(t, err)
	var skBLST blst.Scalar
	res := skBLST.Deserialize(skBytes)
	require.NotNil(t, res)

	// generate two signatures using both libraries
	var sigBLST blst.P1Affine
	sigBLST.Sign(&skBLST, message, blsCipher)
	sigBytesBLST := sigBLST.Compress()

	skBLS, ok := sk.(*PrKeyBLSBLS12381)
	require.True(t, ok, "incoherent key type assertion")
	sig := skBLS.signWithRelicMapTest(message)
	sigBytesBLS := sig.Bytes()

	// check both signatures are equal
	assert.Equal(t, sigBytesBLST, sigBytesBLS)
}

func TestBLSCrossBLST(t *testing.T) {
	rapid.Check(t, testEncodeDecodePrivateKeyCrossBLST)
	rapid.Check(t, testEncodeDecodePublicKeyCrossBLST)
	rapid.Check(t, testEncodeDecodeSignatureCrossBLST)
	rapid.Check(t, testSignWithRelicMapCrossBLST)
}
