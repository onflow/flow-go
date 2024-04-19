package signature

import (
	"crypto/rand"
	"sync"
	"testing"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	msig "github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

// Utility function that flips a point sign bit to negate the point
// this is shortcut which works only for zcash BLS12-381 compressed serialization
// that is currently supported by the flow crypto module
// Applicable to both signatures and public keys
func negatePoint(pointbytes []byte) {
	pointbytes[0] ^= 0x20
}

func createAggregationData(t *testing.T, signersNumber int) (
	flow.IdentityList,
	[]crypto.PublicKey,
	[]crypto.Signature,
	[]byte,
	hash.Hasher,
	string) {

	// create message and tag
	msgLen := 100
	msg := make([]byte, msgLen)
	_, err := rand.Read(msg)
	require.NoError(t, err)
	tag := "random_tag"
	hasher := msig.NewBLSHasher(tag)

	// create keys, identities and signatures
	ids := make([]*flow.Identity, 0, signersNumber)
	sigs := make([]crypto.Signature, 0, signersNumber)
	pks := make([]crypto.PublicKey, 0, signersNumber)
	seed := make([]byte, crypto.KeyGenSeedMinLen)
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
	return ids, pks, sigs, msg, hasher, tag
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
		seed := make([]byte, crypto.KeyGenSeedMinLen)
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
		ids, pks, sigs, msg, hasher, tag := createAggregationData(t, signersNum)
		aggregator, err := NewWeightedSignatureAggregator(ids, pks, msg, tag)
		require.NoError(t, err)
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
			expectedWeight += ids[i+subSet].InitialWeight
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
			expectedWeight += ids[i].InitialWeight
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
		ids, pks, sigs, msg, _, tag := createAggregationData(t, signersNum)
		aggregator, err := NewWeightedSignatureAggregator(ids, pks, msg, tag)
		require.NoError(t, err)
		// generate an ID that is not in the node ID list
		invalidId := unittest.IdentifierFixture()

		err = aggregator.Verify(invalidId, sigs[0])
		assert.True(t, model.IsInvalidSignerError(err))

		weight, err := aggregator.TrustedAdd(invalidId, sigs[0])
		assert.Equal(t, uint64(0), weight)
		assert.Equal(t, uint64(0), aggregator.TotalWeight())
		assert.True(t, model.IsInvalidSignerError(err))
	})

	t.Run("duplicate signature", func(t *testing.T) {
		ids, pks, sigs, msg, _, tag := createAggregationData(t, signersNum)
		aggregator, err := NewWeightedSignatureAggregator(ids, pks, msg, tag)
		require.NoError(t, err)

		expectedWeight := uint64(0)
		// add signatures
		for i, sig := range sigs {
			weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig)
			expectedWeight += ids[i].InitialWeight
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

	// The following tests are related to the `Aggregate()` method.
	// Generally, `Aggregate()` can fail in four cases:
	//  1. No signature has been added.
	//  2. A signature added via `TrustedAdd` has an invalid structure (fails to deserialize)
	//      2.a. aggregated public key is not identity
	//      2.b. aggregated public key is identity
	//  3. Signatures serialization is valid but some signatures are invalid w.r.t their respective public keys.
	//      3.a. aggregated public key is not identity
	//      3.b. aggregated public key is identity
	//  4. All signatures are valid but aggregated key is identity

	//  1. No signature has been added.
	t.Run("aggregating empty set of signatures", func(t *testing.T) {
		ids, pks, _, msg, _, tag := createAggregationData(t, signersNum)
		aggregator, err := NewWeightedSignatureAggregator(ids, pks, msg, tag)
		require.NoError(t, err)

		// no signatures were added => aggregate should error with IsInsufficientSignaturesError
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

	//  2. A signature added via `TrustedAdd` has an invalid structure (fails to deserialize)
	//      2.a. aggregated public key is not identity
	//      2.b. aggregated public key is identity
	t.Run("invalid signature serialization", func(t *testing.T) {
		ids, pks, sigs, msg, _, tag := createAggregationData(t, signersNum)
		// sigs[0] has an invalid struct
		sigs[0] = (crypto.Signature)([]byte{0, 0})

		t.Run("with non-identity aggregated public key", func(t *testing.T) {
			aggregator, err := NewWeightedSignatureAggregator(ids, pks, msg, tag)
			require.NoError(t, err)

			// test Verify
			err = aggregator.Verify(ids[0].NodeID, sigs[0])
			assert.ErrorIs(t, err, model.ErrInvalidSignature)

			// add signatures for aggregation including corrupt sigs[0]
			expectedWeight := uint64(0)
			for i, sig := range sigs {
				weight, err := aggregator.TrustedAdd(ids[i].NodeID, sig)
				require.NoError(t, err)
				expectedWeight += ids[i].InitialWeight
				assert.Equal(t, expectedWeight, weight)
			}

			// Aggregation should error with sentinel InvalidSignatureIncludedError
			// aggregated public key is not identity (equal to sum of all pks)
			signers, agg, err := aggregator.Aggregate()
			assert.True(t, model.IsInvalidSignatureIncludedError(err))
			assert.Nil(t, agg)
			assert.Nil(t, signers)
		})

		t.Run("with identity aggregated public key", func(t *testing.T) {
			// assign  pk1 to -pk0 so that the aggregated public key is identity
			pkBytes := pks[0].Encode()
			negatePoint(pkBytes)
			var err error
			pks[1], err = crypto.DecodePublicKey(crypto.BLSBLS12381, pkBytes)
			require.NoError(t, err)

			// aggregator with two signers
			aggregator, err := NewWeightedSignatureAggregator(ids[:2], pks[:2], msg, tag)
			require.NoError(t, err)

			// add the invalid signature on index 0
			_, err = aggregator.TrustedAdd(ids[0].NodeID, sigs[0])
			require.NoError(t, err)

			// add a second signature for index 1
			_, err = aggregator.TrustedAdd(ids[1].NodeID, sigs[1])
			require.NoError(t, err)

			// Aggregation should error with sentinel InvalidAggregatedKeyError or InvalidSignatureIncludedError
			// aggregated public key is identity
			signers, agg, err := aggregator.Aggregate()
			assert.True(t, model.IsInvalidSignatureIncludedError(err) || model.IsInvalidAggregatedKeyError(err))
			assert.Nil(t, agg)
			assert.Nil(t, signers)
		})
	})

	//  3. Signatures serialization is valid but some signatures are invalid w.r.t their respective public keys.
	//      3.a. aggregated public key is not identity
	//      3.b. aggregated public key is identity
	t.Run("correct serialization and invalid signature", func(t *testing.T) {
		ids, pks, sigs, msg, _, tag := createAggregationData(t, 2)

		t.Run("with non-identity aggregated public key", func(t *testing.T) {
			aggregator, err := NewWeightedSignatureAggregator(ids, pks, msg, tag)
			require.NoError(t, err)

			// add a valid signature
			err = aggregator.Verify(ids[0].NodeID, sigs[0])
			require.NoError(t, err)
			_, err = aggregator.TrustedAdd(ids[0].NodeID, sigs[0])
			require.NoError(t, err)

			// add invalid signature for signer with index 1
			// sanity check: Verify should reject it
			err = aggregator.Verify(ids[1].NodeID, sigs[0])
			assert.ErrorIs(t, err, model.ErrInvalidSignature)
			_, err = aggregator.TrustedAdd(ids[1].NodeID, sigs[0])
			require.NoError(t, err)

			// Aggregation should error with sentinel InvalidSignatureIncludedError
			// aggregated public key is not identity (equal to pk[0] + pk[1])
			signers, agg, err := aggregator.Aggregate()
			assert.Error(t, err)
			assert.True(t, model.IsInvalidSignatureIncludedError(err))
			assert.Nil(t, agg)
			assert.Nil(t, signers)
		})

		t.Run("with identity aggregated public key", func(t *testing.T) {
			// assign  pk1 to -pk0 so that the aggregated public key is identity
			// this is a shortcut since PoPs are not checked in this test
			pkBytes := pks[0].Encode()
			negatePoint(pkBytes)
			var err error
			pks[1], err = crypto.DecodePublicKey(crypto.BLSBLS12381, pkBytes)
			require.NoError(t, err)

			aggregator, err := NewWeightedSignatureAggregator(ids, pks, msg, tag)
			require.NoError(t, err)

			// add a valid signature for index 0
			err = aggregator.Verify(ids[0].NodeID, sigs[0])
			require.NoError(t, err)
			_, err = aggregator.TrustedAdd(ids[0].NodeID, sigs[0])
			require.NoError(t, err)

			// add an invalid signature for signer with index 1
			// sanity check: Verify should reject it
			err = aggregator.Verify(ids[1].NodeID, sigs[0])
			assert.ErrorIs(t, err, model.ErrInvalidSignature)
			_, err = aggregator.TrustedAdd(ids[1].NodeID, sigs[0])
			require.NoError(t, err)

			// Aggregation should error with sentinel InvalidAggregatedKeyError or InvalidSignatureIncludedError
			// aggregated public key is identity
			signers, agg, err := aggregator.Aggregate()
			assert.Error(t, err)
			assert.True(t, model.IsInvalidSignatureIncludedError(err) || model.IsInvalidAggregatedKeyError(err))
			assert.Nil(t, agg)
			assert.Nil(t, signers)
		})
	})

	//  4. All signatures are valid but aggregated key is identity
	t.Run("identity aggregated key resulting in an invalid aggregated signature", func(t *testing.T) {
		ids, pks, sigs, msg, _, tag := createAggregationData(t, 2)

		// public key at index 1 is opposite of public key at index 0 (pks[1] = -pks[0])
		// so that aggregation of pks[0] and pks[1] is identity
		// this is a shortcut given no PoPs are checked in this test
		oppositePk := pks[0].Encode()
		negatePoint(oppositePk)
		var err error
		pks[1], err = crypto.DecodePublicKey(crypto.BLSBLS12381, oppositePk)
		require.NoError(t, err)

		// given how pks[1] was constructed,
		// sig[1]= -sigs[0] is a valid signature for signer with index 1
		copy(sigs[1], sigs[0])
		negatePoint(sigs[1])

		aggregator, err := NewWeightedSignatureAggregator(ids, pks, msg, tag)
		require.NoError(t, err)

		// add a valid signature for index 0
		err = aggregator.Verify(ids[0].NodeID, sigs[0])
		require.NoError(t, err)
		_, err = aggregator.TrustedAdd(ids[0].NodeID, sigs[0])
		require.NoError(t, err)

		// add a valid signature for index 1
		err = aggregator.Verify(ids[1].NodeID, sigs[1])
		require.NoError(t, err)
		_, err = aggregator.TrustedAdd(ids[1].NodeID, sigs[1])
		require.NoError(t, err)

		// Aggregation should error with sentinel model.InvalidAggregatedKeyError
		// because aggregated key is identity, although all signatures are valid
		signers, agg, err := aggregator.Aggregate()
		assert.Error(t, err)
		assert.True(t, model.IsInvalidAggregatedKeyError(err))
		assert.Nil(t, agg)
		assert.Nil(t, signers)
	})
}
