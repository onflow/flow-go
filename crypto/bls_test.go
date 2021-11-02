// +build relic

package crypto

import (
	"crypto/rand"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto/hash"
)

// BLS tests
func TestBLSBLS12381(t *testing.T) {
	halg := NewBLSKMAC("test tag")
	// test the key generation seed lengths
	testKeyGenSeed(t, BLSBLS12381, KeyGenSeedMinLenBLSBLS12381, KeyGenSeedMaxLenBLSBLS12381)
	// test the consistency with different inputs
	testGenSignVerify(t, BLSBLS12381, halg)
}

// Signing bench
func BenchmarkBLSBLS12381Sign(b *testing.B) {
	halg := NewBLSKMAC("bench tag")
	benchSign(b, BLSBLS12381, halg)
}

// Verifying bench
func BenchmarkBLSBLS12381Verify(b *testing.B) {
	halg := NewBLSKMAC("bench tag")
	benchVerify(b, BLSBLS12381, halg)
}

// utility function to generate a random BLS private key
func randomSK(t *testing.T, seed []byte) PrivateKey {
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.NoError(t, err)
	return sk
}

// utility function to generate a non BLS private key
func invalidSK(t *testing.T) PrivateKey {
	seed := make([]byte, KeyGenSeedMinLenECDSAP256)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenECDSAP256)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(ECDSAP256, seed)
	require.NoError(t, err)
	return sk
}

// BLS tests
func TestBLSBLS12381Hasher(t *testing.T) {
	// generate a key pair
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	sk := randomSK(t, seed)
	sig := make([]byte, SignatureLenBLSBLS12381)

	// empty hasher
	t.Run("Empty hasher", func(t *testing.T) {
		_, err := sk.Sign(seed, nil)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		_, err = sk.PublicKey().Verify(sig, seed, nil)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
	})

	// short size hasher
	t.Run("short size hasher", func(t *testing.T) {
		s, err := sk.Sign(seed, hash.NewSHA2_256())
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, s)

		valid, err := sk.PublicKey().Verify(sig, seed, hash.NewSHA2_256())
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, valid)
	})

	t.Run("NewBLSKMAC sanity check", func(t *testing.T) {
		// test the parameter lengths of "NewBLSKMAC" are in the correct range
		// h is nil if the kamc inputs are invalid
		h := internalBLSKMAC("test")
		assert.NotNil(t, h)

		// test the application and PoP prefixes are different and have the same length
		assert.NotEqual(t, applicationTagPrefix, popTagPrefix)
		assert.Equal(t, len(applicationTagPrefix), len(popTagPrefix))
	})
}

// TestBLSEncodeDecode tests encoding and decoding of BLS keys
func TestBLSEncodeDecode(t *testing.T) {
	// generic tests
	testEncodeDecode(t, BLSBLS12381)

	// specific tests for BLS

	//  zero private key
	skBytes := make([]byte, PrKeyLenBLSBLS12381)
	sk, err := DecodePrivateKey(BLSBLS12381, skBytes)
	require.Error(t, err, "the key decoding should fail - key value is zero")
	assert.True(t, IsInvalidInputsError(err))
	assert.Nil(t, sk)

	//  identity public key
	pkBytes := make([]byte, PubKeyLenBLSBLS12381)
	pkBytes[0] = 0xC0
	pk, err := DecodePublicKey(BLSBLS12381, pkBytes)
	require.Error(t, err, "the key decoding should fail - key value is identity")
	assert.True(t, IsInvalidInputsError(err))
	assert.Nil(t, pk)

	// invalid point
	pkBytes = make([]byte, PubKeyLenBLSBLS12381)
	pkBytes[0] = invalidBLSSignatureHeader
	pk, err = DecodePublicKey(BLSBLS12381, pkBytes)
	require.Error(t, err, "the key decoding should fail - key value is invalid")
	assert.True(t, IsInvalidInputsError(err))
	assert.Nil(t, pk)
}

