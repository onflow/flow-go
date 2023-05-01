//go:build relic
// +build relic

package crypto

import (
	"encoding/hex"
	mrand "math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// G1 and G2 scalar multiplication
func BenchmarkScalarMultG1G2(b *testing.B) {
	seed := make([]byte, securityBits/8)
	_, err := mrand.Read(seed)
	require.NoError(b, err)

	var expo scalar
	_ = mapToFr(&expo, seed)

	// G1 generator multiplication
	b.Run("G1 gen", func(b *testing.B) {
		var res pointE1
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			generatorScalarMultG1(&res, &expo)
		}
		b.StopTimer()
	})

	// G1 base point multiplication
	b.Run("G1 generic", func(b *testing.B) {
		var res pointE1
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			genericScalarMultG1(&res, &expo)
		}
		b.StopTimer()
	})

	// G2 base point multiplication
	b.Run("G2 gen", func(b *testing.B) {
		var res pointE2
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			generatorScalarMultG2(&res, &expo)
		}
		b.StopTimer()
	})
}

// Sanity-check of the map-to-G1 with regards to the IETF draft hash-to-curve
func TestMapToG1(t *testing.T) {

	// test vectors from https://datatracker.ietf.org/doc/html/draft-irtf-cfrg-hash-to-curve-14#appendix-J.9.1
	dst := []byte("QUUX-V01-CS02-with-BLS12381G1_XMD:SHA-256_SSWU_RO_")

	msgs := [][]byte{
		[]byte{},
		[]byte("abc"),
		[]byte("abcdef0123456789"),
		[]byte("q128_qqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqqq"),
		[]byte("a512_aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
	}

	expectedPointString := []string{
		"052926add2207b76ca4fa57a8734416c8dc95e24501772c814278700eed6d1e4e8cf62d9c09db0fac349612b759e79a1",
		"03567bc5ef9c690c2ab2ecdf6a96ef1c139cc0b2f284dca0a9a7943388a49a3aee664ba5379a7655d3c68900be2f6903",
		"11e0b079dea29a68f0383ee94fed1b940995272407e3bb916bbf268c263ddd57a6a27200a784cbc248e84f357ce82d98",
		"15f68eaa693b95ccb85215dc65fa81038d69629f70aeee0d0f677cf22285e7bf58d7cb86eefe8f2e9bc3f8cb84fac488",
		"082aabae8b7dedb0e78aeb619ad3bfd9277a2f77ba7fad20ef6aabdc6c31d19ba5a6d12283553294c1825c4b3ca2dcfe",
	}

	for i, msg := range msgs {
		pointBytes := hashToG1Bytes(msg, dst)

		expectedPointBytes, err := hex.DecodeString(expectedPointString[i])
		require.NoError(t, err)
		// skip comparing the first 3 bits that depend on the serialization scheme
		pointBytes[0] = (expectedPointBytes[0] & 0xE0) | (pointBytes[0] & 0x1F)
		assert.Equal(t, expectedPointBytes, pointBytes, "map to G1 should match the IETF draft test vector")
	}
}

// Hashing to G1 bench
func BenchmarkMapToG1(b *testing.B) {

	input := make([]byte, expandMsgOutput)
	for i := 0; i < len(input); i++ {
		input[i] = byte(i)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mapToG1(input)
	}
	b.StopTimer()
}

// test subgroup membership check in G1 and G2
func TestSubgroupCheck(t *testing.T) {
	prg := getPRG(t)
	seed := make([]byte, securityBits/8)
	_, err := prg.Read(seed)
	require.NoError(t, err)

	/*t.Run("G1", func(t *testing.T) {
		var p pointE1
		randPointG1(&p) // point in G1
		res := checkMembershipG1(&p)
		assert.Equal(t, res, int(valid))
		randPointG1Complement(&p) // point in E1\G1
		res = checkMembershipG1(&p)
		assert.Equal(t, res, int(invalid))
	})*/

	t.Run("G2", func(t *testing.T) {
		var p pointE2
		seed := make([]byte, PubKeyLenBLSBLS12381)
		_, err := mrand.Read(seed)
		require.NoError(t, err)
		mapToG2(&p, seed) // point in G2
		assert.True(t, checkMembershipG2(&p))

		inG2 := false
		for !inG2 {
			_, err := mrand.Read(seed)
			require.NoError(t, err)
			inG2 = mapToG2Complement(&p, seed) // point in E2\G2
		}
		assert.False(t, checkMembershipG2(&p))
	})
}

// subgroup membership check bench
func BenchmarkSubgroupCheck(b *testing.B) {

	/*b.Run("G1", func(b *testing.B) {
		var p pointE1
		randPointG1(&p)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = checkMembershipG1(&p) // G1
		}
		b.StopTimer()
	})*/

	b.Run("G2", func(b *testing.B) {
		var p pointE2
		seed := make([]byte, PubKeyLenBLSBLS12381)
		_, err := mrand.Read(seed)
		require.NoError(b, err)
		mapToG2(&p, seed) // point in G2

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = checkMembershipG2(&p) // G2
		}
		b.StopTimer()
	})

}
