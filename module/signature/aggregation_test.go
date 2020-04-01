package signature

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
)

const NUM_AggregationProvider_TEST = 7
const NUM_AggregationProvider_BENCH = 1000

func createAggregationT(t *testing.T) *AggregationProvider {
	agg, err := createAggregation()
	require.NoError(t, err)
	return agg
}

func createAggregationB(b *testing.B) *AggregationProvider {
	agg, err := createAggregation()
	if err != nil {
		b.Fatal(err)
	}
	return agg
}

func createAggregation() (*AggregationProvider, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenBLS_BLS12381)
	n, err := rand.Read(seed)
	if err != nil {
		return nil, err
	}
	if n < len(seed) {
		return nil, fmt.Errorf("insufficient random bytes")
	}
	priv, err := crypto.GeneratePrivateKey(crypto.BLS_BLS12381, seed)
	if err != nil {
		return nil, err
	}
	return NewAggregationProvider("only_testing", priv), nil
}

func TestAggregationSignVerify(t *testing.T) {

	signer := createAggregationT(t)
	msg := createMSGT(t)

	// create the signature
	sig, err := signer.Sign(msg)
	require.NoError(t, err)

	// signature should be valid for the original signer
	valid, err := signer.Verify(msg, sig, signer.priv.PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)

	// signature should not be valid for another signer
	valid, err = signer.Verify(msg, sig, createAggregationT(t).priv.PublicKey())
	require.NoError(t, err)
	assert.False(t, valid)

	// signature should not be valid if we change one byte
	sig[0]++
	valid, err = signer.Verify(msg, sig, signer.priv.PublicKey())
	require.NoError(t, err)
	assert.False(t, valid)
	sig[0]--
}

func TestAggregationAggregateVerifyMany(t *testing.T) {

	// create a certain amount of signers & signatures
	var signers []*AggregationProvider
	var sigs []crypto.Signature
	msg := createMSGT(t)
	for i := 0; i < NUM_AggregationProvider_TEST; i++ {
		signer := createAggregationT(t)
		sig, err := signer.Sign(msg)
		require.NoError(t, err)
		signers = append(signers, signer)
		sigs = append(sigs, sig)
	}

	// aggregate the signatures
	agg := createAggregationT(t)
	aggSig, err := agg.Aggregate(sigs)
	require.NoError(t, err)

	// collect all the public keys
	var keys []crypto.PublicKey
	for _, signer := range signers {
		keys = append(keys, signer.priv.PublicKey())
	}

	// signature should be valid for the given keys
	valid, err := agg.VerifyMany(msg, aggSig, keys)
	require.NoError(t, err)
	require.True(t, valid)

	// signature should fail with one key missing
	_, err = agg.VerifyMany(msg, aggSig, keys[1:])
	require.Error(t, err)

	// signature should be invalid with one key swapped
	temp := keys[0]
	keys[0] = createAggregationT(t).priv.PublicKey()
	valid, err = agg.VerifyMany(msg, aggSig, keys)
	require.NoError(t, err)
	require.False(t, valid)
	keys[0] = temp

	// signature should be invalid with one byte changed
	msg[0]++
	valid, err = agg.VerifyMany(msg, aggSig, keys)
	require.NoError(t, err)
	require.False(t, valid)
	msg[0]--
}

func BenchmarkAggregationProviderAggregation(b *testing.B) {

	// stop timer and reset to zero
	b.StopTimer()
	b.ResetTimer()

	// create the desired number of signers
	var signer *AggregationProvider
	msg := createMSGB(b)
	sigs := make([]crypto.Signature, 0, NUM_AggregationProvider_BENCH)
	for i := 0; i < NUM_AggregationProvider_BENCH; i++ {
		signer = createAggregationB(b)
		sig, err := signer.Sign(msg)
		if err != nil {
			b.Fatal(err)
		}
		sigs = append(sigs, sig)
	}

	// start the timer and benchmark the aggregation
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, err := signer.Aggregate(sigs)
		if err != nil {
			b.Fatal(err)
		}
	}
}