// TestBLSEquals tests equal for BLS keys
func TestBLSEquals(t *testing.T) {
	testEquals(t, BLSBLS12381, ECDSAP256)
}

// TestBLSUtils tests some utility functions
func TestBLSUtils(t *testing.T) {
	// generate a key pair
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	sk := randomSK(t, seed)
	// test Algorithm()
	testKeysAlgorithm(t, sk, BLSBLS12381)
	// test Size()
	testKeySize(t, sk, PrKeyLenBLSBLS12381, PubKeyLenBLSBLS12381)
}

// BLS Proof of Possession test
func TestBLSPOP(t *testing.T) {
	r := time.Now().UnixNano()
	mrand.Seed(r)
	t.Logf("math rand seed is %d", r)
	// make sure the length is larger than minimum lengths of all the signaure algos
	seedMinLength := 48
	seed := make([]byte, seedMinLength)
	input := make([]byte, 100)

	t.Run("PoP tests", func(t *testing.T) {
		loops := 10
		for j := 0; j < loops; j++ {
			n, err := mrand.Read(seed)
			require.Equal(t, n, seedMinLength)
			require.NoError(t, err)
			sk, err := GeneratePrivateKey(BLSBLS12381, seed)
			require.NoError(t, err)
			_, err = mrand.Read(input)
			require.NoError(t, err)
			s, err := BLSGeneratePOP(sk)
			require.NoError(t, err)
			pk := sk.PublicKey()

			// test a valid PoP
			result, err := BLSVerifyPOP(pk, s)
			require.NoError(t, err)
			assert.True(t, result, "Verification should succeed:\n signature:%s\n private key:%s", s, sk)

			// test with a valid but different key
			seed[0] ^= 1
			wrongSk, err := GeneratePrivateKey(BLSBLS12381, seed)
			require.NoError(t, err)
			result, err = BLSVerifyPOP(wrongSk.PublicKey(), s)
			require.NoError(t, err)
			assert.False(t, result, "Verification should fail:\n signature:%s\n private key:%s", s, sk)
		}
	})

	t.Run("invalid inputs", func(t *testing.T) {
		// ecdsa key
		sk := invalidSK(t)
		s, err := BLSGeneratePOP(sk)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, s)

		s = make([]byte, SignatureLenBLSBLS12381)
		result, err := BLSVerifyPOP(sk.PublicKey(), s)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, result)
	})
}

