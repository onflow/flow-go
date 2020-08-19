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

	"github.com/dapperlabs/flow-go/crypto/hash"
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

// BLS tests
func TestBLSBLS12381Hasher(t *testing.T) {
	// generate a key pair
	seed := make([]byte, KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.NoError(t, err)
	// empty hasher
	_, err = sk.Sign(seed, nil)
	assert.Error(t, err)
	_, err = sk.PublicKey().Verify(Signature{}, seed, nil)
	assert.Error(t, err)
	// short size hasher
	_, err = sk.Sign(seed, hash.NewSHA2_256())
	assert.Error(t, err)
	_, err = sk.PublicKey().Verify(Signature{}, seed, hash.NewSHA2_256())
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
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	sk, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.NoError(t, err)
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
		n, err := rand.Read(seed)
		require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(BLSBLS12381, seed)
		require.NoError(t, err)
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
		fmt.Sprintf("incorrect signature %s, should be %s, private keys are %s, input is %s",
			aggSig, expectedSig, sks, input))
	// Second check: Verify the aggregated signature
	valid, err := VerifySignatureOneMessage(pks, aggSig, input, kmac)
	require.NoError(t, err)
	assert.True(t, valid,
		fmt.Sprintf("Verification of %s failed, signature should be %s private keys are %s, input is %s",
			aggSig, expectedSig, sks, input))

	// check if one the signatures is not correct
	input[0] ^= 1
	randomIndex := mrand.Intn(sigsNum)
	sigs[randomIndex], err = sks[randomIndex].Sign(input, kmac)
	aggSig, err = AggregateSignatures(sigs)
	require.NoError(t, err)
	assert.NotEqual(t, aggSig, expectedSig,
		fmt.Sprintf("signature %s shouldn't be %s private keys are %s, input is %s",
			aggSig, expectedSig, sks, input))
	valid, err = VerifySignatureOneMessage(pks, aggSig, input, kmac)
	require.NoError(t, err)
	assert.False(t, valid,
		fmt.Sprintf("verification of signature %s should fail, it shouldn't be %s private keys are %s, input is %s",
			aggSig, expectedSig, sks, input))
	input[0] ^= 1
	sigs[randomIndex], err = sks[randomIndex].Sign(input, kmac)
	// check if one the public keys is not correct
	randomIndex = mrand.Intn(sigsNum)
	n, err := rand.Read(seed)
	require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
	require.NoError(t, err)
	newSk, err := GeneratePrivateKey(BLSBLS12381, seed)
	require.NoError(t, err)
	sks[randomIndex] = newSk
	pks[randomIndex] = newSk.PublicKey()
	aggSk, err = AggregatePrivateKeys(sks)
	require.NoError(t, err)
	expectedSig, err = aggSk.Sign(input, kmac)
	require.NoError(t, err)
	assert.NotEqual(t, aggSig, expectedSig,
		fmt.Sprintf("signature %s shouldn't be %s, private keys are %s, input is %s, wrong key is of index %d",
			aggSig, expectedSig, sks, input, randomIndex))
	valid, err = VerifySignatureOneMessage(pks, aggSig, input, kmac)
	require.NoError(t, err)
	assert.False(t, valid,
		fmt.Sprintf("signature %s should fail, shouldn't be %s, private keys are %s, input is %s, wrong key is of index %d",
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
		fmt.Sprintf("verification should pass with empty list key %s", sks))
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
		n, err := rand.Read(seed)
		require.Equal(t, n, KeyGenSeedMinLenBLSBLS12381)
		require.NoError(t, err)
		sk, err := GeneratePrivateKey(BLSBLS12381, seed)
		require.NoError(t, err)
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
