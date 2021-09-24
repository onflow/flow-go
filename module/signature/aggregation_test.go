// +build relic

package signature

import (
	"crypto/rand"
	mrand "math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
)

func createAggregationData(t *testing.T, signersNumber int) (*SignatureAggregatorSameMessage, []crypto.Signature) {
	// create message and tag
	msgLen := 100
	msg := make([]byte, msgLen)
	tag := "random_tag"
	hasher := crypto.NewBLSKMAC(tag)

	// create keys and signatures
	keys := make([]crypto.PublicKey, 0, signersNumber)
	sigs := make([]crypto.Signature, 0, signersNumber)
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	for i := 0; i < signersNumber; i++ {
		_, err := rand.Read(seed)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
		require.NoError(t, err)
		keys = append(keys, sk.PublicKey())
		sig, err := sk.Sign(msg, hasher)
		require.NoError(t, err)
		sigs = append(sigs, sig)
	}
	aggregator, err := NewSignatureAggregatorSameMessage(msg, tag, keys)
	require.NoError(t, err)
	return aggregator, sigs
}

func TestAggregatorSameMessage(t *testing.T) {

	signersNum := 20

	// constrcutor edge cases
	t.Run("constructor", func(t *testing.T) {
		msg := []byte("random_msg")
		tag := "random_tag"
		// empty keys
		_, err := NewSignatureAggregatorSameMessage(msg, tag, []crypto.PublicKey{})
		assert.Error(t, err)
		// wrong key types
		seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)
		_, err = rand.Read(seed)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
		require.NoError(t, err)
		_, err = NewSignatureAggregatorSameMessage(msg, tag, []crypto.PublicKey{sk.PublicKey()})
		assert.Error(t, err)
		_, err = NewSignatureAggregatorSameMessage(msg, tag, []crypto.PublicKey{nil})
		assert.Error(t, err)
	})

	// Happy paths
	t.Run("happy path", func(t *testing.T) {
		aggregator, sigs := createAggregationData(t, signersNum)
		// only add half of the signatures
		subSet := signersNum / 2
		for i, sig := range sigs[subSet:] {
			index := i + subSet
			// test Verify
			ok, err := aggregator.Verify(index, sig)
			assert.NoError(t, err)
			assert.True(t, ok)
			// test HasSignature with existing sig
			ok, err = aggregator.HasSignature(index)
			assert.NoError(t, err)
			assert.False(t, ok)
			// test TrustedAdd
			err = aggregator.TrustedAdd(index, sig)
			assert.NoError(t, err)
			// test HasSignature with non existing sig
			ok, err = aggregator.HasSignature(index)
			assert.NoError(t, err)
			assert.True(t, ok)
		}
		signers, agg, err := aggregator.Aggregate()
		assert.NoError(t, err)
		ok, err := aggregator.VerifyAggregate(signers, agg)
		assert.NoError(t, err)
		assert.True(t, ok)
		// check signers
		sort.Ints(signers)
		for i := 0; i < subSet; i++ {
			index := i + subSet
			assert.Equal(t, index, signers[i])
		}
		// cached aggregated signature
		_, aggCached, err := aggregator.Aggregate()
		assert.NoError(t, err)
		// make sure signature is equal, even though this doesn't mean caching is working
		assert.Equal(t, agg, aggCached)
		// In the following, new signatures are added which makes sure cached signature
		// was cleared.

		// add remaining signatures, this time using VerifyAndAdd
		for i, sig := range sigs[:subSet] {
			ok, err = aggregator.VerifyAndAdd(i, sig)
			assert.True(t, ok)
			assert.NoError(t, err)
		}
		signers, agg, err = aggregator.Aggregate()
		assert.NoError(t, err)
		ok, err = aggregator.VerifyAggregate(signers, agg)
		assert.NoError(t, err)
		assert.True(t, ok)
		// check signers
		sort.Ints(signers)
		for i := 0; i < signersNum; i++ {
			assert.Equal(t, i, signers[i])
		}
	})

	invalidInput := engine.NewInvalidInputError("some error")
	duplicate := newErrDuplicatedSigner("some error")

	// Unhappy paths
	t.Run("invalid inputs", func(t *testing.T) {
		aggregator, sigs := createAggregationData(t, signersNum)
		// loop through invalid inputs
		for _, index := range []int{-1, signersNum} {
			ok, err := aggregator.Verify(index, sigs[0])
			assert.False(t, ok)
			assert.Error(t, err)
			assert.IsType(t, invalidInput, err)

			ok, err = aggregator.VerifyAndAdd(index, sigs[0])
			assert.False(t, ok)
			assert.Error(t, err)
			assert.IsType(t, invalidInput, err)

			err = aggregator.TrustedAdd(index, sigs[0])
			assert.Error(t, err)
			assert.IsType(t, invalidInput, err)

			ok, err = aggregator.HasSignature(index)
			assert.False(t, ok)
			assert.Error(t, err)
			assert.IsType(t, invalidInput, err)

			ok, err = aggregator.VerifyAggregate([]int{index}, sigs[0])
			assert.False(t, ok)
			assert.Error(t, err)
			assert.IsType(t, invalidInput, err)
		}
		// empty list
		ok, err := aggregator.VerifyAggregate([]int{}, sigs[0])
		assert.False(t, ok)
		assert.Error(t, err)
		assert.IsType(t, invalidInput, err)
	})

	t.Run("duplicate signature", func(t *testing.T) {
		aggregator, sigs := createAggregationData(t, signersNum)
		for i, sig := range sigs {
			err := aggregator.TrustedAdd(i, sig)
			require.NoError(t, err)
		}
		// TrustedAdd
		for i := range sigs {
			err := aggregator.TrustedAdd(i, sigs[i]) // same signature for same index
			assert.Error(t, err)
			assert.IsType(t, duplicate, err)
			err = aggregator.TrustedAdd(0, sigs[(i+1)%signersNum]) // different signature for same index
			assert.Error(t, err)
			assert.IsType(t, duplicate, err)
			// VerifyAndAdd
			ok, err := aggregator.VerifyAndAdd(i, sigs[i]) // valid but redundant signature
			assert.False(t, ok)
			assert.Error(t, err)
			assert.IsType(t, duplicate, err)
		}
	})

	t.Run("invalid signature", func(t *testing.T) {
		aggregator, sigs := createAggregationData(t, signersNum)
		// corrupt sigs[0]
		sigs[0][4] ^= 1
		// test Verify
		ok, err := aggregator.Verify(0, sigs[0])
		require.NoError(t, err)
		assert.False(t, ok)
		// test Verify and Add
		ok, err = aggregator.VerifyAndAdd(0, sigs[0])
		require.NoError(t, err)
		assert.False(t, ok)
		// check signature is still not added
		ok, err = aggregator.HasSignature(0)
		require.NoError(t, err)
		assert.False(t, ok)
		// add signatures for aggregation including corrupt sigs[0]
		for i, sig := range sigs {
			err := aggregator.TrustedAdd(i, sig)
			require.NoError(t, err)
		}
		signers, agg, err := aggregator.Aggregate()
		assert.Error(t, err)
		assert.Nil(t, agg)
		assert.Nil(t, signers)
		// fix sigs[0]
		sigs[0][4] ^= 1
	})
}