// BLS multi-signature
// signature aggregation sanity check
//
// Aggregate n signatures of the same message under different keys, and compare
// it against the signature of the message under an aggregated private key.
// Verify the aggregated signature using the multi-signature verification with
// one message.
func TestAggregateSignatures(t *testing.T) {
	// random message
	input := make([]byte, 100)
	_, err := rand.Read(input)
	require.NoError(t, err)
	// hasher
	kmac := NewBLSKMAC("test tag")
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
		sk := randomSK(t, seed)
		s, err := sk.Sign(input, kmac)
		require.NoError(t, err)
		sigs = append(sigs, s)
		sks = append(sks, sk)
		pks = append(pks, sk.PublicKey())
	}

	// all signatures are valid
	t.Run("all valid signatures", func(t *testing.T) {
		// aggregate private keys
		aggSk, err := AggregateBLSPrivateKeys(sks)
		require.NoError(t, err)
		expectedSig, err := aggSk.Sign(input, kmac)
		require.NoError(t, err)
		// aggregate signatures
		aggSig, err := AggregateBLSSignatures(sigs)
		require.NoError(t, err)
		// First check: check the signatures are equal
		assert.Equal(t, aggSig, expectedSig,
			"incorrect signature %s, should be %s, private keys are %s, input is %x",
			aggSig, expectedSig, sks, input)
		// Second check: Verify the aggregated signature
		valid, err := VerifyBLSSignatureOneMessage(pks, aggSig, input, kmac)
		require.NoError(t, err)
		assert.True(t, valid,
			"Verification of %s failed, signature should be %s private keys are %s, input is %x",
			aggSig, expectedSig, sks, input)
	})

	// check if one signature is not correct
	t.Run("one invalid signature", func(t *testing.T) {
		input[0] ^= 1
		randomIndex := mrand.Intn(sigsNum)
		sigs[randomIndex], err = sks[randomIndex].Sign(input, kmac)
		input[0] ^= 1
		aggSig, err = AggregateBLSSignatures(sigs)
		require.NoError(t, err)
		assert.NotEqual(t, aggSig, expectedSig,
			"signature %s shouldn't be %s private keys are %s, input is %x",
			aggSig, expectedSig, sks, input)
		valid, err := VerifyBLSSignatureOneMessage(pks, aggSig, input, kmac)
		require.NoError(t, err)
		assert.False(t, valid,
			"verification of signature %s should fail, it shouldn't be %s private keys are %s, input is %x",
			aggSig, expectedSig, sks, input)
		sigs[randomIndex], err = sks[randomIndex].Sign(input, kmac)
	})

	// check if one the public keys is not correct
	t.Run("one invalid public key", func(t *testing.T) {
		randomIndex := mrand.Intn(sigsNum)
		newSk := randomSK(t, seed)
		sks[randomIndex] = newSk
		pks[randomIndex] = newSk.PublicKey()
		aggSk, err := AggregateBLSPrivateKeys(sks)
		require.NoError(t, err)
		expectedSig, err = aggSk.Sign(input, kmac)
		require.NoError(t, err)
		assert.NotEqual(t, aggSig, expectedSig,
			"signature %s shouldn't be %s, private keys are %s, input is %x, wrong key is of index %d",
			aggSig, expectedSig, sks, input, randomIndex)
		valid, err := VerifyBLSSignatureOneMessage(pks, aggSig, input, kmac)
		require.NoError(t, err)
		assert.False(t, valid,
			"signature %s should fail, shouldn't be %s, private keys are %s, input is %x, wrong key is of index %d",
			aggSig, expectedSig, sks, input, randomIndex)
	})

	t.Run("invalid inputs", func(t *testing.T) {
		// test aggregating an empty signature list
		aggSig, err = AggregateBLSSignatures(sigs[:0])
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, aggSig)

		// test verification with an empty key list
		result, err := VerifyBLSSignatureOneMessage(pks[:0], aggSig, input, kmac)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, result)

		// test with a signature of a wrong length
		shortSig := sigs[0][:signatureLengthBLSBLS12381-1]
		aggSig, err = AggregateBLSSignatures([]Signature{shortSig})
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, aggSig)

		// test with an invalid signature of a correct length
		invalidSig := BLSInvalidSignature()
		aggSig, err = AggregateBLSSignatures([]Signature{invalidSig})
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, aggSig)

		// test the empty key list
		aggSk, err := AggregateBLSPrivateKeys(sks[:0])
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, aggSk)

		// test with an invalid key type
		sk := invalidSK(t)
		aggSk, err = AggregateBLSPrivateKeys([]PrivateKey{sk})
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, aggSk)
	})
}

