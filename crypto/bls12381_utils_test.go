package crypto

import (
	"crypto/rand"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Sanity check of G1 and G2 scalar multiplication
func TestScalarMultBLS12381(t *testing.T) {
	expoBytes, err := hex.DecodeString("444465cb6cc2dba9474e6beeb6a9013fbf1260d073429fb14a31e63e89129390")
	require.NoError(t, err)

	var expo scalar
	isZero := mapToFr(&expo, expoBytes)
	require.False(t, isZero)

	// G1 generator multiplication
	// Note that generator and random point multiplications
	// are implemented with the same algorithm
	t.Run("G1", func(t *testing.T) {
		if !isG1Compressed() {
			t.Skip()
		}
		var p pointE1
		generatorScalarMultG1(&p, &expo)
		expected, err := hex.DecodeString("96484ca50719f5d2533047960878b6bae8289646c0f00a942a1e6992be9981a9e0c7a51e9918f9b19d178cf04a8018a4")
		require.NoError(t, err)
		pBytes := make([]byte, g1BytesLen)
		writePointE1(pBytes, &p)
		assert.Equal(t, pBytes, expected)
	})

	// G2 generator multiplication
	// Note that generator and random point multiplications
	// are implemented with the same algorithm
	t.Run("G2", func(t *testing.T) {
		if !isG2Compressed() {
			t.Skip()
		}
		var p pointE2
		generatorScalarMultG2(&p, &expo)
		expected, err := hex.DecodeString("b35f5043f166848805b98da62dcb9c5d2f25e497bd0d9c461d4a00d19e4e67cc1e813de3c99479d5a2c62fb754fd7df40c4fd60c46834c8ae665343a3ff7dc3cc929de34ad62b7b55974f4e3fd20990d3e564b96e4d33de87716052d58cf823e")
		require.NoError(t, err)
		pBytes := make([]byte, g2BytesLen)
		writePointE2(pBytes, &p)
		assert.Equal(t, pBytes, expected)
	})
}

// G1 and G2 scalar multiplication
func BenchmarkScalarMult(b *testing.B) {
	seed := make([]byte, securityBits/8)
	_, err := rand.Read(seed)
	require.NoError(b, err)

	var expo scalar
	_ = mapToFr(&expo, seed)

	// G1 generator multiplication
	// Note that generator and random point multiplications
	// are implemented with the same algorithm
	var res pointE1
	b.Run("G1 gen", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			generatorScalarMultG1(&res, &expo)
		}
	})

	// E1 random point multiplication
	// Note that generator and random point multiplications
	// are implemented with the same algorithm
	b.Run("E1 rand", func(b *testing.B) {
		var res pointE1
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res.scalarMultE1(&res, &expo)
		}
	})

	// G2 generator multiplication
	// Note that generator and random point multiplications
	// are implemented with the same algorithm
	b.Run("G2 gen", func(b *testing.B) {
		var res pointE2
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			generatorScalarMultG2(&res, &expo)
		}
	})
}

// Sanity-check of the map-to-G1 with regards to the IETF draft hash-to-curve
func TestMapToG1(t *testing.T) {
	if !isG1Compressed() {
		t.Skip()
	}
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
		require.NotNil(t, pointBytes)

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
	var p *pointE1
	for i := 0; i < b.N; i++ {
		p = mapToG1(input)
	}
	require.NotNil(b, p)
}

// test subgroup membership check in G1 and G2
func TestSubgroupCheck(t *testing.T) {
	prg := getPRG(t)
	seed := make([]byte, 192)
	_, err := prg.Read(seed)
	require.NoError(t, err)

	t.Run("G1", func(t *testing.T) {
		var p pointE1
		unsafeMapToG1(&p, seed) // point in G1
		assert.True(t, checkMembershipG1(&p))

		unsafeMapToG1Complement(&p, seed) // point in E2\G2
		assert.False(t, checkMembershipG1(&p))
	})

	t.Run("G2", func(t *testing.T) {
		var p pointE2
		unsafeMapToG2(&p, seed) // point in G2
		assert.True(t, checkMembershipG2(&p))

		unsafeMapToG2Complement(&p, seed) // point in E2\G2
		assert.False(t, checkMembershipG2(&p))
	})
}

