package signature

import (
	"crypto/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func createAggregationData(t *testing.T, signersNumber int) (
	hotstuff.WeightedSignatureAggregator,
	flow.IdentityList,
	[]crypto.PublicKey,
	[]crypto.Signature,
	[]byte,
	hash.Hasher) {

	// create message and tag
	msgLen := 100
	msg := make([]byte, msgLen)
	tag := "random_tag"
	hasher := crypto.NewBLSKMAC(tag)

	// create keys, identities and signatures
	ids := make([]*flow.Identity, 0, signersNumber)
	sigs := make([]crypto.Signature, 0, signersNumber)
	pks := make([]crypto.PublicKey, 0, signersNumber)
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	for i := 0; i < signersNumber; i++ {
		// id
		ids = append(ids, unittest.IdentityFixture())
		// keys
		_, err := rand.Read(seed)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
		require.NoError(t, err)
		pks = append(pks, sk.PublicKey())
		// signatures
		sig, err := sk.Sign(msg, hasher)
		require.NoError(t, err)
		sigs = append(sigs, sig)
	}
	aggregator, err := NewWeightedSignatureAggregator(ids, pks, msg, tag)
	require.NoError(t, err)
	return aggregator, ids, pks, sigs, msg, hasher
}

func TestWeightedSignatureAggregator(t *testing.T) {
	signersNum := 20

	// constrcutor edge cases
	t.Run("constructor", func(t *testing.T) {
		msg := []byte("random_msg")
		tag := "random_tag"

		signer := unittest.IdentityFixture()
		// identity with empty key
		_, err := NewWeightedSignatureAggregator(flow.IdentityList{signer}, []crypto.PublicKey{nil}, msg, tag)
		assert.Error(t, err)
		// wrong key type
		seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)
		_, err = rand.Read(seed)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
		require.NoError(t, err)
		pk := sk.PublicKey()
		_, err = NewWeightedSignatureAggregator(flow.IdentityList{signer}, []crypto.PublicKey{pk}, msg, tag)
		assert.Error(t, err)
		// empty signers
		_, err = NewWeightedSignatureAggregator(flow.IdentityList{}, []crypto.PublicKey{}, msg, tag)
		assert.Error(t, err)
		// mismatching input lengths
		sk, err = crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
		require.NoError(t, err)
		pk = sk.PublicKey()
		_, err = NewWeightedSignatureAggregator(flow.IdentityList{signer}, []crypto.PublicKey{pk, pk}, msg, tag)
		assert.Error(t, err)
	})

	// Happy paths
	t.Run("happy path and thread safety", func(t *testing.T) {
		aggregator, ids, pks, sigs, msg, hasher := createAggregationData(t, signersNum)
		// only add a subset of the signatures
		subSet := signersNum / 2
		expectedWeight := uint64(0)
		var wg sync.WaitGroup
		for i, sig := range sigs[subSet:] {
			wg.Add(1)
			// test thread safety
			go func(i int, sig crypto.Signature) {
				defer wg.Done()
				index := i + subSet
				// test Verify
				err := aggregator.Verify(ids[index].NodeID, sig)
				assert.NoError(t, err)
				// test TrustedAdd
				_, err = aggregator.TrustedAdd(ids[index].NodeID, sig)
				// ignore weight as comparing against expected weight is not thread safe
				assert.NoError(t, err)
			}(i, sig)
			expectedWeight += ids[i+subSet].Weight
		}

		wg.Wait()
		signers, agg, err := aggregator.Aggregate()
		assert.NoError(t, err)
		ok, err := crypto.VerifyBLSSignatureOneMessage(pks[subSet:], agg, msg, hasher)
		assert.NoError(t, err)
		assert.True(t, ok)
		// check signers
		identifiers := make([]flow.Identifier, 0, signersNum-subSet)
		for i := subSet; i < signersNum; i++ {
			identifiers = append(identifiers, ids[i].NodeID)
		}
		assert.ElementsMatch(t, signers, identifiers)

		// add remaining signatures in one thread in order to test the returned weight
		for i, sig := range sigs[:subSet] {
			weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig)
			assert.NoError(t, err)
			expectedWeight += ids[i].Weight
			assert.Equal(t, expectedWeight, weight)
			// test TotalWeight
			assert.Equal(t, expectedWeight, aggregator.TotalWeight())
		}
		signers, agg, err = aggregator.Aggregate()
		assert.NoError(t, err)
		ok, err = crypto.VerifyBLSSignatureOneMessage(pks, agg, msg, hasher)
		assert.NoError(t, err)
		assert.True(t, ok)
		// check signers
		identifiers = make([]flow.Identifier, 0, signersNum)
		for i := 0; i < signersNum; i++ {
			identifiers = append(identifiers, ids[i].NodeID)
		}
		assert.ElementsMatch(t, signers, identifiers)
	})

	// Unhappy paths
	t.Run("invalid signer ID", func(t *testing.T) {
		aggregator, _, _, sigs, _, _ := createAggregationData(t, signersNum)
		// generate an ID that is not in the node ID list
		invalidId := unittest.IdentifierFixture()

		err := aggregator.Verify(invalidId, sigs[0])
		assert.True(t, model.IsInvalidSignerError(err))

		weight, err := aggregator.TrustedAdd(invalidId, sigs[0])
		assert.Equal(t, uint64(0), weight)
		assert.Equal(t, uint64(0), aggregator.TotalWeight())
		assert.True(t, model.IsInvalidSignerError(err))
	})

	t.Run("duplicate signature", func(t *testing.T) {
		aggregator, ids, _, sigs, _, _ := createAggregationData(t, signersNum)
		expectedWeight := uint64(0)
		// add signatures
		for i, sig := range sigs {
			weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig)
			expectedWeight += ids[i].Weight
			assert.Equal(t, expectedWeight, weight)
			require.NoError(t, err)
		}
		// add same duplicates and test thread safety
		var wg sync.WaitGroup
		for i, sig := range sigs {
			wg.Add(1)
			// test thread safety
			go func(i int, sig crypto.Signature) {
				defer wg.Done()
				weight, err := aggregator.TrustedAdd(ids[i].NodeID, sigs[i]) // same signature for same index
				// weight should not change
				assert.Equal(t, expectedWeight, weight)
				assert.True(t, model.IsDuplicatedSignerError(err))
				weight, err = aggregator.TrustedAdd(ids[i].NodeID, sigs[(i+1)%signersNum]) // different signature for same index
				// weight should not change
				assert.Equal(t, expectedWeight, weight)
				assert.True(t, model.IsDuplicatedSignerError(err))
			}(i, sig)
		}
		wg.Wait()
	})

	t.Run("invalid signature", func(t *testing.T) {
		aggregator, ids, _, sigs, _, _ := createAggregationData(t, signersNum)
		// corrupt sigs[0]
		sigs[0][4] ^= 1
		// test Verify
		err := aggregator.Verify(ids[0].NodeID, sigs[0])
		assert.ErrorIs(t, err, model.ErrInvalidSignature)

		// add signatures for aggregation including corrupt sigs[0]
		expectedWeight := uint64(0)
		for i, sig := range sigs {
			weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig)
			require.NoError(t, err)
			expectedWeight += ids[i].Weight
			assert.Equal(t, expectedWeight, weight)
		}
		signers, agg, err := aggregator.Aggregate()
		assert.True(t, model.IsInvalidSignatureIncludedError(err))
		assert.Nil(t, agg)
		assert.Nil(t, signers)
		// fix sigs[0]
		sigs[0][4] ^= 1
	})

	t.Run("aggregating empty set of signatures", func(t *testing.T) {
		aggregator, _, _, _, _, _ := createAggregationData(t, signersNum)

		// no signatures were added => aggregate should error with
		signers, agg, err := aggregator.Aggregate()
		assert.True(t, model.IsInsufficientSignaturesError(err))
		assert.Nil(t, agg)
		assert.Nil(t, signers)

		// Also, _after_ attempting to add a signature from unknown `signerID`:
		// calling `Aggregate()` should error with `model.InsufficientSignaturesError`,
		// as still zero signatures are stored.
		_, err = aggregator.TrustedAdd(unittest.IdentifierFixture(), unittest.SignatureFixture())
		assert.True(t, model.IsInvalidSignerError(err))
		_, err = aggregator.TrustedAdd(unittest.IdentifierFixture(), unittest.SignatureFixture())
		assert.True(t, model.IsInvalidSignerError(err))

		signers, agg, err = aggregator.Aggregate()
		assert.True(t, model.IsInsufficientSignaturesError(err))
		assert.Nil(t, agg)
		assert.Nil(t, signers)
	})

}
