package timeoutcollector

import (
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"math/rand"
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
	*TimeoutSignatureAggregator,
	flow.IdentityList,
	[]crypto.PublicKey,
	[]crypto.Signature,
	[]uint64,
	[][]byte,
	[]hash.Hasher) {

	// create message and tag
	tag := "random_tag"
	hasher := crypto.NewBLSKMAC(tag)
	sigs := make([]crypto.Signature, 0, signersNumber)
	highQCViews := make([]uint64, 0, signersNumber)
	msgs := make([][]byte, 0, signersNumber)
	hashers := make([]hash.Hasher, 0, signersNumber)

	// create keys, identities and signatures
	ids := make([]*flow.Identity, 0, signersNumber)
	pks := make([]crypto.PublicKey, 0, signersNumber)
	view := uint64(rand.Uint32())
	for i := 0; i < signersNumber; i++ {
		sk := unittest.PrivateKeyFixture(crypto.BLSBLS12381, crypto.KeyGenSeedMinLenECDSAP256)
		identity := unittest.IdentityFixture(unittest.WithStakingPubKey(sk.PublicKey()))
		// id
		ids = append(ids, identity)
		// keys
		highQCView := view + uint64(rand.Intn(signersNumber))
		msg := verification.MakeTimeoutMessage(view, highQCView)
		// signatures
		sig, err := sk.Sign(msg, hasher)
		require.NoError(t, err)
		sigs = append(sigs, sig)

		pks = append(pks, identity.StakingPubKey)
		highQCViews = append(highQCViews, highQCView)
		hashers = append(hashers, hasher)
		msgs = append(msgs, msg)
	}
	aggregator, err := NewTimeoutSignatureAggregator(view, ids, tag)
	require.NoError(t, err)
	return aggregator, ids, pks, sigs, highQCViews, msgs, hashers
}

// TestNewMultiMessageSigAggregator tests different happy and unhappy path scenarios when constructing
// multi message signature aggregator.
func TestNewMultiMessageSigAggregator(t *testing.T) {
	tag := "random_tag"

	sk := unittest.PrivateKeyFixture(crypto.ECDSAP256, crypto.KeyGenSeedMinLenECDSAP256)
	signer := unittest.IdentityFixture(unittest.WithStakingPubKey(sk.PublicKey()))
	// wrong key type
	_, err := NewTimeoutSignatureAggregator(0, flow.IdentityList{signer}, tag)
	require.Error(t, err)
	// empty signers
	_, err = NewTimeoutSignatureAggregator(0, flow.IdentityList{}, tag)
	require.Error(t, err)
}

// TestMultiMessageSignatureAggregator_HappyPath tests happy path when aggregating signatures
// Tests verification, adding and aggregation. Test is performed in concurrent environment
func TestMultiMessageSignatureAggregator_HappyPath(t *testing.T) {
	signersNum := 20
	aggregator, ids, pks, sigs, highQCViews, msgs, hashers := createAggregationData(t, signersNum)

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
			// test TrustedAdd
			_, err := aggregator.VerifyAndAdd(ids[index].NodeID, sig, highQCViews[index])
			// ignore weight as comparing against expected weight is not thread safe
			require.NoError(t, err)
		}(i, sig)
		expectedWeight += ids[i+subSet].Weight
	}

	wg.Wait()
	signers, highQCViewsActual, agg, err := aggregator.Aggregate()
	require.NoError(t, err)
	require.ElementsMatch(t, highQCViews[subSet:], highQCViewsActual)

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
		weight, err := aggregator.VerifyAndAdd(ids[i].NodeID, sig, highQCViews[i])
		require.NoError(t, err)
		expectedWeight += ids[i].Weight
		require.Equal(t, expectedWeight, weight)
		// test TotalWeight
		require.Equal(t, expectedWeight, aggregator.TotalWeight())
	}
	signers, highQCViewsActual, agg, err = aggregator.Aggregate()
	require.NoError(t, err)
	require.ElementsMatch(t, highQCViews, highQCViewsActual)

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

