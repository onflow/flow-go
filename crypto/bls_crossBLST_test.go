// +build relic, blst

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
)

// validPrivateKeyBytesFlow generates bytes of a valid private key in Flow library
func validPrivateKeyBytesFlow(t *rapid.T) []byte {
	seed := rapid.SliceOfN(rapid.Byte(), KeyGenSeedMinLenBLSBLS12381, KeyGenSeedMaxLenBLSBLS12381).Draw(t, "seed").([]byte)
	sk, err := GeneratePrivateKey(BLSBLS12381, seed)
	// TODO: require.NoError(t, err) seems to mess with rapid
	if err != nil {
		assert.FailNow(t, "failed key generation")
	}
	return sk.Encode()
}

// validPublicKeyBytesFlow generates bytes of a valid public key in Flow library
func validPublicKeyBytesFlow(t *rapid.T) []byte {
	seed := rapid.SliceOfN(rapid.Byte(), KeyGenSeedMinLenBLSBLS12381, KeyGenSeedMaxLenBLSBLS12381).Draw(t, "seed").([]byte)
	sk, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.NoError(t, err)
	return sk.PublicKey().Encode()
}

// validSignatureBytesFlow generates bytes of a valid signature in Flow library
func validSignatureBytesFlow(t *rapid.T) []byte {
	seed := rapid.SliceOfN(rapid.Byte(), KeyGenSeedMinLenBLSBLS12381, KeyGenSeedMaxLenBLSBLS12381).Draw(t, "seed").([]byte)
	sk, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.NoError(t, err)
	hasher := NewBLSKMAC("random_tag")
	message := rapid.SliceOfN(rapid.Byte(), 1, 1000).Draw(t, "msg").([]byte)
	signature, err := sk.Sign(message, hasher)
	require.NoError(t, err)
	return signature
}

// validPrivateKeyBytesBLST generates bytes of a valid private key in BLST library
func validPrivateKeyBytesBLST(t *rapid.T) []byte {
	randomSlice := rapid.SliceOfN(rapid.Byte(), KeyGenSeedMinLenBLSBLS12381, KeyGenSeedMaxLenBLSBLS12381)
	ikm := randomSlice.Draw(t, "ikm").([]byte)
	return blst.KeyGen(ikm).Serialize()
}

// validPublicKeyBytesBLST generates bytes of a valid public key in BLST library
func validPublicKeyBytesBLST(t *rapid.T) []byte {
	ikm := rapid.SliceOfN(rapid.Byte(), KeyGenSeedMinLenBLSBLS12381, KeyGenSeedMaxLenBLSBLS12381).Draw(t, "ikm").([]byte)
	blstS := blst.KeyGen(ikm)
	blstG2 := new(blst.P2Affine).From(blstS)
	return blstG2.Compress()
}

// validSignatureBytesBLST generates bytes of a valid signature in BLST library
func validSignatureBytesBLST(t *rapid.T) []byte {
	ikm := rapid.SliceOfN(rapid.Byte(), KeyGenSeedMinLenBLSBLS12381, KeyGenSeedMaxLenBLSBLS12381).Draw(t, "ikm").([]byte)
	blstS := blst.KeyGen(ikm[:])
	blstG1 := new(blst.P1Affine).From(blstS)
	return blstG1.Compress()
}

// testEncodeDecodePrivateKeyCrossBLST tests encoding and decoding of private keys are consistent with BLST.
// This test assumes private key serialization is identical to the one in BLST.
func testEncodeDecodePrivateKeyCrossBLST(t *rapid.T) {
	randomSlice := rapid.SliceOfN(rapid.Byte(), prKeyLengthBLSBLS12381, prKeyLengthBLSBLS12381)
	validSliceFlow := rapid.Custom(validPrivateKeyBytesFlow)
	validSliceBLST := rapid.Custom(validPrivateKeyBytesBLST)
	// skBytes are bytes of either a valid or a random private key
	skBytes := rapid.OneOf(randomSlice, validSliceFlow, validSliceBLST).Example().([]byte)

	// check decoding results are consistent
	skFlow, err := DecodePrivateKey(BLSBLS12381, skBytes)
	var skBLST blst.Scalar
	res := skBLST.Deserialize(skBytes)

	flowPass := err == nil
	blstPass := res != nil
	require.Equal(t, flowPass, blstPass, "deserialization of the private key %x differs", skBytes)

	// check private keys are equal
	if blstPass && flowPass {
		skFlowOutBytes := skFlow.Encode()
		skBLSTOutBytes := skBLST.Serialize()

		assert.Equal(t, skFlowOutBytes, skBLSTOutBytes)
	}
}

