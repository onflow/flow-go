// +build relic

package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeterministicKeyGen(t *testing.T) {
	// 2 keys generated with the same seed should be equal
	seed := make([]byte, KeyGenSeedMinLenBLS_BLS12381)
	rand.Read(seed)
	sk1, err := GeneratePrivateKey(BLS_BLS12381, seed)
	require.Nil(t, err)
	sk2, err := GeneratePrivateKey(BLS_BLS12381, seed)
	require.Nil(t, err)
	assert.True(t, sk1.Equals(sk2), "private keys should be equal")
}

// test the deterministicity of the relic PRG (used by the DKG polynimials)
func TestPRGseeding(t *testing.T) {
	NewSigner(BLS_BLS12381)
	// 2 scalars generated with the same seed should be equal
	seed := make([]byte, KeyGenSeedMinLenBLS_BLS12381)
	rand.Read(seed)
	// 1st scalar (wrapped in a private key)
	err := seedRelic(seed)
	require.Nil(t, err)
	var sk1 PrKeyBLS_BLS12381
	err = randZr(&sk1.scalar)
	require.Nil(t, err)
	// 2nd scalar (wrapped in a private key)
	err = seedRelic(seed)
	require.Nil(t, err)
	var sk2 PrKeyBLS_BLS12381
	err = randZr(&sk2.scalar)
	require.Nil(t, err)
	// compare the 2 scalars (by comparing the private keys)
	assert.True(t, sk1.Equals(&sk2), "private keys should be equal")
}

// TestG1 helps debugging but is not a unit test
func TestG1(t *testing.T) {
	NewSigner(BLS_BLS12381)
	seed := make([]byte, securityBits/8)
	rand.Read(seed)
	seedRelic(seed)
	var expo scalar
	randZr(&expo)
	var res pointG1
	_G1scalarGenMult(&res, &expo)

}

// G1 bench
func BenchmarkG1(b *testing.B) {
	NewSigner(BLS_BLS12381)
	seed := make([]byte, securityBits/8)
	rand.Read(seed)
	seedRelic(seed)
	var expo scalar
	randZr(&expo)
	var res pointG1

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_G1scalarGenMult(&res, &expo)
	}
	b.StopTimer()
	return
}

// TestG2 helps debugging but is not a unit test
func TestG2(t *testing.T) {
	NewSigner(BLS_BLS12381)
	var expo scalar
	(&expo).setInt(1)
	var res pointG2
	_G2scalarGenMult(&res, &expo)

}

// G2 bench
func BenchmarkG2(b *testing.B) {
	NewSigner(BLS_BLS12381)
	seed := make([]byte, securityBits/8)
	rand.Read(seed)
	seedRelic(seed)
	var expo scalar
	randZr(&expo)
	var res pointG2

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_G2scalarGenMult(&res, &expo)
	}
	b.StopTimer()
	return
}

// Hashing to G1 bench
func BenchmarkHashToG1(b *testing.B) {
	NewSigner(BLS_BLS12381)
	input := make([]byte, OpSwUInputLenBLS_BLS12381)
	for i := 0; i < len(input); i++ {
		input[i] = byte(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hashToG1(input)
	}
	b.StopTimer()
	return
}

// test the optimized SwU algorithm core against a valid test vector
// the test vector is taken from the original implementation used by the authors of
// the paper https://eprint.iacr.org/2019/403.pdf
func TestOpSwuHashToG1(t *testing.T) {
	NewSigner(BLS_BLS12381)
	inputs := []string{
		"0e58bd6d947af8aec009ff396cd83a3636614f917423db76e8948e9c25130ae04e721beb924efca3ce585540b2567cf6",
		"0082bd2ed5473b191da55420c9b4df9031a50445b28c17115d614ad6993d7037d6792dd2211e4b485761a6fe2df17582",
		"1243affd90a88d6c1c68748f7855d18acec21331f84abbadbfc13b55e8f9f011c6cffdcce173e4f37841e7ebe2d73f82",
		"027c48089c1c93756b0820f7cec9fcd7d5c31c7c47825eb5e9d90ed9d82fdd31b4aeca2b94d48033a260aa4e0651820e",
	}
	expected := []string{
		"8cb46e12d85fc2f7ac9dbb68c3d62d206a2f0a90d85d25c13e3c6fdf8f0b44096c3ba3ecdcd57d95c5ad0727d6025188",
		"b6251e8d37663a78eed9ad6f1a0eb1915733a74acc2e1b4428d63aa78b765786f3ff56f6abace6ae88494f138acf8eca",
		"accd59ffa4cbe6d721d4b4a41c8f12d7d8a9e2bd60e218471c45d6c340feb2b1e193932c4169945f40dc214a9e1766fe",
		"a21671a9cbbbf73c429d32bf9a07b64141118a00301d8a1a07de587818d788b37ed0b568c6ede80bd31426bafc142981",
	}

	output := make([]byte, SignatureLenBLS_BLS12381)
	for i, msg := range inputs {
		input, _ := hex.DecodeString(msg)
		OpSwUUnitTest(output, input)
		assert.Equal(t, hex.EncodeToString(output), expected[i], "hash to G1 is not equal to the expected value")
	}
	return
}
