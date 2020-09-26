// +build relic

package crypto

import (
	"crypto/rand"
	"fmt"
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

func randomSK(t *testing.T, seed []byte) PrivateKey {
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(BLSBLS12381, seed)
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
	_, err := sk.Sign(seed, nil)
	assert.Error(t, err)
	_, err = sk.PublicKey().Verify(sig, seed, nil)
	assert.Error(t, err)
	// short size hasher
	_, err = sk.Sign(seed, hash.NewSHA2_256())
	assert.Error(t, err)
	_, err = sk.PublicKey().Verify(sig, seed, hash.NewSHA2_256())
	assert.Error(t, err)
}

// TestBLSEncodeDecode tests encoding and decoding of BLS keys
func TestBLSEncodeDecode(t *testing.T) {
	testEncodeDecode(t, BLSBLS12381)
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
	kmac := NewBLSKMAC("POP test tag")
	testPOP(t, BLSBLS12381, kmac)
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
	mrand.Seed(time.Now().UnixNano())
	sigsNum := mrand.Intn(100) + 1
	sigs := make([]Signature, 0, sigsNum)
	sks := make([]PrivateKey, 0, sigsNum)
	pks := make([]PublicKey, 0, sigsNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)

	// create the signatures
	for i := 0; i < sigsNum; i++ {
		sk := randomSK(t, seed)
		s, err := sk.Sign(input, kmac)
		require.NoError(t, err)
		sigs = append(sigs, s)
		sks = append(sks, sk)
		pks = append(pks, sk.PublicKey())
	}
	// aggregate private keys
	aggSk, err := AggregatePrivateKeys(sks)
	require.NoError(t, err)
	expectedSig, err := aggSk.Sign(input, kmac)
	require.NoError(t, err)
	// aggregate signatures
	aggSig, err := AggregateSignatures(sigs)
	require.NoError(t, err)
	// First check: check the signatures are equal
	assert.Equal(t, aggSig, expectedSig,
		fmt.Sprintf("incorrect signature %s, should be %s, private keys are %s, input is %x",
			aggSig, expectedSig, sks, input))
	// Second check: Verify the aggregated signature
	valid, err := VerifySignatureOneMessage(pks, aggSig, input, kmac)
	require.NoError(t, err)
	assert.True(t, valid,
		fmt.Sprintf("Verification of %s failed, signature should be %s private keys are %s, input is %x",
			aggSig, expectedSig, sks, input))

	// check if one the signatures is not correct
	input[0] ^= 1
	randomIndex := mrand.Intn(sigsNum)
	sigs[randomIndex], err = sks[randomIndex].Sign(input, kmac)
	input[0] ^= 1
	aggSig, err = AggregateSignatures(sigs)
	require.NoError(t, err)
	assert.NotEqual(t, aggSig, expectedSig,
		fmt.Sprintf("signature %s shouldn't be %s private keys are %s, input is %x",
			aggSig, expectedSig, sks, input))
	valid, err = VerifySignatureOneMessage(pks, aggSig, input, kmac)
	require.NoError(t, err)
	assert.False(t, valid,
		fmt.Sprintf("verification of signature %s should fail, it shouldn't be %s private keys are %s, input is %x",
			aggSig, expectedSig, sks, input))
	sigs[randomIndex], err = sks[randomIndex].Sign(input, kmac)
	// check if one the public keys is not correct
	randomIndex = mrand.Intn(sigsNum)
	newSk := randomSK(t, seed)
	sks[randomIndex] = newSk
	pks[randomIndex] = newSk.PublicKey()
	aggSk, err = AggregatePrivateKeys(sks)
	require.NoError(t, err)
	expectedSig, err = aggSk.Sign(input, kmac)
	require.NoError(t, err)
	assert.NotEqual(t, aggSig, expectedSig,
		fmt.Sprintf("signature %s shouldn't be %s, private keys are %s, input is %x, wrong key is of index %d",
			aggSig, expectedSig, sks, input, randomIndex))
	valid, err = VerifySignatureOneMessage(pks, aggSig, input, kmac)
	require.NoError(t, err)
	assert.False(t, valid,
		fmt.Sprintf("signature %s should fail, shouldn't be %s, private keys are %s, input is %x, wrong key is of index %d",
			aggSig, expectedSig, sks, input, randomIndex))

	// test the empty list case
	aggSk, err = AggregatePrivateKeys(sks[:0])
	assert.NoError(t, err)
	expectedSig, err = aggSk.Sign(input, kmac)
	aggSig, err = AggregateSignatures(sigs[:0])
	assert.NoError(t, err)
	assert.Equal(t, aggSig, expectedSig,
		fmt.Sprintf("wrong empty list key %s", sks))
	valid, err = VerifySignatureOneMessage(pks[:0], aggSig, input, kmac)
	assert.Error(t, err)
	assert.False(t, valid,
		fmt.Sprintf("verification should fail with empty list key %s", sks))
}

// BLS multi-signature
// public keys aggregation sanity check
//
// Aggregate n public keys and their respective private keys and compare
// the public key of the aggregated private key is equal to the aggregated
// public key
func TestAggregatePubKeys(t *testing.T) {
	// number of keys to aggregate
	mrand.Seed(time.Now().UnixNano())
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
	// aggregate private keys
	aggSk, err := AggregatePrivateKeys(sks)
	require.NoError(t, err)
	expectedPk := aggSk.PublicKey()
	// aggregate public keys
	aggPk, err := AggregatePublicKeys(pks)
	assert.NoError(t, err)
	assert.True(t, expectedPk.Equals(aggPk),
		fmt.Sprintf("incorrect public key %s, should be %s, public keys are %s",
			aggPk, expectedPk, pks))

	// aggregate an empty list
	aggSk, err = AggregatePrivateKeys(sks[:0])
	assert.NoError(t, err)
	expectedPk = aggSk.PublicKey()
	aggPk, err = AggregatePublicKeys(pks[:0])
	assert.NoError(t, err)
	assert.True(t, expectedPk.Equals(aggPk),
		fmt.Sprintf("incorrect generator %s, should be %s",
			aggPk, expectedPk))
}

// BLS multi-signature
// public keys aggregation sanity check
//
// Aggregate n public keys and their respective private keys and compare
// the public key of the aggregated private key is equal to the aggregated
// public key
func TestRemovePubKeys(t *testing.T) {
	mrand.Seed(time.Now().UnixNano())
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
	aggPk, err := AggregatePublicKeys(pks)
	require.NoError(t, err)

	// random number of keys to remove
	pkToRemoveNum := mrand.Intn(pkNum)

	partialPk, err := RemovePublicKeys(aggPk, pks[:pkToRemoveNum])
	require.NoError(t, err)
	expectedPatrialPk, err := AggregatePublicKeys(pks[pkToRemoveNum:])
	require.NoError(t, err)

	BLSkey, ok := expectedPatrialPk.(*PubKeyBLSBLS12381)
	require.True(t, ok)

	assert.True(t, BLSkey.Equals(partialPk),
		fmt.Sprintf("incorrect key %s, should be %s, keys are %s, index is %d",
			partialPk, BLSkey, pks, pkToRemoveNum))

	// remove an extra key and check inequality
	extraPk := randomSK(t, seed).PublicKey()
	partialPk, err = RemovePublicKeys(aggPk, []PublicKey{extraPk})
	assert.NoError(t, err)
	assert.False(t, BLSkey.Equals(partialPk),
		fmt.Sprintf("incorrect key %s, should not be %s, keys are %s, index is %d, extra key is %s",
			partialPk, BLSkey, pks, pkToRemoveNum, extraPk))

	// specific test to remove all keys
	partialPk, err = RemovePublicKeys(aggPk, pks)
	require.NoError(t, err)
	expectedPatrialPk, err = AggregatePublicKeys([]PublicKey{})
	require.NoError(t, err)

	BLSkey, ok = expectedPatrialPk.(*PubKeyBLSBLS12381)
	require.True(t, ok)

	assert.True(t, BLSkey.Equals(partialPk),
		fmt.Sprintf("incorrect key %s, should be infinity point, keys are %s",
			partialPk, pks))

	// specific test with an empty slice of keys to remove
	partialPk, err = RemovePublicKeys(aggPk, pks)
	require.NoError(t, err)
	expectedPatrialPk, err = AggregatePublicKeys([]PublicKey{})
	require.NoError(t, err)

	BLSkey, ok = expectedPatrialPk.(*PubKeyBLSBLS12381)
	require.True(t, ok)

	assert.True(t, BLSkey.Equals(partialPk),
		fmt.Sprintf("incorrect key %s, should be %s, keys are %s",
			partialPk, BLSkey, pks))
}

// BLS multi-signature
// batch verification
//
// Verify n signatures of the same message under different keys using the fast
// batch verification technique and compares the result to verifying each signature
// separately.
func TestBatchVerify(t *testing.T) {
	mrand.Seed(time.Now().UnixNano())
	// random message
	input := make([]byte, 100)
	_, err := mrand.Read(input)
	require.NoError(t, err)
	// hasher
	kmac := NewBLSKMAC("test tag")
	// number of signatures to aggregate
	sigsNum := mrand.Intn(100) + 1
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

	// Batch verify the signatures
	// all signatures are valid
	valid, err := BatchVerifySignaturesOneMessage(pks, sigs, input, kmac)
	require.NoError(t, err)
	assert.Equal(t, valid, expectedValid,
		fmt.Sprintf("Verification of %s failed, private keys are %s, input is %x, results is %v",
			sigs, sks, input, valid))

	// some signatures are invalid
	invalidSigsNum := mrand.Intn(sigsNum-1) + 1 // pick a random number of invalid signatures
	indices := make([]int, 0, sigsNum)          // pick invalidSigsNum random indices
	for i := 0; i < sigsNum; i++ {
		indices = append(indices, i)
	}
	mrand.Shuffle(sigsNum, func(i, j int) {
		indices[i], indices[j] = indices[j], indices[i]
	})

	for i := 0; i < invalidSigsNum; i++ { // alter invalidSigsNum random signatures
		alterSignature(sigs[indices[i]])
		expectedValid[indices[i]] = false
	}

	valid, err = BatchVerifySignaturesOneMessage(pks, sigs, input, kmac)
	require.NoError(t, err)
	assert.Equal(t, expectedValid, valid,
		fmt.Sprintf("Verification of %s failed\n private keys are %s\n input is %x\n results is %v",
			sigs, sks, input, valid))

	// all signatures are invalid
	for i := invalidSigsNum; i < sigsNum; i++ { // alter the remaining random signatures
		alterSignature(sigs[indices[i]])
		expectedValid[indices[i]] = false
		if i%5 == 0 {
			sigs[indices[i]] = sigs[indices[i]][:3] // test the short signatures
		}
	}

	valid, err = BatchVerifySignaturesOneMessage(pks, sigs, input, kmac)
	require.NoError(t, err)
	assert.Equal(t, valid, expectedValid,
		fmt.Sprintf("Verification of %s failed, private keys are %s, input is %x, results is %v",
			sigs, sks, input, valid))

	// test the empty list case
	valid, err = BatchVerifySignaturesOneMessage(pks[:0], sigs[:0], input, kmac)
	require.Error(t, err)
	assert.Equal(t, valid, []bool{},
		fmt.Sprintf("verification should fail with empty list key, got %v", valid))
	// test incorrect inputs
	valid, err = BatchVerifySignaturesOneMessage(pks[:len(pks)-1], sigs, input, kmac)
	require.Error(t, err)
	assert.Equal(t, valid, []bool{},
		fmt.Sprintf("verification should fail with incorrect input lenghts, got %v", valid))
	// test wrong hasher
	for i := 0; i < sigsNum; i++ {
		expectedValid[i] = false
	}
	valid, err = BatchVerifySignaturesOneMessage(pks, sigs, input, nil)
	require.Error(t, err)
	//require.Nil(t, aggSig)
	assert.Equal(t, valid, expectedValid,
		fmt.Sprintf("verification should fail with incorrect input lenghts, got %v", valid))
}

// alter or fix a signature
func alterSignature(s Signature) {
	// this causes the signature to remain in G1 and be invalid
	// OR to be a non-point in G1 (either on curve or not)
	// which tests multiple error cases.
	s[10] ^= 1
}

// Batch verify bench when all signatures are valid
// (2) pairing compared to (2*n) pairings for the batch verification.
func BenchmarkBatchVerifyHappyPath(b *testing.B) {
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
		sk, _ := GeneratePrivateKey(BLSBLS12381, seed)
		s, _ := sk.Sign(input, kmac)
		sigs = append(sigs, s)
		pks = append(pks, sk.PublicKey())
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// all signatures are valid
		_, _ = BatchVerifySignaturesOneMessage(pks, sigs, input, kmac)
	}
	b.StopTimer()
}

// Batch verify bench when some signatures are invalid
// - if only one signaure is invalid (a valid point in G1):
// less than (2*2*log(n)) pairings compared to (2*n) pairings for the simple verification.
// - if all signatures are invalid (valid points in G1):
// (2*2*(n-1)) pairings compared to (2*n) pairings for the simple verification.
func BenchmarkBatchVerifyUnHappyPath(b *testing.B) {
	input := make([]byte, 100)
	_, _ = mrand.Read(input)
	kmac := NewBLSKMAC("bench tag")
	sigsNum := 100
	sigs := make([]Signature, 0, sigsNum)
	pks := make([]PublicKey, 0, sigsNum)
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)

	// create the signatures
	for i := 0; i < sigsNum; i++ {
		_, _ = mrand.Read(seed)
		sk, _ := GeneratePrivateKey(BLSBLS12381, seed)
		s, _ := sk.Sign(input, kmac)
		sigs = append(sigs, s)
		pks = append(pks, sk.PublicKey())
	}

	// only one invalid signature
	alterSignature(sigs[sigsNum/2])
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// all signatures are valid
		_, _ = BatchVerifySignaturesOneMessage(pks, sigs, input, kmac)
	}
	b.StopTimer()
}
