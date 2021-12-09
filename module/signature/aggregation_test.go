// +build relic

package signature

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
)

const NUM_AGG_TEST = 7
const NUM_AGG_BENCH = 100

func createAggregationT(t *testing.T) (*AggregationProvider, crypto.PrivateKey) {
	agg, priv, err := createAggregation()
	require.NoError(t, err)
	return agg, priv
}

func createAggregationB(b *testing.B) (*AggregationProvider, crypto.PrivateKey) {
	agg, priv, err := createAggregation()
	if err != nil {
		b.Fatal(err)
	}
	return agg, priv
}

// can't use the unittest.IdentityFixture due to circular import
func IdentityFixture() *flow.Identity {
	var id flow.Identifier
	_, _ = rand.Read(id[:])
	return &flow.Identity{
		NodeID:  id,
		Address: fmt.Sprintf("address-%v", id[0:7]),
		Role:    flow.RoleConsensus,
		Stake:   1000,
	}
}

func createAggregation() (*AggregationProvider, crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	if err != nil {
		return nil, nil, err
	}
	if n < len(seed) {
		return nil, nil, fmt.Errorf("insufficient random bytes")
	}
	priv, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	if err != nil {
		return nil, nil, err
	}

	node := IdentityFixture()
	node.StakingPubKey = priv.PublicKey()

	local, err := local.New(node, priv)
	if err != nil {
		return nil, nil, err
	}
	return NewAggregationProvider("test_staking", local), priv, nil
}

func TestAggregationSignVerify(t *testing.T) {

	signer, priv := createAggregationT(t)
	msg := randomByteSliceT(t, 128)

	// create the signature
	sig, err := signer.Sign(msg)
	require.NoError(t, err)

	// signature should be valid for the original signer
	valid, err := signer.Verify(msg, sig, priv.PublicKey())
	require.NoError(t, err)
	assert.True(t, valid)

	// signature should not be valid for another signer
	_, altPriv := createAggregationT(t)
	valid, err = signer.Verify(msg, sig, altPriv.PublicKey())
	require.NoError(t, err)
	assert.False(t, valid)

	// signature should not be valid if we change one byte
	sig[0]++
	valid, err = signer.Verify(msg, sig, priv.PublicKey())
	require.NoError(t, err)
	assert.False(t, valid)
	sig[0]--
}

func TestAggregationAggregateVerifyMany(t *testing.T) {

	// create a certain amount of signers & signatures
	var keys []crypto.PublicKey
	var sigs []crypto.Signature
	msg := randomByteSliceT(t, 128)
	for i := 0; i < NUM_AGG_TEST; i++ {
		signer, priv := createAggregationT(t)
		sig, err := signer.Sign(msg)
		require.NoError(t, err)
		keys = append(keys, priv.PublicKey())
		sigs = append(sigs, sig)
	}

	// aggregate the signatures
	agg, _ := createAggregationT(t)
	aggSig, err := agg.Aggregate(sigs)
	require.NoError(t, err)

	// signature should be valid for the given keys
	valid, err := agg.VerifyMany(msg, aggSig, keys)
	require.NoError(t, err)
	require.True(t, valid)

	// signature should fail with one key missing
	valid, err = agg.VerifyMany(msg, aggSig, keys[1:])
	require.NoError(t, err)
	require.False(t, valid)

	// signature should be valid with one key swapped
	keys[0], keys[1] = keys[1], keys[0]
	valid, err = agg.VerifyMany(msg, aggSig, keys)
	require.NoError(t, err)
	require.True(t, valid)

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
	msg := randomByteSliceB(b)
	sigs := make([]crypto.Signature, 0, NUM_AGG_BENCH)
	for i := 0; i < NUM_AGG_BENCH; i++ {
		signer, _ = createAggregationB(b)
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

func BenchmarkVerifyMany(b *testing.B) {

	// stop timer and reset to zero
	b.StopTimer()
	b.ResetTimer()

	// create the desired number of signers
	var signer *AggregationProvider
	var priv crypto.PrivateKey
	msg := randomByteSliceB(b)
	sigs := make([]crypto.Signature, 0, NUM_AGG_BENCH)
	keys := make([]crypto.PublicKey, 0, NUM_AGG_BENCH)
	for i := 0; i < NUM_AGG_BENCH; i++ {
		signer, priv = createAggregationB(b)
		sig, err := signer.Sign(msg)
		if err != nil {
			b.Fatal(err)
		}
		sigs = append(sigs, sig)
		keys = append(keys, priv.PublicKey())
	}
	sig, err := signer.Aggregate(sigs)
	if err != nil {
		b.Fatal(err)
	}

	// start the timer and benchmark the aggregation
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, err := signer.VerifyMany(msg, sig, keys)
		if err != nil {
			b.Fatal(err)
		}
	}
}
