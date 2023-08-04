package timeoutcollector

import (
	"math/rand"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

// createAggregationData is a helper which creates fixture data for testing
func createAggregationData(t *testing.T, signersNumber int) (
	*TimeoutSignatureAggregator,
	flow.IdentityList,
	[]crypto.PublicKey,
	[]crypto.Signature,
	[]hotstuff.TimeoutSignerInfo,
	[][]byte,
	[]hash.Hasher) {

	// create message and tag
	tag := "random_tag"
	hasher := msig.NewBLSHasher(tag)
	sigs := make([]crypto.Signature, 0, signersNumber)
	signersInfo := make([]hotstuff.TimeoutSignerInfo, 0, signersNumber)
	msgs := make([][]byte, 0, signersNumber)
	hashers := make([]hash.Hasher, 0, signersNumber)

	// create keys, identities and signatures
	ids := make([]*flow.Identity, 0, signersNumber)
	pks := make([]crypto.PublicKey, 0, signersNumber)
	view := 10 + uint64(rand.Uint32())
	for i := 0; i < signersNumber; i++ {
		sk := unittest.PrivateKeyFixture(crypto.BLSBLS12381, crypto.KeyGenSeedMinLen)
		identity := unittest.IdentityFixture(unittest.WithStakingPubKey(sk.PublicKey()))
		// id
		ids = append(ids, identity)
		// keys
		newestQCView := uint64(rand.Intn(int(view)))
		msg := verification.MakeTimeoutMessage(view, newestQCView)
		// signatures
		sig, err := sk.Sign(msg, hasher)
		require.NoError(t, err)
		sigs = append(sigs, sig)

		pks = append(pks, identity.StakingPubKey)
		signersInfo = append(signersInfo, hotstuff.TimeoutSignerInfo{
			NewestQCView: newestQCView,
			Signer:       identity.NodeID,
		})
		hashers = append(hashers, hasher)
		msgs = append(msgs, msg)
	}
	aggregator, err := NewTimeoutSignatureAggregator(view, ids, tag)
	require.NoError(t, err)
	return aggregator, ids, pks, sigs, signersInfo, msgs, hashers
}

// TestNewTimeoutSignatureAggregator tests different happy and unhappy path scenarios when constructing
// multi message signature aggregator.
func TestNewTimeoutSignatureAggregator(t *testing.T) {
	tag := "random_tag"

	sk := unittest.PrivateKeyFixture(crypto.ECDSAP256, crypto.KeyGenSeedMinLen)
	signer := unittest.IdentityFixture(unittest.WithStakingPubKey(sk.PublicKey()))
	// wrong key type
	_, err := NewTimeoutSignatureAggregator(0, flow.IdentityList{signer}, tag)
	require.Error(t, err)
	// empty signers
	_, err = NewTimeoutSignatureAggregator(0, flow.IdentityList{}, tag)
	require.Error(t, err)
}

// TestTimeoutSignatureAggregator_HappyPath tests happy path when aggregating signatures
// Tests verification, adding and aggregation. Test is performed in concurrent environment
func TestTimeoutSignatureAggregator_HappyPath(t *testing.T) {
	signersNum := 20
	aggregator, ids, pks, sigs, signersData, msgs, hashers := createAggregationData(t, signersNum)

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
			// test VerifyAndAdd
			_, err := aggregator.VerifyAndAdd(ids[index].NodeID, sig, signersData[index].NewestQCView)
			// ignore weight as comparing against expected weight is not thread safe
			require.NoError(t, err)
		}(i, sig)
		expectedWeight += ids[i+subSet].Weight
	}

	wg.Wait()
	actualSignersInfo, aggSig, err := aggregator.Aggregate()
	require.NoError(t, err)
	require.ElementsMatch(t, signersData[subSet:], actualSignersInfo)

	ok, err := crypto.VerifyBLSSignatureManyMessages(pks[subSet:], aggSig, msgs[subSet:], hashers[subSet:])
	require.NoError(t, err)
	require.True(t, ok)

	// add remaining signatures in one thread in order to test the returned weight
	for i, sig := range sigs[:subSet] {
		weight, err := aggregator.VerifyAndAdd(ids[i].NodeID, sig, signersData[i].NewestQCView)
		require.NoError(t, err)
		expectedWeight += ids[i].Weight
		require.Equal(t, expectedWeight, weight)
		// test TotalWeight
		require.Equal(t, expectedWeight, aggregator.TotalWeight())
	}
	actualSignersInfo, aggSig, err = aggregator.Aggregate()
	require.NoError(t, err)
	require.ElementsMatch(t, signersData, actualSignersInfo)

	ok, err = crypto.VerifyBLSSignatureManyMessages(pks, aggSig, msgs, hashers)
	require.NoError(t, err)
	require.True(t, ok)
}