// BLS multi-signature
// public keys aggregation sanity check
//
// Aggregate n public keys and their respective private keys and compare
// the public key of the aggregated private key is equal to the aggregated
// public key
func TestAggregatePubKeys(t *testing.T) {
	r := time.Now().UnixNano()
	mrand.Seed(r)
	t.Logf("math rand seed is %d", r)
	// number of keys to aggregate
	pkNum := mrand.Intn(100) + 1
	pks := make([]PublicKey, 0, pkNum)
	sks := make([]PrivateKey, 0, pkNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)

	// create the signatures
	for i := 0; i < pkNum; i++ {
		sk := randomSK(t, seed)
		sks = append(sks, sk)
		pks = append(pks, sk.PublicKey())
	}

	// consistent private and public key aggregation
	t.Run("correctness check", func(t *testing.T) {
		// aggregate private keys
		aggSk, err := AggregateBLSPrivateKeys(sks)
		require.NoError(t, err)
		expectedPk := aggSk.PublicKey()
		// aggregate public keys
		aggPk, err := AggregateBLSPublicKeys(pks)
		assert.NoError(t, err)
		assert.True(t, expectedPk.Equals(aggPk),
			"incorrect public key %s, should be %s, public keys are %s",
			aggPk, expectedPk, pks)
	})

	// aggregate with the neutral key
	t.Run("empty list", func(t *testing.T) {
		// private keys
		aggSk, err := AggregateBLSPrivateKeys(sks[:0])
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, aggSk)
		// public keys
		aggPk, err := AggregateBLSPublicKeys(pks[:0])
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, aggPk)
	})

	// aggregate an empty list
	t.Run("neutral list", func(t *testing.T) {
		// aggregate the neutral key with a non neutral key
		keys := []PublicKey{pks[0], NeutralBLSPublicKey()}
		aggPkWithNeutral, err := AggregateBLSPublicKeys(keys)
		assert.NoError(t, err)
		assert.True(t, aggPkWithNeutral.Equals(pks[0]),
			"incorrect public key %s, should be %s",
			aggPkWithNeutral, pks[0])
	})

	t.Run("invalid inputs", func(t *testing.T) {
		// empty list
		aggPK, err := AggregateBLSPublicKeys(pks[:0])
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, aggPK)

		// test with an invalid key type
		pk := invalidSK(t).PublicKey()
		aggPK, err = AggregateBLSPublicKeys([]PublicKey{pk})
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, aggPK)
	})
}

// BLS multi-signature
// public keys removal sanity check
func TestRemovePubKeys(t *testing.T) {
	r := time.Now().UnixNano()
	mrand.Seed(r)
	t.Logf("math rand seed is %d", r)
	// number of keys to aggregate
	pkNum := mrand.Intn(100) + 1
	pks := make([]PublicKey, 0, pkNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)

	// generate public keys
	for i := 0; i < pkNum; i++ {
		sk := randomSK(t, seed)
		pks = append(pks, sk.PublicKey())
	}
	// aggregate public keys
	aggPk, err := AggregateBLSPublicKeys(pks)
	require.NoError(t, err)

	// random number of keys to remove (at least one key is left)
	pkToRemoveNum := mrand.Intn(pkNum)
	expectedPatrialPk, err := AggregateBLSPublicKeys(pks[pkToRemoveNum:])
	require.NoError(t, err)

	// check correctness
	t.Run("equality check", func(t *testing.T) {
		partialPk, err := RemoveBLSPublicKeys(aggPk, pks[:pkToRemoveNum])
		require.NoError(t, err)

		BLSkey, ok := expectedPatrialPk.(*PubKeyBLSBLS12381)
		require.True(t, ok)

		assert.True(t, BLSkey.Equals(partialPk),
			"incorrect key %s, should be %s, keys are %s, index is %d",
			partialPk, BLSkey, pks, pkToRemoveNum)
	})

	// remove an extra key and check inequality
	t.Run("inequality check", func(t *testing.T) {
		extraPk := randomSK(t, seed).PublicKey()
		partialPk, err := RemoveBLSPublicKeys(aggPk, []PublicKey{extraPk})
		assert.NoError(t, err)

		BLSkey, ok := expectedPatrialPk.(*PubKeyBLSBLS12381)
		require.True(t, ok)
		assert.False(t, BLSkey.Equals(partialPk),
			"incorrect key %s, should not be %s, keys are %s, index is %d, extra key is %s",
			partialPk, BLSkey, pks, pkToRemoveNum, extraPk)
	})

	// specific test to remove all keys
	t.Run("remove all keys", func(t *testing.T) {
		neutralPk, err := RemoveBLSPublicKeys(aggPk, pks)
		require.NoError(t, err)
		// neutral public key is expected
		randomPk := randomSK(t, seed).PublicKey()
		randomPkPlusNeutralPk, err := AggregateBLSPublicKeys([]PublicKey{randomPk, neutralPk})
		require.NoError(t, err)

		BLSRandomPk, ok := randomPk.(*PubKeyBLSBLS12381)
		require.True(t, ok)

		assert.True(t, BLSRandomPk.Equals(randomPkPlusNeutralPk),
			"incorrect key %s, should be infinity point, keys are %s",
			neutralPk, pks)
	})

	// specific test with an empty slice of keys to remove
	t.Run("remove empty list", func(t *testing.T) {
		partialPk, err := RemoveBLSPublicKeys(aggPk, []PublicKey{})
		require.NoError(t, err)

		aggBLSkey, ok := aggPk.(*PubKeyBLSBLS12381)
		require.True(t, ok)

		assert.True(t, aggBLSkey.Equals(partialPk),
			"incorrect key %s, should be %s",
			partialPk, aggBLSkey)
	})

	t.Run("invalid inputs", func(t *testing.T) {
		pk := invalidSK(t).PublicKey()
		partialPk, err := RemoveBLSPublicKeys(pk, pks)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, partialPk)

		partialPk, err = RemoveBLSPublicKeys(aggPk, []PublicKey{pk})
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, partialPk)
	})
}

