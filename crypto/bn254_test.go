package crypto

import (
	"crypto/rand"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/big"
	mrand "math/rand"
	"testing"
	"time"
)

func BigFromBase16(s string) *big.Int {
	if s[:2] == "0x" || s[:2] == "0X" {
		s = s[2:]
	}
	n, _ := new(big.Int).SetString(s, 16)
	return n
}

func TestBN256MainMethods(t *testing.T) {
	t.Run("sign and verify", func(t *testing.T) {
		k256 := hash.NewKeccak_256()
		sk := "0x15a8efb503f46667a47aafa8d6f2ac105c72d3af6ff95068c7ec91ab32b00e89"
		bsk := BigFromBase16(sk)
		s := BigToBytes(bsk, 0)
		priKey, err := DecodePrivateKey(BLSBN256, s)
		require.NoError(t, err)
		pubKey := priKey.PublicKey()

		randomPriKey, err := GeneratePrivateKey(BLSBN256, nil)
		randomPubKey := randomPriKey.PublicKey()
		require.NoError(t, err)
		// random message
		m := make([]byte, 100)
		_, err = rand.Read(m)
		require.NoError(t, err)
		rs, err := randomPriKey.Sign(m, k256)
		require.NoError(t, err)

		ok, err := randomPubKey.Verify(rs, m, k256)
		require.NoError(t, err)
		require.True(t, ok)

		tests := [][3]string{
			{"", "0x1eb14c42f5450b0fad22238b3e2822c701c5f2b81d675f91cbaa635c5e110180", "0x03a919f2f4db5045616849e6dbc01278d0fcb7c7beb79945ad2cddfc75fc0360"},
			{"test", "0x21018644bc20df8dc208abc632ba1b5a3d3131b9358bf043303a5f796e9ea42d", "0x0986031ed2cae311f566f4a652a21ab07a450652d8a761296c6dae36649d7eeb"},
			{"address: 0x0000000000000000000000000000000000000000", "0x2c4cec8195f908b892fe0234d4e67c91408d75635bbee7c952f9264d373572b8", "0x02ff60f96d5211b83d94a01e426efaf2b4a5cb91d3cfda82dcec88afc31c6279"},
			{"address: 0x0000000000000000000000000000000000000001", "0x2c18ab45e99cbf21500865b296a1cf5654dfecfc40761a0f7cfce48509e29ac6", "0x1e06068808754a9375a0430621836e42b0546d05831f65bbf1d4e08da1413180"},
			{"address: 0x0000000000000000000000000000000000000002", "0x2dc69a9bc0ed17e2969bf25e2adf12dd19e62c516ab44e745055de98df9acbb0", "0x202c5b593c3c739e225291de6d64cc6de36fdef33b1b9207adb945e2fbf588c8"},
			{"address: 0x0000000000000000000000000000000000000003", "0x27c9686c7e590e681fd141df081a9d5e59bfc4869e74e2f1bcb3076ee3de2356", "0x0488a16d7f38b899f8d561f1ca230ef80183f5d7a9afad64d38e89ae0a1d9a7f"},
			{"address: 0x0000000000000000000000000000000000000004", "0x06aa97ae15fe2aeb23731780ed6871302afe9847ea9dceaf2ed4ac5c88412b8b", "0x01fc0f07def4ed8a8459a58b1e91f5359084b1a3c4821f4e42049189db0fe2f0"},
			{"symbols: ~!@#.$%^&*()_+-=", "0x0a58087c06ba326bfb869f687bdd1d3d7b91777cdd6f1b147510fb12e52ca88a", "0x27a83aa5e87c47474c97a8eb8e83c11eddca6abd4ee5b49b6401f3714296cd6e"},
		}

		for _, pair := range tests {
			m := []byte(pair[0])
			s, err := priKey.Sign(m, k256)
			require.NoError(t, err)

			ok, err := pubKey.Verify(s, m, k256)
			require.NoError(t, err)
			require.True(t, ok)

			point, err := GetBN256SignaturePoint(s)
			require.NoError(t, err)
			ps := Point2BigInt(point)
			// println(PointToStringG1(s.GetPoint()))
			assert.Equal(t, pair[1], BigToHex32(ps[0]), string(m))
			assert.Equal(t, pair[2], BigToHex32(ps[1]), string(m))

			// invalid msg
			badM := append(m, "a"...)
			ok1, err := pubKey.Verify(s, badM, k256)
			require.NoError(t, err)
			require.False(t, ok1)
			// invalid signature
			badS, err := priKey.Sign(badM, k256)
			require.NoError(t, err)
			ok2, err := pubKey.Verify(badS, m, k256)
			require.NoError(t, err)
			require.False(t, ok2)
			// the right one
			ok4, err := pubKey.Verify(s, m, k256)
			require.NoError(t, err)
			require.True(t, ok4)
		}
	})
}

