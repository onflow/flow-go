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

// Sanity-check of the map-to-G1 with regards to the IRTF draft hash-to-curve
func TestMapToG1(t *testing.T) {

	// test vectors from https://datatracker.ietf.org/doc/html/draft-irtf-cfrg-hash-to-curve-11#appendix-J.9.1
	dst := []byte("QUUX-V01-CS02-with-BLS12381G1_XMD:SHA-256_SSWU_RO_")

	msgs := [][]byte{
		[]byte("abc"),
		[]byte("abcdef0123456789"),
		[]byte("q128_qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq"),
		[]byte("a512_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}

	expectedPointString := []string{
		"03567bc5ef9c690c2ab2ecdf6a96ef1c139cc0b2f284dca0a9a7943388a49a3aee664ba5379a7655d3c68900be2f6903",
		"11e0b079dea29a68f0383ee94fed1b940995272407e3bb916bbf268c263ddd57a6a27200a784cbc248e84f357ce82d98",
		"15f68eaa693b95ccb85215dc65fa81038d69629f70aeee0d0f677cf22285e7bf58d7cb86eefe8f2e9bc3f8cb84fac488",
		"082aabae8b7dedb0e78aeb619ad3bfd9277a2f77ba7fad20ef6aabdc6c31d19ba5a6d12283553294c1825c4b3ca2dcfe"}

	for i, msg := range msgs {
		pointBytes := hashToG1Bytes(msg, dst)

		expectedPointBytes, err := hex.DecodeString(expectedPointString[i])
		require.NoError(t, err)
		// skip comparing the first 3 bits that depend on the serialization scheme
		pointBytes[0] = (expectedPointBytes[0] & 0xE0) | (pointBytes[0] & 0x1F)
		assert.Equal(t, expectedPointBytes, pointBytes, "map to G1 should match the IRTF draft test vector")
	}
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