// BLS multi-signature
// batch verification
//
// Verify n signatures of the same message under different keys using the fast
// batch verification technique and compares the result to verifying each signature
// separately.
func TestBatchVerify(t *testing.T) {
	r := time.Now().UnixNano()
	mrand.Seed(r)
	t.Logf("math rand seed is %d", r)
	// random message
	input := make([]byte, 100)
	_, err := mrand.Read(input)
	require.NoError(t, err)
	// hasher
	kmac := NewBLSKMAC("test tag")
	// number of signatures to aggregate
	sigsNum := mrand.Intn(100) + 2
	sigs := make([]Signature, 0, sigsNum)
	sks := make([]PrivateKey, 0, sigsNum)
	pks := make([]PublicKey, 0, sigsNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	expectedValid := make([]bool, 0, sigsNum)

	// create the signatures
	for i := 0; i < sigsNum; i++ {
		sk := randomSK(t, seed)
		s, err := sk.Sign(input, kmac)
		require.NoError(t, err)
		sigs = append(sigs, s)
		sks = append(sks, sk)
		pks = append(pks, sk.PublicKey())
		expectedValid = append(expectedValid, true)
	}

	// all signatures are valid
	t.Run("all signatures are valid", func(t *testing.T) {
		valid, err := BatchVerifyBLSSignaturesOneMessage(pks, sigs, input, kmac)
		require.NoError(t, err)
		assert.Equal(t, valid, expectedValid,
			"Verification of %s failed, private keys are %s, input is %x, results is %v",
			sigs, sks, input, valid)
	})

	// one valid signature
	t.Run("one valid signature", func(t *testing.T) {
		valid, err := BatchVerifyBLSSignaturesOneMessage(pks[:1], sigs[:1], input, kmac)
		require.NoError(t, err)
		assert.Equal(t, valid, expectedValid[:1],
			"Verification of %s failed, private keys are %s, input is %x, results is %v",
			sigs, sks, input, valid)
	})

	// pick a random number of invalid signatures
	invalidSigsNum := mrand.Intn(sigsNum-1) + 1
	// generate a random permutation of indices to pick the
	// invalid signatures.
	indices := make([]int, 0, sigsNum)
	for i := 0; i < sigsNum; i++ {
		indices = append(indices, i)
	}
	mrand.Shuffle(sigsNum, func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	// some signatures are invalid
	t.Run("some signatures are invalid", func(t *testing.T) {

		for i := 0; i < invalidSigsNum; i++ { // alter invalidSigsNum random signatures
			alterSignature(sigs[indices[i]])
			expectedValid[indices[i]] = false
		}

		valid, err := BatchVerifyBLSSignaturesOneMessage(pks, sigs, input, kmac)
		require.NoError(t, err)
		assert.Equal(t, expectedValid, valid,
			"Verification of %s failed\n private keys are %s\n input is %x\n results is %v",
			sigs, sks, input, valid)
	})

	// all signatures are invalid
	t.Run("all signatures are invalid", func(t *testing.T) {
		for i := invalidSigsNum; i < sigsNum; i++ { // alter the remaining random signatures
			alterSignature(sigs[indices[i]])
			expectedValid[indices[i]] = false
			if i%5 == 0 {
				sigs[indices[i]] = sigs[indices[i]][:3] // test the short signatures
			}
		}

		valid, err := BatchVerifyBLSSignaturesOneMessage(pks, sigs, input, kmac)
		require.NoError(t, err)
		assert.Equal(t, valid, expectedValid,
			"Verification of %s failed, private keys are %s, input is %x, results is %v",
			sigs, sks, input, valid)
	})

	// test the empty list case
	t.Run("empty list", func(t *testing.T) {
		valid, err := BatchVerifyBLSSignaturesOneMessage(pks[:0], sigs[:0], input, kmac)
		require.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Equal(t, valid, []bool{},
			"verification should fail with empty list key, got %v", valid)
	})

	// test incorrect inputs
	t.Run("inconsistent inputs", func(t *testing.T) {
		valid, err := BatchVerifyBLSSignaturesOneMessage(pks[:len(pks)-1], sigs, input, kmac)
		require.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Equal(t, valid, []bool{},
			"verification should fail with incorrect input lenghts, got %v", valid)
	})

	// test wrong hasher
	t.Run("invalid hasher", func(t *testing.T) {
		for i := 0; i < sigsNum; i++ {
			expectedValid[i] = false
		}
		valid, err := BatchVerifyBLSSignaturesOneMessage(pks, sigs, input, nil)
		require.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))

		assert.Equal(t, valid, expectedValid,
			"verification should fail with nil hasher, got %v", valid)
	})

	// test wrong key
	t.Run("wrong key", func(t *testing.T) {
		for i := 0; i < sigsNum; i++ {
			expectedValid[i] = false
		}
		pks[0] = invalidSK(t).PublicKey()
		valid, err := BatchVerifyBLSSignaturesOneMessage(pks, sigs, input, nil)
		require.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))

		assert.Equal(t, valid, expectedValid,
			"verification should fail with invalid key, got %v", valid)
	})
}

