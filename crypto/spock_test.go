//go:build relic
// +build relic

package crypto

import (
	crand "crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSPOCKProveVerifyAgainstData(t *testing.T) {
	// test the consistency with different data
	seed := make([]byte, KeyGenSeedMinLen)
	data := make([]byte, 100)

	n, err := crand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLen)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.NoError(t, err)
	_, err = crand.Read(data)
	require.NoError(t, err)

	// generate a SPoCK proof
	kmac := NewExpandMsgXOFKMAC128("spock test")
	s, err := SPOCKProve(sk, data, kmac)
	require.NoError(t, err)
	pk := sk.PublicKey()

	// SPoCK verify against the data (happy path)
	t.Run("correctness check", func(t *testing.T) {
		result, err := SPOCKVerifyAgainstData(pk, s, data, kmac)
		require.NoError(t, err)
		assert.True(t, result,
			"Verification should succeed:\n signature:%s\n message:%s\n private key:%s", s, data, sk)
	})

	// test with a different message (unhappy path)
	t.Run("invalid message", func(t *testing.T) {
		data[0] ^= 1
		result, err := SPOCKVerifyAgainstData(pk, s, data, kmac)
		require.NoError(t, err)
		assert.False(t, result,
			"Verification should fail:\n signature:%s\n message:%s\n private key:%s", s, data, sk)
		data[0] ^= 1
	})

	// test with a valid but different key (unhappy path)
	t.Run("invalid key", func(t *testing.T) {
		seed[0] ^= 1
		wrongSk, err := GeneratePrivateKey(BLSBLS12381, seed)
		require.NoError(t, err)
		result, err := SPOCKVerifyAgainstData(wrongSk.PublicKey(), s, data, kmac)
		require.NoError(t, err)
		assert.False(t, result,
			"Verification should fail:\n signature:%s\n message:%s\n private key:%s", s, data, sk)
	})

	// test with an invalid key type
	t.Run("invalid key type", func(t *testing.T) {
		wrongSk := invalidSK(t)
		result, err := SPOCKVerifyAgainstData(wrongSk.PublicKey(), s, data, kmac)
		require.Error(t, err)
		assert.True(t, IsNotBLSKeyError(err))
		assert.False(t, result)
	})

	// test with an identity public key
	t.Run("identity proof", func(t *testing.T) {
		// verifying with a pair of (proof, publicKey) equal to (identity_signature, identity_key) should
		// return false
		identityProof := identityBLSSignature
		result, err := SPOCKVerifyAgainstData(IdentityBLSPublicKey(), identityProof, data, kmac)
		assert.NoError(t, err)
		assert.False(t, result)
	})
}

// tests of happy and unhappy paths of SPOCKVerify
func TestSPOCKProveVerify(t *testing.T) {
	// test the consistency with different data
	seed1 := make([]byte, KeyGenSeedMinLen)
	seed2 := make([]byte, KeyGenSeedMinLen)
	data := make([]byte, 100)

	// data
	_, err := crand.Read(data)
	require.NoError(t, err)
	// sk1
	n, err := crand.Read(seed1)
	require.Equal(t, n, KeyGenSeedMinLen)
	require.NoError(t, err)
	sk1, err := GeneratePrivateKey(BLSBLS12381, seed1)
	require.NoError(t, err)
	// sk2
	n, err = crand.Read(seed2)
	require.Equal(t, n, KeyGenSeedMinLen)
	require.NoError(t, err)
	sk2, err := GeneratePrivateKey(BLSBLS12381, seed2)
	require.NoError(t, err)

	// generate SPoCK proofs
	kmac := NewExpandMsgXOFKMAC128("spock test")
	pr1, err := SPOCKProve(sk1, data, kmac)
	require.NoError(t, err)
	pr2, err := SPOCKProve(sk2, data, kmac)
	require.NoError(t, err)

	// SPoCK verify against the data, happy path
	t.Run("correctness check", func(t *testing.T) {
		result, err := SPOCKVerify(sk1.PublicKey(), pr1, sk2.PublicKey(), pr2)
		require.NoError(t, err)
		assert.True(t, result,
			"Verification should succeed:\n proofs:%s\n %s\n private keys:%s\n %s\n data:%x",
			pr1, pr2, sk1, sk2, data)
	})

	// test with a different message, verification should fail for proofs
	// of different messages.
	t.Run("inconsistent proofs", func(t *testing.T) {
		data[0] ^= 1 // alter the data
		pr2bis, err := SPOCKProve(sk2, data, kmac)
		require.NoError(t, err)
		result, err := SPOCKVerify(sk1.PublicKey(), pr1, sk2.PublicKey(), pr2bis)
		require.NoError(t, err)
		assert.False(t, result,
			"Verification should fail:\n proofs:%s\n %s\n private keys:%s\n %s \n data:%x",
			pr1, pr2bis, sk1, sk2, data)
		data[0] ^= 1 // restore the data
	})

	// test with a different key, verification should fail if the public keys are not
	// matching the private keys used to generate the proofs.
	t.Run("invalid public key", func(t *testing.T) {
		seed2[0] ^= 1 // alter the seed
		sk2bis, err := GeneratePrivateKey(BLSBLS12381, seed2)
		require.NoError(t, err)
		result, err := SPOCKVerify(sk1.PublicKey(), pr1, sk2bis.PublicKey(), pr2)
		require.NoError(t, err)
		assert.False(t, result,
			"Verification should succeed:\n proofs:%s\n %s\n private keys:%s\n %s \n data:%s",
			pr1, pr2, sk1, sk2bis, data)
	})

	// test with an invalid key type
	t.Run("invalid key type", func(t *testing.T) {
		wrongSk := invalidSK(t)

		pr, err := SPOCKProve(wrongSk, data, kmac)
		require.Error(t, err)
		assert.True(t, IsNotBLSKeyError(err))
		assert.Nil(t, pr)

		result, err := SPOCKVerify(wrongSk.PublicKey(), pr1, sk2.PublicKey(), pr2)
		require.Error(t, err)
		assert.True(t, IsNotBLSKeyError(err))
		assert.False(t, result)

		result, err = SPOCKVerify(sk1.PublicKey(), pr1, wrongSk.PublicKey(), pr2)
		require.Error(t, err)
		assert.True(t, IsNotBLSKeyError(err))
		assert.False(t, result)
	})

	// test with identity public key and proof
	t.Run("identity proof", func(t *testing.T) {
		// verifying with either pair of (proof, publicKey) equal to (identity_signature, identity_key) should
		// return falsen with any other (proof, key) pair.
		identityProof := identityBLSSignature
		result, err := SPOCKVerify(IdentityBLSPublicKey(), identityProof, sk2.PublicKey(), pr2)
		assert.NoError(t, err)
		assert.False(t, result)

		result, err = SPOCKVerify(sk1.PublicKey(), pr1, IdentityBLSPublicKey(), identityProof)
		assert.NoError(t, err)
		assert.False(t, result)

		result, err = SPOCKVerify(IdentityBLSPublicKey(), identityProof, IdentityBLSPublicKey(), identityProof)
		assert.NoError(t, err)
		assert.False(t, result)
	})
}
