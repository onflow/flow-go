//go:build !relic
// +build !relic

package crypto

import (
	"encoding/hex"
	"testing"

	"crypto/elliptic"
	crand "crypto/rand"
	"math/big"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto/hash"
)

var ecdsaCurves = []SigningAlgorithm{
	ECDSAP256,
	ECDSASecp256k1,
}
var ecdsaPrKeyLen = map[SigningAlgorithm]int{
	ECDSAP256:      PrKeyLenECDSAP256,
	ECDSASecp256k1: PrKeyLenECDSASecp256k1,
}
var ecdsaPubKeyLen = map[SigningAlgorithm]int{
	ECDSAP256:      PubKeyLenECDSAP256,
	ECDSASecp256k1: PubKeyLenECDSASecp256k1,
}
var ecdsaSigLen = map[SigningAlgorithm]int{
	ECDSAP256:      SignatureLenECDSAP256,
	ECDSASecp256k1: SignatureLenECDSASecp256k1,
}

// ECDSA tests
func TestECDSA(t *testing.T) {

	for _, curve := range ecdsaCurves {
		t.Logf("Testing ECDSA for curve %s", curve)
		// test key generation seed limits
		testKeyGenSeed(t, curve, KeyGenSeedMinLen, KeyGenSeedMaxLen)
		// test consistency
		halg := hash.NewSHA3_256()
		testGenSignVerify(t, curve, halg)
	}
}

type dummyHasher struct{ size int }

func newDummyHasher(size int) hash.Hasher               { return &dummyHasher{size} }
func (d *dummyHasher) Algorithm() hash.HashingAlgorithm { return hash.UnknownHashingAlgorithm }
func (d *dummyHasher) Size() int                        { return d.size }
func (d *dummyHasher) ComputeHash([]byte) hash.Hash     { return make([]byte, d.size) }
func (d *dummyHasher) Write([]byte) (int, error)        { return 0, nil }
func (d *dummyHasher) SumHash() hash.Hash               { return make([]byte, d.size) }
func (d *dummyHasher) Reset()                           {}

func TestECDSAHasher(t *testing.T) {

	for _, curve := range ecdsaCurves {

		// generate a key pair
		seed := make([]byte, KeyGenSeedMinLen)
		n, err := crand.Read(seed)
		require.Equal(t, n, KeyGenSeedMinLen)
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(curve, seed)
		require.NoError(t, err)
		sig := make([]byte, ecdsaSigLen[curve])

		// empty hasher
		t.Run("Empty hasher", func(t *testing.T) {
			_, err := sk.Sign(seed, nil)
			assert.Error(t, err)
			assert.True(t, IsNilHasherError(err))
			_, err = sk.PublicKey().Verify(sig, seed, nil)
			assert.Error(t, err)
			assert.True(t, IsNilHasherError(err))
		})

		// hasher with large output size
		t.Run("large size hasher is accepted", func(t *testing.T) {
			dummy := newDummyHasher(500)
			_, err := sk.Sign(seed, dummy)
			assert.NoError(t, err)
			_, err = sk.PublicKey().Verify(sig, seed, dummy)
			assert.NoError(t, err)
		})

		// hasher with small output size
		t.Run("small size hasher is rejected", func(t *testing.T) {
			dummy := newDummyHasher(31) // 31 is one byte less than the supported curves' order
			_, err := sk.Sign(seed, dummy)
			assert.Error(t, err)
			assert.True(t, IsInvalidHasherSizeError(err))
			_, err = sk.PublicKey().Verify(sig, seed, dummy)
			assert.Error(t, err)
			assert.True(t, IsInvalidHasherSizeError(err))
		})
	}
}

// Signing bench
func BenchmarkECDSAP256Sign(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchSign(b, ECDSAP256, halg)
}

// Verifying bench
func BenchmarkECDSAP256Verify(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchVerify(b, ECDSAP256, halg)
}

// Signing bench
func BenchmarkECDSASecp256k1Sign(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchSign(b, ECDSASecp256k1, halg)
}

// Verifying bench
func BenchmarkECDSASecp256k1Verify(b *testing.B) {
	halg := hash.NewSHA3_256()
	benchVerify(b, ECDSASecp256k1, halg)
}

// TestECDSAEncodeDecode tests encoding and decoding of ECDSA keys
func TestECDSAEncodeDecode(t *testing.T) {
	for _, curve := range ecdsaCurves {
		testEncodeDecode(t, curve)
	}
}

// TestECDSAEquals tests equal for ECDSA keys
func TestECDSAEquals(t *testing.T) {
	for i, curve := range ecdsaCurves {
		testEquals(t, curve, ecdsaCurves[i]^1)
	}
}

