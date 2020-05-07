package crypto

import (
	"testing"

	"crypto/elliptic"
	"crypto/rand"
	"math/big"

	"github.com/stretchr/testify/assert"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/stretchr/testify/require"
)

// ECDSA tests
func TestECDSA(t *testing.T) {
	ecdsaCurves := []SigningAlgorithm{
		ECDSAP256,
		ECDSASecp256k1,
	}
	for i, curve := range ecdsaCurves {
		t.Logf("Testing ECDSA for curve %s", curve)
		halg := hash.NewSHA3_256()
		testGenSignVerify(t, ecdsaCurves[i], halg)
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
	ecdsaCurves := []SigningAlgorithm{
		ECDSAP256,
		ECDSASecp256k1,
	}

	for _, curve := range ecdsaCurves {
		testEncodeDecode(t, curve)
	}
}

// TestECDSAEquals tests equal for ECDSA keys
func TestECDSAEquals(t *testing.T) {
	ecdsaCurves := []SigningAlgorithm{
		ECDSAP256,
		ECDSASecp256k1,
	}
	for i, curve := range ecdsaCurves {
		testEquals(t, curve, ecdsaCurves[i]^1)
	}
}

// TestECDSAUtils tests some utility functions
func TestECDSAUtils(t *testing.T) {
	ecdsaCurves := []SigningAlgorithm{
		ECDSAP256,
		ECDSASecp256k1,
	}
	ecdsaSeedLen := []int{
		KeyGenSeedMinLenECDSAP256,
		KeyGenSeedMinLenECDSASecp256k1,
	}
	ecdsaPrKeyLen := []int{
		PrKeyLenECDSAP256,
		PrKeyLenECDSASecp256k1,
	}
	ecdsaPubKeyLen := []int{
		PubKeyLenECDSAP256,
		PubKeyLenECDSASecp256k1,
	}

	for i, curve := range ecdsaCurves {
		// generate a key pair
		seed := make([]byte, ecdsaSeedLen[i])
		n, err := rand.Read(seed)
		require.Equal(t, n, ecdsaSeedLen[i])
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(curve, seed)
		require.NoError(t, err)
		testKeysAlgorithm(t, sk, ecdsaCurves[i])
		testKeySize(t, sk, ecdsaPrKeyLen[i], ecdsaPubKeyLen[i])
	}
}

// TestScalarMult is a unit test of the scalar multiplication 
// This is only a sanity check meant to make sure the curve implemented 
// is checked against an independant test vector 
func TestScalarMult(t *testing.T) {
	secp256k1 := newECDSASecp256k1().curve
	p256 := newECDSAP256().curve
	genericMultTests := []struct {
		curve elliptic.Curve
		Px  string
		Py  string
		k  string
		Qx string
		Qy string
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
		k  string
		Qx string
		Qy string
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
}