package signature

import (
	"bytes"
	"crypto/rand"
	"errors"
	"sort"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func sortIdentities(ids []flow.Identity) []flow.Identity {
	canonicalOrder := func(i, j int) bool {
		return bytes.Compare(ids[i].NodeID[:], ids[j].NodeID[:]) < 0
	}
	sort.Slice(ids, canonicalOrder)
	return ids
}

func sortIdentifiers(ids []flow.Identifier) []flow.Identifier {
	canonicalOrder := func(i, j int) bool {
		return bytes.Compare(ids[i][:], ids[j][:]) < 0
	}
	sort.Slice(ids, canonicalOrder)
	return ids
}

func createAggregationData(t *testing.T, signersNumber int) (
	hotstuff.WeightedSignatureAggregator, []flow.Identity, []crypto.Signature, []byte, string) {
	// create identities
	ids := make([]flow.Identity, 0, signersNumber)
	for i := 0; i < signersNumber; i++ {
		ids = append(ids, *unittest.IdentityFixture())
	}
	ids = sortIdentities(ids)

	// create message and tag
	msgLen := 100
	msg := make([]byte, msgLen)
	tag := "random_tag"
	hasher := crypto.NewBLSKMAC(tag)

	// create keys, identities and signatures
	sigs := make([]crypto.Signature, 0, signersNumber)
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	for i := 0; i < signersNumber; i++ {
		// keys
		_, err := rand.Read(seed)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
		require.NoError(t, err)
		ids[i].StakingPubKey = sk.PublicKey()
		// signatures
		sig, err := sk.Sign(msg, hasher)
		require.NoError(t, err)
		sigs = append(sigs, sig)
	}
	aggregator, err := NewWeightedSignatureAggregator(ids, msg, tag)
	require.NoError(t, err)
	return aggregator, ids, sigs, msg, tag
}

func verifyAggregate(signers []flow.Identifier, ids []flow.Identity, sig []byte,
	msg []byte, tag string) (bool, error) {
	// query identity using identifier
	// done linearly just for testing
	getIdentity := func(signer flow.Identifier) *flow.Identity {
		for _, id := range ids {
			if id.NodeID == signer {
				return &id
			}
		}
		return nil
	}

	// get keys
	keys := make([]crypto.PublicKey, 0, len(ids))
	for _, signer := range signers {
		id := getIdentity(signer)
		if id == nil {
			return false, errors.New("unexpected test error")
		}
		keys = append(keys, id.StakingPubKey)
	}
	// verify signature
	hasher := crypto.NewBLSKMAC(tag)
	return crypto.VerifyBLSSignatureOneMessage(keys, sig, msg, hasher)
}

func TestWeightedSignatureAggregator(t *testing.T) {
	signersNum := 20

	// constrcutor edge cases
	t.Run("constructor", func(t *testing.T) {
		msg := []byte("random_msg")
		tag := "random_tag"

		signer := unittest.IdentityFixture()
		// identity with empty key
		_, err := NewWeightedSignatureAggregator([]flow.Identity{*signer}, msg, tag)
		assert.Error(t, err)
		// wrong key types
		seed := make([]byte, crypto.KeyGenSeedMinLenECDSAP256)
		_, err = rand.Read(seed)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
		require.NoError(t, err)
		signer.StakingPubKey = sk.PublicKey()
		_, err = NewWeightedSignatureAggregator([]flow.Identity{*signer}, msg, tag)
		assert.Error(t, err)
		// empty signers
		_, err = NewWeightedSignatureAggregator([]flow.Identity{}, msg, tag)
		assert.Error(t, err)
	})

	// Happy paths
	t.Run("happy path and thread safety", func(t *testing.T) {
		aggregator, ids, sigs, msg, tag := createAggregationData(t, signersNum)
		// only add half of the signatures
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
			expectedWeight += ids[i+subSet].Stake
		}

		wg.Wait()
		signers, agg, err := aggregator.Aggregate()
		assert.NoError(t, err)
		ok, err := verifyAggregate(signers, ids, agg, msg, tag)
		assert.NoError(t, err)
		assert.True(t, ok)
		// check signers
		signers = sortIdentifiers(signers)
		for i := 0; i < subSet; i++ {
			index := i + subSet
			assert.Equal(t, signers[i], ids[index].NodeID)
		}

		// add remaining signatures in one thread in order to test the returned weight
		for i, sig := range sigs[:subSet] {
			weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig)
			assert.NoError(t, err)
			expectedWeight += ids[i].Stake
			assert.Equal(t, expectedWeight, weight)
			// test TotalWeight
			assert.Equal(t, expectedWeight, aggregator.TotalWeight())
		}
		signers, agg, err = aggregator.Aggregate()
		assert.NoError(t, err)
		ok, err = verifyAggregate(signers, ids, agg, msg, tag)
		assert.NoError(t, err)
		assert.True(t, ok)
		// check signers
		signers = sortIdentifiers(signers)
		for i := 0; i < signersNum; i++ {
			assert.Equal(t, signers[i], ids[i].NodeID)
		}
	})

	invalidInput := engine.NewInvalidInputError("some error")
	duplicate := engine.NewDuplicatedEntryErrorf("some error")
	invalidSig := signature.NewErrInvalidFormat("some error")

	// Unhappy paths
	t.Run("invalid signer ID", func(t *testing.T) {
		aggregator, _, sigs, _, _ := createAggregationData(t, signersNum)
		// generate an ID that is not in the node ID list
		invalidId := unittest.IdentityFixture().NodeID

		err := aggregator.Verify(invalidId, sigs[0])
		assert.Error(t, err)
		assert.IsType(t, invalidInput, err)

		weight, err := aggregator.TrustedAdd(invalidId, sigs[0])
		assert.Equal(t, uint64(0), weight)
		assert.Equal(t, uint64(0), aggregator.TotalWeight())
		assert.Error(t, err)
		assert.IsType(t, invalidInput, err)
	})

	t.Run("duplicate signature", func(t *testing.T) {
		aggregator, ids, sigs, _, _ := createAggregationData(t, signersNum)
		expectedWeight := uint64(0)
		// add signatures
		for i, sig := range sigs {
			weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig)
			expectedWeight += ids[i].Stake
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
				assert.Error(t, err)
				assert.IsType(t, duplicate, err)
				weight, err = aggregator.TrustedAdd(ids[i].NodeID, sigs[(i+1)%signersNum]) // different signature for same index
				// weight should not change
				assert.Equal(t, expectedWeight, weight)
				assert.Error(t, err)
				assert.IsType(t, duplicate, err)
			}(i, sig)
		}
		wg.Wait()
	})

	t.Run("invalid signature", func(t *testing.T) {
		aggregator, ids, sigs, _, _ := createAggregationData(t, signersNum)
		// corrupt sigs[0]
		sigs[0][4] ^= 1
		// test Verify
		err := aggregator.Verify(ids[0].NodeID, sigs[0])
		require.Error(t, err)
		assert.IsType(t, invalidSig, err)

		// add signatures for aggregation including corrupt sigs[0]
		expectedWeight := uint64(0)
		for i, sig := range sigs {
			weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig)
			require.NoError(t, err)
			expectedWeight += ids[i].Stake
			assert.Equal(t, expectedWeight, weight)
		}
		signers, agg, err := aggregator.Aggregate()
		assert.Error(t, err)
		assert.Nil(t, agg)
		assert.Nil(t, signers)
		// fix sigs[0]
		sigs[0][4] ^= 1
	})
}