// TestECDSAUtils tests some utility functions
func TestECDSAUtils(t *testing.T) {

	for _, curve := range ecdsaCurves {
		// generate a key pair
		seed := make([]byte, KeyGenSeedMinLen)
		n, err := crand.Read(seed)
		require.Equal(t, n, KeyGenSeedMinLen)
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(curve, seed)
		require.NoError(t, err)
		testKeysAlgorithm(t, sk, curve)
		testKeySize(t, sk, ecdsaPrKeyLen[curve], ecdsaPubKeyLen[curve])
	}
}

// TestScalarMult is a unit test of the scalar multiplication
// This is only a sanity check meant to make sure the curve implemented
// is checked against an independent test vector
func TestScalarMult(t *testing.T) {
	secp256k1 := secp256k1Instance.curve
	p256 := p256Instance.curve
	genericMultTests := []struct {
		curve elliptic.Curve
		Px    string
		Py    string
		k     string
		Qx    string
		Qy    string
	}{
		{
			secp256k1,
			"858a2ea2498449acf531128892f8ee5eb6d10cfb2f7ebfa851def0e0d8428742",
			"015c59492d794a4f6a3ab3046eecfc85e223d1ce8571aa99b98af6838018286e",
			"6e37a39c31a05181bf77919ace790efd0bdbcaf42b5a52871fc112fceb918c95",
			"fea24b9a6acdd97521f850e782ef4a24f3ef672b5cd51f824499d708bb0c744d",
			"5f0b6db1a2c851cb2959fab5ed36ad377e8b53f1f43b7923f1be21b316df1ea1",
		},
		{
			p256,
			"fa1a85f1ae436e9aa05baabe60eb83b2d7ff52e5766504fda4e18d2d25887481",
			"f7cc347e1ac53f6720ffc511bfb23c2f04c764620be0baf8c44313e92d5404de",
			"6e37a39c31a05181bf77919ace790efd0bdbcaf42b5a52871fc112fceb918c95",
			"28a27fc352f315d5cc562cb0d97e5882b6393fd6571f7d394cc583e65b5c7ffe",
			"4086d17a2d0d9dc365388c91ba2176de7acc5c152c1a8d04e14edc6edaebd772",
		},
	}

	baseMultTests := []struct {
		curve elliptic.Curve
		k     string
		Qx    string
		Qy    string
	}{
		{
			secp256k1,
			"6e37a39c31a05181bf77919ace790efd0bdbcaf42b5a52871fc112fceb918c95",
			"36f292f6c287b6e72ca8128465647c7f88730f84ab27a1e934dbd2da753930fa",
			"39a09ddcf3d28fb30cc683de3fc725e095ec865c3d41aef6065044cb12b1ff61",
		},
		{
			p256,
			"6e37a39c31a05181bf77919ace790efd0bdbcaf42b5a52871fc112fceb918c95",
			"78a80dfe190a6068be8ddf05644c32d2540402ffc682442f6a9eeb96125d8681",
			"3789f92cf4afabf719aaba79ecec54b27e33a188f83158f6dd15ecb231b49808",
		},
	}

	t.Run("scalar mult check", func(t *testing.T) {
		for _, test := range genericMultTests {
			Px, _ := new(big.Int).SetString(test.Px, 16)
			Py, _ := new(big.Int).SetString(test.Py, 16)
			k, _ := new(big.Int).SetString(test.k, 16)
			Qx, _ := new(big.Int).SetString(test.Qx, 16)
			Qy, _ := new(big.Int).SetString(test.Qy, 16)
			Rx, Ry := test.curve.ScalarMult(Px, Py, k.Bytes())
			assert.Equal(t, Rx.Cmp(Qx), 0)
			assert.Equal(t, Ry.Cmp(Qy), 0)
		}
	})

	t.Run("base scalar mult check", func(t *testing.T) {
		for _, test := range baseMultTests {
			k, _ := new(big.Int).SetString(test.k, 16)
			Qx, _ := new(big.Int).SetString(test.Qx, 16)
			Qy, _ := new(big.Int).SetString(test.Qy, 16)
			// base mult
			Rx, Ry := test.curve.ScalarBaseMult(k.Bytes())
			assert.Equal(t, Rx.Cmp(Qx), 0)
			assert.Equal(t, Ry.Cmp(Qy), 0)
			// generic mult with base point
			Px := new(big.Int).Set(test.curve.Params().Gx)
			Py := new(big.Int).Set(test.curve.Params().Gy)
			Rx, Ry = test.curve.ScalarMult(Px, Py, k.Bytes())
			assert.Equal(t, Rx.Cmp(Qx), 0)
			assert.Equal(t, Ry.Cmp(Qy), 0)
		}
	})
}