// TestMultiMessageSignatureAggregator_VerifyAndAdd tests behavior of VerifyAndAdd under invalid input data.
func TestMultiMessageSignatureAggregator_VerifyAndAdd(t *testing.T) {
	signersNum := 20

	// Unhappy paths
	t.Run("invalid signer ID", func(t *testing.T) {
		aggregator, _, _, sigs, highQCViews, _, _ := createAggregationData(t, signersNum)
		// generate an ID that is not in the node ID list
		invalidId := unittest.IdentifierFixture()

		weight, err := aggregator.VerifyAndAdd(invalidId, sigs[0], highQCViews[0])
		require.Equal(t, uint64(0), weight)
		require.Equal(t, uint64(0), aggregator.TotalWeight())
		require.True(t, model.IsInvalidSignerError(err))
	})

	t.Run("duplicate signature", func(t *testing.T) {
		aggregator, ids, _, sigs, highQCViews, _, _ := createAggregationData(t, signersNum)
		expectedWeight := uint64(0)
		// add signatures
		for i, sig := range sigs {
			weight, err := aggregator.VerifyAndAdd(ids[i].NodeID, sig, highQCViews[i])
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
				weight, err := aggregator.VerifyAndAdd(ids[i].NodeID, sigs[i], highQCViews[i]) // same signature for same index
				// weight should not change
				require.Equal(t, expectedWeight, weight)
				require.True(t, model.IsDuplicatedSignerError(err))
				weight, err = aggregator.VerifyAndAdd(ids[i].NodeID, sigs[(i+1)%signersNum], highQCViews[(i+1)%signersNum]) // different signature for same index
				// weight should not change
				require.Equal(t, expectedWeight, weight)
				require.True(t, model.IsDuplicatedSignerError(err))
				weight, err = aggregator.VerifyAndAdd(ids[(i+1)%signersNum].NodeID, sigs[(i+1)%signersNum], highQCViews[(i+1)%signersNum]) // different signature for same index
				// weight should not change
				require.Equal(t, expectedWeight, weight)
				require.ErrorAs(t, err, &model.ErrInvalidSignature)
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
		var err error
		aggregator, ids, pks, sigs, highQCViews, msgs, hashers := createAggregationData(t, signersNum)
		// replace sig with random one
		sk := unittest.PrivateKeyFixture(crypto.BLSBLS12381, crypto.KeyGenSeedMinLenECDSAP256)
		sigs[0], err = sk.Sign([]byte("dummy"), hashers[0])
		require.NoError(t, err)

		// test VerifyAndAdd
		_, err = aggregator.VerifyAndAdd(ids[0].NodeID, sigs[0], highQCViews[0])
		require.ErrorIs(t, err, model.ErrInvalidSignature)

		// add signatures for aggregation including corrupt sigs[0]
		expectedWeight := uint64(0)
		for i, sig := range sigs {
			weight, err := aggregator.VerifyAndAdd(ids[i].NodeID, sig, highQCViews[i])
			if err == nil {
				expectedWeight += ids[i].Weight
			}
			require.Equal(t, expectedWeight, weight)
		}
		signers, _, agg, err := aggregator.Aggregate()
		require.NoError(t, err)
		// we should have signers for all signatures except first one since it's invalid
		require.Equal(t, len(signers), len(ids)-1)

		ok, err := crypto.VerifyBLSSignatureManyMessages(pks[1:], agg, msgs[1:], hashers[1:])
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("aggregating empty set of signatures", func(t *testing.T) {
		aggregator, _, _, _, _, _, _ := createAggregationData(t, signersNum)

		// no signatures were added => aggregate should error with
		signers, highQCViews, agg, err := aggregator.Aggregate()
		require.True(t, model.IsInsufficientSignaturesError(err))
		require.Nil(t, agg)
		require.Nil(t, signers)
		require.Nil(t, highQCViews)

		// Also, _after_ attempting to add a signature from unknown `signerID`:
		// calling `Aggregate()` should error with `model.InsufficientSignaturesError`,
		// as still zero signatures are stored.
		_, err = aggregator.VerifyAndAdd(unittest.IdentifierFixture(), unittest.SignatureFixture(), 0)
		require.True(t, model.IsInvalidSignerError(err))

		signers, highQCViews, agg, err = aggregator.Aggregate()
		require.True(t, model.IsInsufficientSignaturesError(err))
		require.Nil(t, agg)
		require.Nil(t, signers)
		require.Nil(t, highQCViews)
	})
}