func TestKeyAggregator(t *testing.T) {
	r := time.Now().UnixNano()
	mrand.Seed(r)
	t.Logf("math rand seed is %d", r)

	signersNum := 20
	// create keys
	indices := make([]int, 0, signersNum)
	keys := make([]crypto.PublicKey, 0, signersNum)
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	for i := 0; i < signersNum; i++ {
		indices = append(indices, i)
		_, err := rand.Read(seed)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
		require.NoError(t, err)
		keys = append(keys, sk.PublicKey())
	}
	aggregator, err := NewPublicKeyAggregator(keys)
	require.NoError(t, err)

	// constrcutor edge cases
	t.Run("constructor", func(t *testing.T) {
		// wrong key types
		seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)
		_, err = rand.Read(seed)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
		require.NoError(t, err)
		_, err = NewPublicKeyAggregator([]crypto.PublicKey{sk.PublicKey()})
		assert.Error(t, err)
		_, err = NewPublicKeyAggregator([]crypto.PublicKey{nil})
		assert.Error(t, err)
	})

	t.Run("greedy algorithm", func(t *testing.T) {
		// iterate over different random cases to make sure
		// the delta algorithm works
		rounds := 30
		for i := 0; i < rounds; i++ {
			go func() { // test module concurrency
				low := mrand.Intn(signersNum)
				high := low + mrand.Intn(signersNum-low)
				var key, expectedKey crypto.PublicKey
				var err error
				key, err = aggregator.KeyAggregate(indices[low:high])
				require.NoError(t, err)
				if low == high {
					expectedKey = crypto.NeutralBLSPublicKey()
				} else {
					expectedKey, err = crypto.AggregateBLSPublicKeys(keys[low:high])
					require.NoError(t, err)
				}
				assert.True(t, key.Equals(expectedKey))
			}()
		}
	})
}