// alter or fix a signature
func alterSignature(s Signature) {
	// this causes the signature to remain in G1 and be invalid
	// OR to be a non-point in G1 (either on curve or not)
	// which tests multiple error cases.
	s[10] ^= 1
}

// Batch verify bench in the happy (all signatures are valid)
// and unhappy path (only one signature is invalid)
func BenchmarkBatchVerify(b *testing.B) {
	// random message
	input := make([]byte, 100)
	_, _ = mrand.Read(input)
	// hasher
	kmac := NewBLSKMAC("bench tag")
	sigsNum := 100
	sigs := make([]Signature, 0, sigsNum)
	pks := make([]PublicKey, 0, sigsNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)

	// create the signatures
	for i := 0; i < sigsNum; i++ {
		_, _ = mrand.Read(seed)
		sk, err := GeneratePrivateKey(BLSBLS12381, seed)
		require.NoError(b, err)
		s, err := sk.Sign(input, kmac)
		require.NoError(b, err)
		sigs = append(sigs, s)
		pks = append(pks, sk.PublicKey())
	}

	// Batch verify bench when all signatures are valid
	// (2) pairing compared to (2*n) pairings for the batch verification.
	b.Run("happy path", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// all signatures are valid
			_, err := BatchVerifyBLSSignaturesOneMessage(pks, sigs, input, kmac)
			require.NoError(b, err)
		}
		b.StopTimer()
	})

	// Batch verify bench when some signatures are invalid
	// - if only one signaure is invalid (a valid point in G1):
	// less than (2*2*log(n)) pairings compared to (2*n) pairings for the simple verification.
	// - if all signatures are invalid (valid points in G1):
	// (2*2*(n-1)) pairings compared to (2*n) pairings for the simple verification.
	b.Run("unhappy path", func(b *testing.B) {
		// only one invalid signature
		alterSignature(sigs[sigsNum/2])
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			// all signatures are valid
			_, err := BatchVerifyBLSSignaturesOneMessage(pks, sigs, input, kmac)
			require.NoError(b, err)
		}
		b.StopTimer()
	})
}