// BLS multi-signature
// signature aggregation sanity check
//
// Aggregate n signatures of the same message under different keys, and compare
// it against the signature of the message under an aggregated private key.
// Verify the aggregated signature using the multi-signature verification with
// one message.
func TestBN256AggregateSignatures(t *testing.T) {
	// random message
	input := make([]byte, 100)
	_, err := rand.Read(input)
	require.NoError(t, err)
	// hasher
	k256 := hash.NewKeccak_256()
	// number of signatures to aggregate
	r := time.Now().UnixNano()
	mrand.Seed(r)
	t.Logf("math rand seed is %d", r)
	sigsNum := mrand.Intn(100) + 1
	sigs := make([]Signature, 0, sigsNum)
	sks := make([]PrivateKey, 0, sigsNum)
	pks := make([]PublicKey, 0, sigsNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	var aggSig, expectedSig Signature

	// create the signatures
	for i := 0; i < sigsNum; i++ {
		sk, err := GeneratePrivateKey(BLSBN256, nil)
		require.NoError(t, err)
		s, err := sk.Sign(input, k256)
		require.NoError(t, err)
		sigs = append(sigs, s)
		sks = append(sks, sk)
		pks = append(pks, sk.PublicKey())
	}

	// all signatures are valid
	t.Run("all valid signatures", func(t *testing.T) {
		for i := 0; i < len(sigs); i++ {
			ok, err := pks[i].Verify(sigs[i], input, k256)
			require.NoError(t, err)
			require.True(t, ok)
		}
		// aggregate private keys
		aggSk, err := AggregateBN256PrivateKeys(sks)
		require.NoError(t, err)
		expectedSig, err := aggSk.Sign(input, k256)
		require.NoError(t, err)
		// aggregate signatures
		aggSig, err := AggregateBN256Signatures(sigs)
		require.NoError(t, err)
		// First check: check the signatures are equal
		assert.Equal(t, aggSig, expectedSig,
			"incorrect signature %s, should be %s, private keys are %s, input is %x",
			aggSig, expectedSig, sks, input)
		// Second check: Verify the aggregated signature
		valid, err := VerifyBN256SignatureOneMessage(pks, aggSig, input, k256)
		require.NoError(t, err)
		assert.True(t, valid,
			"Verification of %s failed, signature should be %s private keys are %s, input is %x",
			aggSig, expectedSig, sks, input)
	})

	// check if one signature is not correct
	t.Run("one invalid signature", func(t *testing.T) {
		input[0] ^= 1
		randomIndex := mrand.Intn(sigsNum)
		sigs[randomIndex], err = sks[randomIndex].Sign(input, k256)
		input[0] ^= 1
		aggSig, err = AggregateBN256Signatures(sigs)
		require.NoError(t, err)
		assert.NotEqual(t, aggSig, expectedSig,
			"signature %s shouldn't be %s private keys are %s, input is %x",
			aggSig, expectedSig, sks, input)
		valid, err := VerifyBN256SignatureOneMessage(pks, aggSig, input, k256)
		require.NoError(t, err)
		assert.False(t, valid,
			"verification of signature %s should fail, it shouldn't be %s private keys are %s, input is %x",
			aggSig, expectedSig, sks, input)
		sigs[randomIndex], err = sks[randomIndex].Sign(input, k256)
		require.NoError(t, err)
	})

	// check if one the public keys is not correct
	t.Run("one invalid public key", func(t *testing.T) {
		randomIndex := mrand.Intn(sigsNum)
		newSk := randomSK(t, seed)
		sks[randomIndex] = newSk
		pks[randomIndex] = newSk.PublicKey()
		aggSk, err := AggregateBN256PrivateKeys(sks)
		require.NoError(t, err)
		expectedSig, err = aggSk.Sign(input, k256)
		require.NoError(t, err)
		assert.NotEqual(t, aggSig, expectedSig,
			"signature %s shouldn't be %s, private keys are %s, input is %x, wrong key is of index %d",
			aggSig, expectedSig, sks, input, randomIndex)
		valid, err := VerifyBN256SignatureOneMessage(pks, aggSig, input, k256)
		require.NoError(t, err)
		assert.False(t, valid,
			"signature %s should fail, shouldn't be %s, private keys are %s, input is %x, wrong key is of index %d",
			aggSig, expectedSig, sks, input, randomIndex)
	})

	t.Run("invalid inputs", func(t *testing.T) {
		// test aggregating an empty signature list
		aggSig, err = AggregateBN256Signatures(sigs[:0])
		assert.Error(t, err)
		assert.True(t, IsBN256AggregateEmptyListError(err))
		assert.Nil(t, aggSig)

		// test verification with an empty key list
		result, err := VerifyBN256SignatureOneMessage(pks[:0], aggSig, input, k256)
		assert.Error(t, err)
		assert.True(t, IsBN256AggregateEmptyListError(err))
		assert.False(t, result)

		// test the empty key list
		aggSk, err := AggregateBN256PrivateKeys(sks[:0])
		assert.Error(t, err)
		assert.True(t, IsBN256AggregateEmptyListError(err))
		assert.Nil(t, aggSk)
	})
}

// Bench of all aggregation functions
func BenchmarkBN256Aggregate(b *testing.B) {
	// random message
	input := make([]byte, 100)
	_, _ = mrand.Read(input)
	// hasher
	k256 := hash.NewKeccak_256()
	sigsNum := 1000
	sigs := make([]Signature, 0, sigsNum)
	sks := make([]PrivateKey, 0, sigsNum)
	pks := make([]PublicKey, 0, sigsNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)

	// create the signatures
	for i := 0; i < sigsNum; i++ {
		_, _ = mrand.Read(seed)
		sk, err := GeneratePrivateKey(BLSBN256, seed)
		require.NoError(b, err)
		s, err := sk.Sign(input, k256)
		if err != nil {
			b.Fatal()
		}
		sigs = append(sigs, s)
		sks = append(sks, sk)
		pks = append(pks, sk.PublicKey())
	}

	// private keys
	b.Run("PrivateKeys", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := AggregateBN256PrivateKeys(sks)
			require.NoError(b, err)
		}
		b.StopTimer()
	})

	// public keys
	b.Run("PublicKeys", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := AggregateBN256PublicKeys(pks)
			require.NoError(b, err)
		}
		b.StopTimer()
	})

	// signatures
	b.Run("Signatures", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := AggregateBN256Signatures(sigs)
			require.NoError(b, err)
		}
		b.StopTimer()
	})
}

func randomSK(t *testing.T, seed []byte) PrivateKey {
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(BLSBN256, seed)
	require.NoError(t, err)
	return sk
}
