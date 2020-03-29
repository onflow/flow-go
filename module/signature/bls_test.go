package signature

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const NUM_BLS_TEST = 7
const NUM_BLS_BENCH = 1000

func createBLST(t *testing.T) *BLS {
	bls, err := createBLS()
	require.NoError(t, err)
	return bls
}

func createBLSB(b *testing.B) *BLS {
	bls, err := createBLS()
	if err != nil {
		b.Fatal(err)
	}
	return bls
}

func createBLS() (*BLS, error) {
	seed := make([]byte, 48)
	n, err := rand.Read(seed)
	if err != nil {
		return nil, err
	}
	if n < len(seed) {
		return nil, fmt.Errorf("insufficient random bytes")
	}
	hasher := crypto.NewBLS_KMAC("only_testing")
	priv, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, seed)
	if err != nil {
		return nil, err
	}
	return NewBLS(hasher, priv), nil
}

func TestBLSSignVerify(t *testing.T) {

	signer := createBLST(t)
	msg := createMSGT(t)

	// create the signature
	sig, err := signer.Sign(msg)
	require.NoError(t, err)

	// signature should be valid for the original signer
	valid, err := signer.Verify(msg, sig, signer.priv.PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)

	// signature should not be valid for another signer
	valid, err = signer.Verify(msg, sig, createBLST(t).priv.PublicKey())
	require.NoError(t, err)
	assert.False(t, valid)

	// signature should not be valid if we change one byte
	sig[0]++
	valid, err = signer.Verify(msg, sig, signer.priv.PublicKey())
	require.NoError(t, err)
	assert.False(t, valid)
	sig[0]--
}

func TestBLSAggregateVerifyMany(t *testing.T) {

	// create a certain amount of signers & signatures
	var signers []*BLS
	var sigs []crypto.Signature
	msg := createMSGT(t)
	for i := 0; i < NUM_BLS_TEST; i++ {
		signer := createBLST(t)
		sig, err := signer.Sign(msg)
		require.NoError(t, err)
		signers = append(signers, signer)
		sigs = append(sigs, sig)
	}

	// aggregate the signatures
	bls := createBLST(t)
	aggSig, err := bls.Aggregate(sigs)
	require.NoError(t, err)

	// collect all the public keys
	var keys []crypto.PublicKey
	for _, signer := range signers {
		keys = append(keys, signer.priv.PublicKey())
	}

	// signature should be valid for the given keys
	valid, err := bls.VerifyMany(msg, aggSig, keys)
	require.NoError(t, err)
	require.True(t, valid)

	// signature should fail with one key missing
	valid, err = bls.VerifyMany(msg, aggSig, keys[1:])
	require.Error(t, err)

	// signature should be invalid with one key swapped
	temp := keys[0]
	keys[0] = createBLST(t).priv.PublicKey()
	valid, err = bls.VerifyMany(msg, aggSig, keys)
	require.NoError(t, err)
	require.False(t, valid)
	keys[0] = temp

	// signature should be invalid with one byte changed
	msg[0]++
	valid, err = bls.VerifyMany(msg, aggSig, keys)
	require.NoError(t, err)
	require.False(t, valid)
	msg[0]--
}

func BenchmarkBLSAggregation(b *testing.B) {

	// stop timer and reset to zero
	b.StopTimer()
	b.ResetTimer()

	// create the desired number of signers
	signers := make([]*BLS, 0, NUM_BLS_BENCH)
	for i := 0; i < NUM_BLS_BENCH; i++ {
		signer := createBLSB(b)
		signers = append(signers, signer)
	}

	// generate the desired number of signatures
	msg := createMSGB(b)
	sigs := make([]crypto.Signature, 0, len(msg))
	for _, signer := range signers {
		sig, err := signer.Sign(msg)
		if err != nil {
			b.Fatal(err)
		}
		sigs = append(sigs, sig)
	}

	// start the timer and benchmark the aggregation
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, err := signers[0].Aggregate(sigs)
		if err != nil {
			b.Fatal(err)
		}
	}
}
