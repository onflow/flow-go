package timeoutcollector

import (
	"crypto/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// createAggregationData is a helper which creates fixture data for testing
func createAggregationData(t *testing.T, signersNumber int) (
	*WeightedMultiMessageSignatureAggregator,
	flow.IdentityList,
	[]crypto.PublicKey,
	[]crypto.Signature,
	[][]byte,
	[]hash.Hasher) {

	// create message and tag
	tag := "random_tag"
	hasher := crypto.NewBLSKMAC(tag)
	sigs := make([]crypto.Signature, 0, signersNumber)
	msgs := make([][]byte, 0, signersNumber)
	hashers := make([]hash.Hasher, 0, signersNumber)

	// create keys, identities and signatures
	ids := make([]*flow.Identity, 0, signersNumber)
	pks := make([]crypto.PublicKey, 0, signersNumber)
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	for i := 0; i < signersNumber; i++ {
		identity := unittest.IdentityFixture()
		// id
		ids = append(ids, identity)
		// keys
		_, err := rand.Read(seed)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
		require.NoError(t, err)
		pks = append(pks, sk.PublicKey())
		msg := identity.NodeID[:]
		// signatures
		sig, err := sk.Sign(msg, hasher)
		require.NoError(t, err)
		sigs = append(sigs, sig)

		msgs = append(msgs, msg)
		hashers = append(hashers, hasher)
	}
	aggregator, err := NewWeightedMultiMessageSigAggregator(ids, pks, tag)
	require.NoError(t, err)
	return aggregator, ids, pks, sigs, msgs, hashers
}

// TestNewMultiMessageSigAggregator tests different happy and unhappy path scenarios when constructing
// multi message signature aggregator.
func TestNewMultiMessageSigAggregator(t *testing.T) {
	tag := "random_tag"

	signer := unittest.IdentityFixture()
	// identity with empty key
	_, err := NewWeightedMultiMessageSigAggregator(flow.IdentityList{signer}, []crypto.PublicKey{nil}, tag)
	require.Error(t, err)
	// wrong key type
	seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)
	_, err = rand.Read(seed)
	require.NoError(t, err)
	sk, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
	require.NoError(t, err)
	pk := sk.PublicKey()
	_, err = NewWeightedMultiMessageSigAggregator(flow.IdentityList{signer}, []crypto.PublicKey{pk}, tag)
	require.Error(t, err)
	// empty signers
	_, err = NewWeightedMultiMessageSigAggregator(flow.IdentityList{}, []crypto.PublicKey{}, tag)
	require.Error(t, err)
	// mismatching input lengths
	sk, err = crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	require.NoError(t, err)
	pk = sk.PublicKey()
	_, err = NewWeightedMultiMessageSigAggregator(flow.IdentityList{signer}, []crypto.PublicKey{pk, pk}, tag)
	require.Error(t, err)
}

// TestMultiMessageSignatureAggregator_HappyPath tests happy path when aggregating signatures
// Tests verification, adding and aggregation. Test is performed in concurrent environment
func TestMultiMessageSignatureAggregator_HappyPath(t *testing.T) {
	signersNum := 20
	aggregator, ids, pks, sigs, msgs, hashers := createAggregationData(t, signersNum)

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
			err := aggregator.Verify(ids[index].NodeID, sig, msgs[index])
			require.NoError(t, err)
			// test TrustedAdd
			_, err = aggregator.TrustedAdd(ids[index].NodeID, sig, msgs[index])
			// ignore weight as comparing against expected weight is not thread safe
			require.NoError(t, err)
		}(i, sig)
		expectedWeight += ids[i+subSet].Weight
	}

	wg.Wait()
	signers, agg, err := aggregator.Aggregate()
	require.NoError(t, err)
	ok, err := crypto.VerifyBLSSignatureManyMessages(pks[subSet:], agg, msgs[subSet:], hashers[subSet:])
	require.NoError(t, err)
	require.True(t, ok)
	// check signers
	identifiers := make([]flow.Identifier, 0, signersNum-subSet)
	for i := subSet; i < signersNum; i++ {
		identifiers = append(identifiers, ids[i].NodeID)
	}
	require.ElementsMatch(t, signers, identifiers)

	// add remaining signatures in one thread in order to test the returned weight
	for i, sig := range sigs[:subSet] {
		weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig, msgs[i])
		require.NoError(t, err)
		expectedWeight += ids[i].Weight
		require.Equal(t, expectedWeight, weight)
		// test TotalWeight
		require.Equal(t, expectedWeight, aggregator.TotalWeight())
	}
	signers, agg, err = aggregator.Aggregate()
	require.NoError(t, err)
	ok, err = crypto.VerifyBLSSignatureManyMessages(pks, agg, msgs, hashers)
	require.NoError(t, err)
	require.True(t, ok)
	// check signers
	identifiers = make([]flow.Identifier, 0, signersNum)
	for i := 0; i < signersNum; i++ {
		identifiers = append(identifiers, ids[i].NodeID)
	}
	require.ElementsMatch(t, signers, identifiers)
}

