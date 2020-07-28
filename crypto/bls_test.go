// +build relic

package crypto

import (
	"crypto/rand"
	mrand "math/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	}
	// aggregate private keys
	aggSk, err := AggregatePrivateKeys(sks)
	require.NoError(t, err)
	expectedSig, err := aggSk.Sign(input, kmac)
	// aggregate signatures
	aggSig, err := AggregateSignatures(sigs)
	assert.NoError(t, err)
	assert.Equal(t, aggSig, expectedSig)
	// aggregate signatures with an empty list
	aggSig, err = AggregateSignatures(sigs[:0])
	assert.Error(t, err)
}

// BLS multi-signature
// public keys aggregation sanity check
//
// Aggregate n public keys and their respective private keys and compare
// the public key of the aggregated private keys is equal to the aggregated
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
	assert.Equal(t, aggPk, expectedPk)
}