// subgroup membership check bench
func BenchmarkSubgroupCheck(b *testing.B) {
	seed := make([]byte, g2BytesLen)
	_, err := rand.Read(seed)
	require.NoError(b, err)

	b.Run("G1", func(b *testing.B) {
		var p pointE1
		unsafeMapToG1(&p, seed) // point in G1
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = checkMembershipG1(&p) // G1
		}
	})

	b.Run("G2", func(b *testing.B) {
		var p pointE2
		unsafeMapToG2(&p, seed) // point in G2
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = checkMembershipG2(&p) // G2
		}
	})
}

// specific test of G1 points Encode and decode (BLS signature since the library is set for min_sig).
// G2 points read and write are implicitly tested by public keys Encode/Decode.
func TestReadWriteG1(t *testing.T) {
	prg := getPRG(t)
	seed := make([]byte, frBytesLen)
	bytes := make([]byte, g1BytesLen)
	// generate a random G1 point, encode it, decode it,
	// and compare it the original point
	t.Run("random points", func(t *testing.T) {
		iterations := 50
		for i := 0; i < iterations; i++ {
			var p, q pointE1
			_, err := prg.Read(seed)
			unsafeMapToG1(&p, seed)
			require.NoError(t, err)
			writePointE1(bytes, &p)
			err = readPointE1(&q, bytes)
			require.NoError(t, err)
			assert.True(t, p.equals(&q))
		}
	})

	t.Run("infinity", func(t *testing.T) {
		var p, q pointE1
		seed := make([]byte, frBytesLen)
		unsafeMapToG1(&p, seed) // this results in the infinity point given how `unsafeMapToG1` works with an empty scalar
		writePointE1(bytes, &p)
		require.True(t, IsBLSSignatureIdentity(bytes)) // sanity check
		err := readPointE1(&q, bytes)
		require.NoError(t, err)
		assert.True(t, p.equals(&q))
	})
}

// test some edge cases of MapToFr to validate modular reduction and endianness:
//   - inputs `0` and curve order `r`
//   - inputs `1` and `r+1`
func TestMapToFr(t *testing.T) {
	var x scalar
	offset := 10
	bytes := make([]byte, frBytesLen+offset)
	expectedEncoding := make([]byte, frBytesLen)
	// zero bytes
	isZero := mapToFr(&x, bytes)
	assert.True(t, isZero)
	assert.True(t, x.isZero())
	assert.Equal(t, expectedEncoding, newPrKeyBLSBLS12381(&x).Encode())
	// curve order bytes
	copy(bytes[offset:], BLS12381Order)
	isZero = mapToFr(&x, bytes)
	assert.True(t, isZero)
	assert.True(t, x.isZero())
	assert.Equal(t, expectedEncoding, newPrKeyBLSBLS12381(&x).Encode())
	// curve order + 1
	g1, err := hex.DecodeString("824aa2b2f08f0a91260805272dc51051c6e47ad4fa403b02b4510b647ae3d1770bac0326a805bbefd48056c8c121bdb813e02b6052719f607dacd3a088274f65596bd0d09920b61ab5da61bbdc7f5049334cf11213945d57e5ac7d055d042b7e")
	require.NoError(t, err)
	bytes[len(bytes)-1] += 1
	isZero = mapToFr(&x, bytes)
	assert.False(t, isZero)
	assert.False(t, x.isZero())
	expectedEncoding[frBytesLen-1] = 1
	sk := newPrKeyBLSBLS12381(&x)
	assert.Equal(t, expectedEncoding, sk.Encode())
	// check scalar is equal to "1" in the lower layer (scalar multiplication)
	assert.Equal(t, sk.PublicKey().Encode(), g1, "scalar should be 1, check endianness in the C layer")
	// 1
	copy(bytes[offset:], expectedEncoding)
	isZero = mapToFr(&x, bytes)
	assert.False(t, isZero)
	assert.False(t, x.isZero())
	expectedEncoding[frBytesLen-1] = 1
	sk = newPrKeyBLSBLS12381(&x)
	assert.Equal(t, expectedEncoding, sk.Encode())
	// check scalar is equal to "1" in the lower layer (scalar multiplication)
	assert.Equal(t, sk.PublicKey().Encode(), g1, "scalar should be 1, check endianness in the C layer")
}