// TestMultiMessageSignatureAggregator_TrustedAdd tests behavior of TrustedAdd under invalid input data.
func TestMultiMessageSignatureAggregator_TrustedAdd(t *testing.T) {
	signersNum := 20

	// Unhappy paths
	t.Run("invalid signer ID", func(t *testing.T) {
		aggregator, _, _, sigs, msgs, _ := createAggregationData(t, signersNum)
		// generate an ID that is not in the node ID list
		invalidId := unittest.IdentifierFixture()

		err := aggregator.Verify(invalidId, sigs[0], msgs[0])
		require.True(t, model.IsInvalidSignerError(err))

		weight, err := aggregator.TrustedAdd(invalidId, sigs[0], msgs[0])
		require.Equal(t, uint64(0), weight)
		require.Equal(t, uint64(0), aggregator.TotalWeight())
		require.True(t, model.IsInvalidSignerError(err))
	})

	t.Run("duplicate signature", func(t *testing.T) {
		aggregator, ids, _, sigs, msgs, _ := createAggregationData(t, signersNum)
		expectedWeight := uint64(0)
		// add signatures
		for i, sig := range sigs {
			weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig, msgs[i])
			expectedWeight += ids[i].Weight
			require.Equal(t, expectedWeight, weight)
			require.NoError(t, err)
		}
		// add same duplicates and test thread safety
		var wg sync.WaitGroup
		for i, sig := range sigs {
			wg.Add(1)
			// test thread safety
			go func(i int, sig crypto.Signature) {
				defer wg.Done()
				weight, err := aggregator.TrustedAdd(ids[i].NodeID, sigs[i], msgs[i]) // same signature for same index
				// weight should not change
				require.Equal(t, expectedWeight, weight)
				require.True(t, model.IsDuplicatedSignerError(err))
				weight, err = aggregator.TrustedAdd(ids[i].NodeID, sigs[(i+1)%signersNum], msgs[(i+1)%signersNum]) // different signature for same index
				// weight should not change
				require.Equal(t, expectedWeight, weight)
				require.True(t, model.IsDuplicatedSignerError(err))
			}(i, sig)
		}
		wg.Wait()
	})
}

// TestMultiMessageSignatureAggregator_Aggregate tests that Aggregate performs internal checks and
// doesn't produce aggregated signature even when feed with invalid signatures.
func TestMultiMessageSignatureAggregator_Aggregate(t *testing.T) {
	signersNum := 20

	t.Run("invalid signature", func(t *testing.T) {
		aggregator, ids, _, sigs, msgs, _ := createAggregationData(t, signersNum)
		// corrupt sigs[0]
		sigs[0][4] ^= 1
		// test Verify
		err := aggregator.Verify(ids[0].NodeID, sigs[0], msgs[0])
		require.ErrorIs(t, err, model.ErrInvalidSignature)

		// add signatures for aggregation including corrupt sigs[0]
		expectedWeight := uint64(0)
		for i, sig := range sigs {
			weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig, msgs[i])
			require.NoError(t, err)
			expectedWeight += ids[i].Weight
			require.Equal(t, expectedWeight, weight)
		}
		signers, agg, err := aggregator.Aggregate()
		require.True(t, model.IsInvalidSignatureIncludedError(err))
		require.Nil(t, agg)
		require.Nil(t, signers)
		// fix sigs[0]
		sigs[0][4] ^= 1
	})

	t.Run("aggregating empty set of signatures", func(t *testing.T) {
		aggregator, _, _, _, _, _ := createAggregationData(t, signersNum)

		// no signatures were added => aggregate should error with
		signers, agg, err := aggregator.Aggregate()
		require.True(t, model.IsInsufficientSignaturesError(err))
		require.Nil(t, agg)
		require.Nil(t, signers)

		// Also, _after_ attempting to add a signature from unknown `signerID`:
		// calling `Aggregate()` should error with `model.InsufficientSignaturesError`,
		// as still zero signatures are stored.
		_, err = aggregator.TrustedAdd(unittest.IdentifierFixture(), unittest.SignatureFixture(), []byte("random-msg"))
		require.True(t, model.IsInvalidSignerError(err))
		_, err = aggregator.TrustedAdd(unittest.IdentifierFixture(), unittest.SignatureFixture(), []byte("random-msg"))
		require.True(t, model.IsInvalidSignerError(err))

		signers, agg, err = aggregator.Aggregate()
		require.True(t, model.IsInsufficientSignaturesError(err))
		require.Nil(t, agg)
		require.Nil(t, signers)
	})
}