func TestSignatureFormatCheck(t *testing.T) {

	for _, curve := range ecdsaCurves {
		t.Run("valid signature", func(t *testing.T) {
			len := ecdsaSigLen[curve]
			sig := Signature(make([]byte, len))
			_, err := crand.Read(sig)
			require.NoError(t, err)
			sig[len/2] = 0    // force s to be less than the curve order
			sig[len-1] |= 1   // force s to be non zero
			sig[0] = 0        // force r to be less than the curve order
			sig[len/2-1] |= 1 // force r to be non zero
			valid, err := SignatureFormatCheck(curve, sig)
			assert.Nil(t, err)
			assert.True(t, valid)
		})

		t.Run("invalid length", func(t *testing.T) {
			len := ecdsaSigLen[curve]
			shortSig := Signature(make([]byte, len/2))
			valid, err := SignatureFormatCheck(curve, shortSig)
			assert.Nil(t, err)
			assert.False(t, valid)

			longSig := Signature(make([]byte, len*2))
			valid, err = SignatureFormatCheck(curve, longSig)
			assert.Nil(t, err)
			assert.False(t, valid)
		})

		t.Run("zero values", func(t *testing.T) {
			// signature with a zero s
			len := ecdsaSigLen[curve]
			sig0s := Signature(make([]byte, len))
			_, err := crand.Read(sig0s[:len/2])
			require.NoError(t, err)

			valid, err := SignatureFormatCheck(curve, sig0s)
			assert.Nil(t, err)
			assert.False(t, valid)

			// signature with a zero r
			sig0r := Signature(make([]byte, len))
			_, err = crand.Read(sig0r[len/2:])
			require.NoError(t, err)

			valid, err = SignatureFormatCheck(curve, sig0r)
			assert.Nil(t, err)
			assert.False(t, valid)
		})

		t.Run("large values", func(t *testing.T) {
			len := ecdsaSigLen[curve]
			sigLargeS := Signature(make([]byte, len))
			_, err := crand.Read(sigLargeS[:len/2])
			require.NoError(t, err)
			// make sure s is larger than the curve order
			for i := len / 2; i < len; i++ {
				sigLargeS[i] = 0xFF
			}

			valid, err := SignatureFormatCheck(curve, sigLargeS)
			assert.Nil(t, err)
			assert.False(t, valid)

			sigLargeR := Signature(make([]byte, len))
			_, err = crand.Read(sigLargeR[len/2:])
			require.NoError(t, err)
			// make sure s is larger than the curve order
			for i := 0; i < len/2; i++ {
				sigLargeR[i] = 0xFF
			}

			valid, err = SignatureFormatCheck(curve, sigLargeR)
			assert.Nil(t, err)
			assert.False(t, valid)
		})
	}
}

func TestEllipticUnmarshalSecp256k1(t *testing.T) {

	testVectors := []string{
		"028b10bf56476bf7da39a3286e29df389177a2fa0fca2d73348ff78887515d8da1", // IsOnCurve for elliptic returns false
		"03d39427f07f680d202fe8504306eb29041aceaf4b628c2c69b0ec248155443166", // odd, IsOnCurve for elliptic returns false
		"0267d1942a6cbe4daec242ea7e01c6cdb82dadb6e7077092deb55c845bf851433e", // arith of sqrt in elliptic doesn't match secp256k1
		"0345d45eda6d087918b041453a96303b78c478dce89a4ae9b3c933a018888c5e06", // odd, arith of sqrt in elliptic doesn't match secp256k1
	}

	for _, testVector := range testVectors {

		// get the compressed bytes
		publicBytes, err := hex.DecodeString(testVector)
		require.NoError(t, err)

		// decompress, check that those are perfectly valid Secp256k1 public keys
		retrieved, err := DecodePublicKeyCompressed(ECDSASecp256k1, publicBytes)
		require.NoError(t, err)

		// check the compression is canonical by re-compressing to the same bytes
		require.Equal(t, retrieved.EncodeCompressed(), publicBytes)

		// check that elliptic fails at decompressing them
		x, y := elliptic.UnmarshalCompressed(btcec.S256(), publicBytes)
		require.Nil(t, x)
		require.Nil(t, y)
	}
}

func BenchmarkECDSADecode(b *testing.B) {
	// random message
	seed := make([]byte, 50)
	_, _ = crand.Read(seed)

	for _, curve := range []SigningAlgorithm{ECDSASecp256k1, ECDSAP256} {
		sk, _ := GeneratePrivateKey(curve, seed)
		comp := sk.PublicKey().EncodeCompressed()
		uncomp := sk.PublicKey().Encode()

		b.Run("compressed point on "+curve.String(), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := DecodePublicKeyCompressed(curve, comp)
				require.NoError(b, err)
			}
			b.StopTimer()
		})

		b.Run("uncompressed point on "+curve.String(), func(b *testing.B) {
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				_, err := DecodePublicKey(curve, uncomp)
				require.NoError(b, err)
			}
			b.StopTimer()
		})
	}
}
