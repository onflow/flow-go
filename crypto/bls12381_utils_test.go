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
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	sk1, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.Nil(t, err)
	sk2, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.Nil(t, err)
	assert.True(t, sk1.Equals(sk2), "private keys should be equal")
}

// test the deterministicity of the relic PRG (used by the DKG polynomials)
func TestPRGseeding(t *testing.T) {
	blsInstance.reInit()
	// 2 scalars generated with the same seed should be equal
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	// 1st scalar (wrapped in a private key)
	err = seedRelic(seed)
	require.Nil(t, err)
	var sk1 PrKeyBLSBLS12381
	randZr(&sk1.scalar)
	// 2nd scalar (wrapped in a private key)
	err = seedRelic(seed)
	require.Nil(t, err)
	var sk2 PrKeyBLSBLS12381
	randZr(&sk2.scalar)
	// compare the 2 scalars (by comparing the private keys)
	assert.True(t, sk1.Equals(&sk2), "private keys should be equal")
}

// G1 and G2 scalar multiplication
func BenchmarkScalarMult(b *testing.B) {
	blsInstance.reInit()
	seed := make([]byte, securityBits/8)
	rand.Read(seed)
	seedRelic(seed)
	var expo scalar
	randZr(&expo)

	// G1 bench
	b.Run("G1", func(b *testing.B) {
		var res pointG1
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			genScalarMultG1(&res, &expo)
		}
		b.StopTimer()
	})

	// G2 bench
	b.Run("G2", func(b *testing.B) {
		var res pointG2
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			genScalarMultG2(&res, &expo)
		}
		b.StopTimer()
	})
}

// Hashing to G1 bench
func BenchmarkHashToG1(b *testing.B) {
	blsInstance.reInit()
	input := make([]byte, minHashSizeBLSBLS12381)
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

// test the optimized SwU algorithm core against a valid test vector.
// The test vector is taken from the original sage implementation
// https://github.com/kwantam/bls12-381_hash from the author of
// https://eprint.iacr.org/2019/403.pdf.
func TestOpSwuHashToG1(t *testing.T) {
	blsInstance.reInit()
	inputs := []string{
		"0e58bd6d947af8aec009ff396cd83a3636614f917423db76e8948e9c25130ae04e721beb924efca3ce585540b2567cf6",
		"0082bd2ed5473b191da55420c9b4df9031a50445b28c17115d614ad6993d7037d6792dd2211e4b485761a6fe2df17582",
		"1243affd90a88d6c1c68748f7855d18acec21331f84abbadbfc13b55e8f9f011c6cffdcce173e4f37841e7ebe2d73f82",
		"027c48089c1c93756b0820f7cec9fcd7d5c31c7c47825eb5e9d90ed9d82fdd31b4aeca2b94d48033a260aa4e0651820e",
	}
	var expected []string
	if serializationG1 == compressed {
		expected = []string{
			"acb46e12d85fc2f7ac9dbb68c3d62d206a2f0a90d85d25c13e3c6fdf8f0b44096c3ba3ecdcd57d95c5ad0727d6025188",
			"b6251e8d37663a78eed9ad6f1a0eb1915733a74acc2e1b4428d63aa78b765786f3ff56f6abace6ae88494f138acf8eca",
			"accd59ffa4cbe6d721d4b4a41c8f12d7d8a9e2bd60e218471c45d6c340feb2b1e193932c4169945f40dc214a9e1766fe",
			"821671a9cbbbf73c429d32bf9a07b64141118a00301d8a1a07de587818d788b37ed0b568c6ede80bd31426bafc142981",
		}
	} else {
		expected = []string{
			"0cb46e12d85fc2f7ac9dbb68c3d62d206a2f0a90d85d25c13e3c6fdf8f0b44096c3ba3ecdcd57d95c5ad0727d6025188192b6d0ab1fba7f71e777764a5b2758b255533ad743d4420892eeb01d4fca183c8fcf8379e8af6b6e10f46152d4fc894",
			"16251e8d37663a78eed9ad6f1a0eb1915733a74acc2e1b4428d63aa78b765786f3ff56f6abace6ae88494f138acf8eca0dac56ebe57e5c34eae9c1016b660b38f7285ff50a1cf1089a50150aaf5a6c96a4fd3950dc49e384f12e45691d9cfac1",
			"0ccd59ffa4cbe6d721d4b4a41c8f12d7d8a9e2bd60e218471c45d6c340feb2b1e193932c4169945f40dc214a9e1766fe192a96341516edac8fefb60bc3a3e208ada81926082c59d2ead61a1c6dd1dc5b92e3ced1aec2816698fb4c0657f9beb9",
			"021671a9cbbbf73c429d32bf9a07b64141118a00301d8a1a07de587818d788b37ed0b568c6ede80bd31426bafc142981065d82aff97cd769c8bcee50616cf88009b26e2ce196af41a6fe5de7b097b5f2acfb429294e7c91a5059f7b2ede6511b",
		}
	}

	output := make([]byte, SignatureLenBLSBLS12381)
	for i, msg := range inputs {
		input, _ := hex.DecodeString(msg)
		OpSwUUnitTest(output, input)
		assert.Equal(t, expected[i], hex.EncodeToString(output), "hash to G1 is not equal to the expected value")
	}
}

// test Bowe subgroup check in G1
// The test compares Bowe's check result to multiplying by the group order
func TestSubgroupCheckG1(t *testing.T) {
	blsInstance.reInit()
	// seed Relic PRG
	seed := make([]byte, securityBits/8)
	rand.Read(seed)
	seedRelic(seed)

	// tests for simple membership check
	t.Run("simple check", func(t *testing.T) {
		simple := 0
		check := checkG1Test(1, simple) // point in G1
		assert.True(t, check)
		check = checkG1Test(0, simple) // point in E1\G1
		assert.False(t, check)
	})

	// tests for Bowe membership check
	t.Run("bowe check", func(t *testing.T) {
		bowe := 1
		check := checkG1Test(1, bowe) // point in G1
		assert.True(t, check)
		check = checkG1Test(0, bowe) // point in E1\G1
		assert.False(t, check)
	})
}

// G1 membership check bench
func BenchmarkCheckG1(b *testing.B) {
	blsInstance.reInit()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchG1Test()
	}
	b.StopTimer()
	return
}