// testEncodeDecodePublicKeyCrossBLST tests encoding and decoding of public keys keys are consistent with BLST.
// This test assumes public key serialization is identical to the one in BLST.
func testEncodeDecodePublicKeyCrossBLST(t *rapid.T) {
	randomSlice := rapid.SliceOfN(rapid.Byte(), PubKeyLenBLSBLS12381, PubKeyLenBLSBLS12381)
	validSliceFlow := rapid.Custom(validPublicKeyBytesFlow)
	validSliceBLST := rapid.Custom(validPublicKeyBytesBLST)
	// pkBytes are bytes of either a valid or a random public key
	pkBytes := rapid.OneOf(randomSlice, validSliceFlow, validSliceBLST).Example().([]byte)

	// check decoding results are consistent
	pkFlow, err := DecodePublicKey(BLSBLS12381, pkBytes)
	var pkBLST blst.P2Affine
	res := pkBLST.Deserialize(pkBytes)
	pkValidBLST := pkBLST.KeyValidate()

	flowPass := err == nil
	blstPass := res != nil && pkValidBLST
	require.Equal(t, flowPass, blstPass, "deserialization of pubkey %x differs", pkBytes)

	// check public keys are equal
	if flowPass && blstPass {
		pkFlowOutBytes := pkFlow.Encode()
		pkBLSTOutBytes := pkBLST.Compress()

		assert.Equal(t, pkFlowOutBytes, pkBLSTOutBytes)
	}
}

// testEncodeDecodeSignatureCrossBLST tests encoding and decoding of signatures are consistent with BLST.
// This test assumes signature serialization is identical to the one in BLST.
func testEncodeDecodeSignatureCrossBLST(t *rapid.T) {
	randomSlice := rapid.SliceOfN(rapid.Byte(), SignatureLenBLSBLS12381, SignatureLenBLSBLS12381)
	validSignatureFlow := rapid.Custom(validSignatureBytesFlow)
	validSignatureBLST := rapid.Custom(validSignatureBytesBLST)
	// sigBytes are bytes of either a valid or a random signature
	sigBytes := rapid.OneOf(randomSlice, validSignatureFlow, validSignatureBLST).Example().([]byte)

	// check decoding results are consistent
	var pointFlow pointG1
	// here we test readPointG1 rather than the simple Signature type alias
	err := readPointG1(&pointFlow, sigBytes)
	flowPass := (err == nil) && checkInG1Test(&pointFlow)

	var pointBLST blst.P1Affine
	res := pointBLST.Uncompress(sigBytes)
	// flow validation has no infinity rejection for G1
	blstPass := (res != nil) && pointBLST.SigValidate(false)

	require.Equal(t, flowPass, blstPass, "deserialization of signature %x differs", sigBytes)

	// check both signatures (G1 points) are are equal
	if flowPass && blstPass {
		sigFlowOutBytes := make([]byte, signatureLengthBLSBLS12381)
		writePointG1(sigFlowOutBytes, &pointFlow)
		sigBLSTOutBytes := pointBLST.Compress()

		assert.Equal(t, sigFlowOutBytes, sigBLSTOutBytes)
	}
}

// testSignHashCrossBLST tests signing a hashed message is consistent with BLST.
//
// The tests assumes the used hash-to-field and map-to-curve are identical in the 2 signatures:
// - hash-to-field : use XMD_SHA256 in both signatures
// - map to curve : Flow and BLST use an SWU mapping consistent with the test vector in
// https://datatracker.ietf.org/doc/html/draft-irtf-cfrg-hash-to-curve-11#appendix-J.9.1
// (Flow map to curve is tested agaisnt the IRTF draft in TestMapToG1, BLST map to curve is not
// tested in this repo)
//
// The test also assumes Flow signature serialization is identical to the one in BLST.
func testSignHashCrossBLST(t *rapid.T) {
	// generate two private keys from the same seed
	skBytes := rapid.Custom(validPrivateKeyBytesFlow).Example().([]byte)

	skFlow, err := DecodePrivateKey(BLSBLS12381, skBytes)
	require.NoError(t, err)
	var skBLST blst.Scalar
	res := skBLST.Deserialize(skBytes)
	require.NotNil(t, res)

	// generate two signatures using both libraries
	blsCipher := []byte("BLS_SIG_BLS12381G1_XMD:SHA-256_SSWU_RO_NUL_")
	message := rapid.SliceOfN(rapid.Byte(), 1, 1000).Example().([]byte)

	var sigBLST blst.P1Affine
	sigBLST.Sign(&skBLST, message, blsCipher)
	sigBytesBLST := sigBLST.Compress()

	skFlowBLS, ok := skFlow.(*PrKeyBLSBLS12381)
	require.True(t, ok, "incoherent key type assertion")
	sigFlow := skFlowBLS.signWithXMDSHA256(message)
	sigBytesFlow := sigFlow.Bytes()

	// check both signatures are equal
	assert.Equal(t, sigBytesBLST, sigBytesFlow)
}

func TestEncodeDecodePrivateKeyCrossBLST(t *testing.T) {
	rapid.Check(t, testEncodeDecodePrivateKeyCrossBLST)
}

func TestEncodeDecodePublicKeyCrossBLST(t *testing.T) {
	rapid.Check(t, testEncodeDecodePublicKeyCrossBLST)
}

func TestEncodeDecodeSignatureCrossBLST(t *testing.T) {
	rapid.Check(t, testEncodeDecodeSignatureCrossBLST)
}

func TestSignHashCrossBLST(t *testing.T) {
	rapid.Check(t, testSignHashCrossBLST)
}
