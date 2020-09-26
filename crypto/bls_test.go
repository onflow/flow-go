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
func TestAggregateSignaturesOneMessage(t *testing.T) {
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
// signature aggregation sanity check
//
// Aggregate n signatures of distinct messages under different keys,
// and verify the aggregated signature using the multi-signature verification with
// many message.
func TestAggregateSignaturesManyMessages(t *testing.T) {
	//int64(1601003187394381000) //
	mrand.Seed(time.Now().UnixNano())

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

	// number of messages (could be more or less than keys)
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
	// aggregate signatures
	aggSig, err := AggregateSignatures(sigs)
	require.NoError(t, err)
	// Verify the aggregated signature
	valid, err := VerifySignatureManyMessages(inputPks, aggSig, inputMsgs, inputKmacs)
	require.NoError(t, err)
	assert.True(t, valid,
		fmt.Sprintf("Verification of %s failed, should be valid, private keys are %s, inputs are %x, input public keys are %s",
			aggSig, sks, inputMsgs, inputPks))

	// check if one the signatures is not correct
	randomIndex := mrand.Intn(sigsNum) // pick a random signature
	messages[0][0] ^= 1                // make sure the signature is different
	sigs[randomIndex], err = sks[0].Sign(messages[0][:], inputKmacs[0])
	messages[0][0] ^= 1
	aggSig, err = AggregateSignatures(sigs)
	require.NoError(t, err)
	valid, err = VerifySignatureManyMessages(inputPks, aggSig, inputMsgs, inputKmacs)
	require.NoError(t, err)
	assert.False(t, valid,
		fmt.Sprintf("Verification of %s should fail, private keys are %s, inputs are %x, input public keys are %s",
			aggSig, sks, inputMsgs, inputPks))

	// test the empty keys case
	valid, err = VerifySignatureManyMessages(inputPks[:0], aggSig, inputMsgs, inputKmacs)
	assert.Error(t, err)
	assert.False(t, valid,
		fmt.Sprintf("verification should fail with empty list key"))

	// test inconsistent input arrays
	valid, err = VerifySignatureManyMessages(inputPks, aggSig, inputMsgs[:sigsNum-1], inputKmacs)
	assert.Error(t, err)
	assert.False(t, valid,
		fmt.Sprintf("verification should fail with empty list key"))

	valid, err = VerifySignatureManyMessages(inputPks, aggSig, inputMsgs, inputKmacs[:sigsNum-1])
	assert.Error(t, err)
	assert.False(t, valid,
		fmt.Sprintf("verification should fail with empty list key"))
}

// VerifySignatureManyMessages bench
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
		sk, _ := GeneratePrivateKey(BLSBLS12381, seed)
		s, _ := sk.Sign(input, kmac)
		sigs = append(sigs, s)
		pks = append(pks, sk.PublicKey())
		inputKmacs = append(inputKmacs, kmac)
		inputMsgs = append(inputMsgs, input)
	}
	aggSig, _ := AggregateSignatures(sigs)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = VerifySignatureManyMessages(pks, aggSig, inputMsgs, inputKmacs)
	}
	b.StopTimer()
}