// BLS multi-signature
// signature aggregation sanity check
//
// Aggregate n signatures of distinct messages under different keys,
// and verify the aggregated signature using the multi-signature verification with
// many message.
func TestAggregateSignaturesManyMessages(t *testing.T) {
	r := time.Now().UnixNano()
	mrand.Seed(r)
	t.Logf("math rand seed is %d", r)

	// number of signatures to aggregate
	sigsNum := mrand.Intn(20) + 1
	sigs := make([]Signature, 0, sigsNum)

	// number of keys
	keysNum := mrand.Intn(sigsNum) + 1
	sks := make([]PrivateKey, 0, keysNum)
	pks := make([]PublicKey, 0, keysNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	// generate the keys
	for i := 0; i < keysNum; i++ {
		sk := randomSK(t, seed)
		sks = append(sks, sk)
		pks = append(pks, sk.PublicKey())
	}

	// number of messages (could be larger or smaller than the number of keys)
	msgsNum := mrand.Intn(sigsNum) + 1
	messages := make([][20]byte, msgsNum)
	for i := 0; i < msgsNum; i++ {
		_, err := rand.Read(messages[i][:])
		require.NoError(t, err)
	}

	inputMsgs := make([][]byte, 0, sigsNum)
	inputPks := make([]PublicKey, 0, sigsNum)
	inputKmacs := make([]hash.Hasher, 0, sigsNum)

	// create the signatures
	for i := 0; i < sigsNum; i++ {
		kmac := NewBLSKMAC("test tag")
		// pick a key randomly from the list
		skRand := mrand.Intn(keysNum)
		sk := sks[skRand]
		// pick a message randomly from the list
		msgRand := mrand.Intn(msgsNum)
		msg := messages[msgRand][:]
		// generate a signature
		s, err := sk.Sign(msg, kmac)
		require.NoError(t, err)
		// update signatures and api inputs
		sigs = append(sigs, s)
		inputPks = append(inputPks, sk.PublicKey())
		inputMsgs = append(inputMsgs, msg)
		inputKmacs = append(inputKmacs, kmac)
	}
	var aggSig Signature

	t.Run("correctness check", func(t *testing.T) {
		// aggregate signatures
		var err error
		aggSig, err = AggregateBLSSignatures(sigs)
		require.NoError(t, err)
		// Verify the aggregated signature
		valid, err := VerifyBLSSignatureManyMessages(inputPks, aggSig, inputMsgs, inputKmacs)
		require.NoError(t, err)
		assert.True(t, valid,
			"Verification of %s failed, should be valid, private keys are %s, inputs are %x, input public keys are %s",
			aggSig, sks, inputMsgs, inputPks)
	})

	// check if one of the signatures is not correct
	t.Run("one signature is invalid", func(t *testing.T) {
		randomIndex := mrand.Intn(sigsNum) // pick a random signature
		messages[0][0] ^= 1                // make sure the signature is different
		var err error
		sigs[randomIndex], err = sks[0].Sign(messages[0][:], inputKmacs[0])
		messages[0][0] ^= 1
		aggSig, err = AggregateBLSSignatures(sigs)
		require.NoError(t, err)
		valid, err := VerifyBLSSignatureManyMessages(inputPks, aggSig, inputMsgs, inputKmacs)
		require.NoError(t, err)
		assert.False(t, valid,
			"Verification of %s should fail, private keys are %s, inputs are %x, input public keys are %s",
			aggSig, sks, inputMsgs, inputPks)
	})

	// test the empty keys case
	t.Run("empty list", func(t *testing.T) {
		valid, err := VerifyBLSSignatureManyMessages(inputPks[:0], aggSig, inputMsgs, inputKmacs)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, valid,
			"verification should fail with an empty key list")
	})

	// test inconsistent input arrays
	t.Run("inconsistent inputs", func(t *testing.T) {
		// inconsistent lengths
		valid, err := VerifyBLSSignatureManyMessages(inputPks, aggSig, inputMsgs[:sigsNum-1], inputKmacs)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, valid, "verification should fail with inconsistent messages and hashers")

		// empty key list
		valid, err = VerifyBLSSignatureManyMessages(inputPks[:0], aggSig, inputMsgs, inputKmacs)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, valid, "verification should fail with empty list key")

		// nil hasher
		tmp := inputKmacs[0]
		inputKmacs[0] = nil
		valid, err = VerifyBLSSignatureManyMessages(inputPks, aggSig, inputMsgs, inputKmacs)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, valid, "verification should fail with nil hasher")
		inputKmacs[0] = tmp

		// wrong key
		tmpPK := inputPks[0]
		inputPks[0] = invalidSK(t).PublicKey()
		valid, err = VerifyBLSSignatureManyMessages(inputPks, aggSig, inputMsgs, inputKmacs)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, valid, "verification should fail with nil hasher")
		inputPks[0] = tmpPK
	})
}