// TestTimeoutSignatureAggregator_VerifyAndAdd tests behavior of VerifyAndAdd under invalid input data.
func TestTimeoutSignatureAggregator_VerifyAndAdd(t *testing.T) {
	signersNum := 20

	// Unhappy paths
	t.Run("invalid signer ID", func(t *testing.T) {
		aggregator, _, _, sigs, signersInfo, _, _ := createAggregationData(t, signersNum)
		// generate an ID that is not in the node ID list
		invalidId := unittest.IdentifierFixture()

		weight, err := aggregator.VerifyAndAdd(invalidId, sigs[0], signersInfo[0].NewestQCView)
		require.Equal(t, uint64(0), weight)
		require.Equal(t, uint64(0), aggregator.TotalWeight())
		require.True(t, model.IsInvalidSignerError(err))
	})

	t.Run("duplicate signature", func(t *testing.T) {
		aggregator, ids, _, sigs, signersInfo, _, _ := createAggregationData(t, signersNum)
		expectedWeight := uint64(0)
		// add signatures
		for i, sig := range sigs {
			weight, err := aggregator.VerifyAndAdd(ids[i].NodeID, sig, signersInfo[i].NewestQCView)
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
				weight, err := aggregator.VerifyAndAdd(ids[i].NodeID, sigs[i], signersInfo[i].NewestQCView) // same signature for same index
				// weight should not change
				require.Equal(t, expectedWeight, weight)
				require.True(t, model.IsDuplicatedSignerError(err))
				weight, err = aggregator.VerifyAndAdd(ids[i].NodeID, sigs[(i+1)%signersNum], signersInfo[(i+1)%signersNum].NewestQCView) // different signature for same index
				// weight should not change
				require.Equal(t, expectedWeight, weight)
				require.True(t, model.IsDuplicatedSignerError(err))
				weight, err = aggregator.VerifyAndAdd(ids[(i+1)%signersNum].NodeID, sigs[(i+1)%signersNum], signersInfo[(i+1)%signersNum].NewestQCView) // different signature for same index
				// weight should not change
				require.Equal(t, expectedWeight, weight)
				require.True(t, model.IsDuplicatedSignerError(err))
			}(i, sig)
		}
		wg.Wait()
	})
}

// TestTimeoutSignatureAggregator_Aggregate tests that Aggregate performs internal checks and
// doesn't produce aggregated signature even when feed with invalid signatures.
func TestTimeoutSignatureAggregator_Aggregate(t *testing.T) {
	signersNum := 20

	t.Run("invalid signature", func(t *testing.T) {
		var err error
		aggregator, ids, pks, sigs, signersInfo, msgs, hashers := createAggregationData(t, signersNum)
		// replace sig with random one
		sk := unittest.PrivateKeyFixture(crypto.BLSBLS12381, crypto.KeyGenSeedMinLen)
		sigs[0], err = sk.Sign([]byte("dummy"), hashers[0])
		require.NoError(t, err)

		// test VerifyAndAdd
		_, err = aggregator.VerifyAndAdd(ids[0].NodeID, sigs[0], signersInfo[0].NewestQCView)
		require.ErrorIs(t, err, model.ErrInvalidSignature)

		// add signatures for aggregation including corrupt sigs[0]
		expectedWeight := uint64(0)
		for i, sig := range sigs {
			weight, err := aggregator.VerifyAndAdd(ids[i].NodeID, sig, signersInfo[i].NewestQCView)
			if err == nil {
				expectedWeight += ids[i].Weight
			}
			require.Equal(t, expectedWeight, weight)
		}
		signers, aggSig, err := aggregator.Aggregate()
		require.NoError(t, err)
		// we should have signers for all signatures except first one since it's invalid
		require.Equal(t, len(signers), len(ids)-1)

		ok, err := crypto.VerifyBLSSignatureManyMessages(pks[1:], aggSig, msgs[1:], hashers[1:])
		require.NoError(t, err)
		require.True(t, ok)
	})

	t.Run("aggregating empty set of signatures", func(t *testing.T) {
		aggregator, _, _, _, _, _, _ := createAggregationData(t, signersNum)

		// no signatures were added => aggregate should error with
		signersData, aggSig, err := aggregator.Aggregate()
		require.True(t, model.IsInsufficientSignaturesError(err))
		require.Nil(t, signersData)
		require.Nil(t, aggSig)

		// Also, _after_ attempting to add a signature from unknown `signerID`:
		// calling `Aggregate()` should error with `model.InsufficientSignaturesError`,
		// as still zero signatures are stored.
		_, err = aggregator.VerifyAndAdd(unittest.IdentifierFixture(), unittest.SignatureFixture(), 0)
		require.True(t, model.IsInvalidSignerError(err))

		signersData, aggSig, err = aggregator.Aggregate()
		require.True(t, model.IsInsufficientSignaturesError(err))
		require.Nil(t, signersData)
		require.Nil(t, aggSig)
	})
}