// VerifyBLSSignatureManyMessages bench
// Bench the slowest case where all messages and public keys are distinct.
// (2*n) pairings without aggrgetion Vs (n+1) pairings with aggregation.
// The function is faster whenever there are redundant messages or public keys.
func BenchmarkVerifySignatureManyMessages(b *testing.B) {
	// inputs
	sigsNum := 100
	inputKmacs := make([]hash.Hasher, 0, sigsNum)
	sigs := make([]Signature, 0, sigsNum)
	pks := make([]PublicKey, 0, sigsNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	inputMsgs := make([][]byte, 0, sigsNum)
	kmac := NewBLSKMAC("bench tag")

	// create the signatures
	for i := 0; i < sigsNum; i++ {
		input := make([]byte, 100)
		_, _ = mrand.Read(seed)
		sk, err := GeneratePrivateKey(BLSBLS12381, seed)
		require.NoError(b, err)
		s, err := sk.Sign(input, kmac)
		require.NoError(b, err)
		sigs = append(sigs, s)
		pks = append(pks, sk.PublicKey())
		inputKmacs = append(inputKmacs, kmac)
		inputMsgs = append(inputMsgs, input)
	}
	aggSig, err := AggregateBLSSignatures(sigs)
	require.NoError(b, err)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := VerifyBLSSignatureManyMessages(pks, aggSig, inputMsgs, inputKmacs)
		require.NoError(b, err)
	}
	b.StopTimer()
}

// Bench of all aggregation functions
func BenchmarkAggregate(b *testing.B) {
	// random message
	input := make([]byte, 100)
	_, _ = mrand.Read(input)
	// hasher
	kmac := NewBLSKMAC("bench tag")
	sigsNum := 1000
	sigs := make([]Signature, 0, sigsNum)
	sks := make([]PrivateKey, 0, sigsNum)
	pks := make([]PublicKey, 0, sigsNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)

	// create the signatures
	for i := 0; i < sigsNum; i++ {
		_, _ = mrand.Read(seed)
		sk, err := GeneratePrivateKey(BLSBLS12381, seed)
		require.NoError(b, err)
		s, err := sk.Sign(input, kmac)
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
			_, err := AggregateBLSPrivateKeys(sks)
			require.NoError(b, err)
		}
		b.StopTimer()
	})

	// public keys
	b.Run("PublicKeys", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := AggregateBLSPublicKeys(pks)
			require.NoError(b, err)
		}
		b.StopTimer()
	})

	// signatures
	b.Run("Signatures", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := AggregateBLSSignatures(sigs)
			require.NoError(b, err)
		}
		b.StopTimer()
	})
}
