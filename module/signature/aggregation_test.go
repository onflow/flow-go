package signature

import (
	"errors"
	mrand "math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
)

func getPRG(t *testing.T) *mrand.Rand {
	random := time.Now().UnixNano()
	t.Logf("rng seed is %d", random)
	rng := mrand.New(mrand.NewSource(random))
	return rng
}

// Utility function that flips a point sign bit to negate the point
// this is shortcut which works only for zcash BLS12-381 compressed serialization
// that is currently supported by the flow crypto module
// Applicable to both signatures and public keys
func negatePoint(pointbytes []byte) {
	pointbytes[0] ^= 0x20
}

func createAggregationData(t *testing.T, rand *mrand.Rand, signersNumber int) (
	[]byte, string, []crypto.Signature, []crypto.PublicKey,
) {
	// create message and tag
	msgLen := 100
	msg := make([]byte, msgLen)
	_, err := rand.Read(msg)
	require.NoError(t, err)
	tag := "random_tag"
	hasher := NewBLSHasher(tag)

	// create keys and signatures
	keys := make([]crypto.PublicKey, 0, signersNumber)
	sigs := make([]crypto.Signature, 0, signersNumber)
	seed := make([]byte, crypto.KeyGenSeedMinLen)
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
	return msg, tag, sigs, keys
}

func TestAggregatorSameMessage(t *testing.T) {
	rand := getPRG(t)
	signersNum := 20

	// constructor edge cases
	t.Run("constructor", func(t *testing.T) {
		msg := []byte("random_msg")
		tag := "random_tag"
		// empty keys
		_, err := NewSignatureAggregatorSameMessage(msg, tag, []crypto.PublicKey{})
		assert.Error(t, err)
		// wrong key types
		seed := make([]byte, crypto.KeyGenSeedMinLen)
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
	// all signatures are valid
	t.Run("happy path", func(t *testing.T) {
		msg, tag, sigs, pks := createAggregationData(t, rand, signersNum)
		aggregator, err := NewSignatureAggregatorSameMessage(msg, tag, pks)
		require.NoError(t, err)

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
		ok, aggKey, err := aggregator.VerifyAggregate(signers, agg)
		assert.NoError(t, err)
		assert.True(t, ok)
		// check aggregated public key
		expectedKey, err := crypto.AggregateBLSPublicKeys(pks[subSet:])
		assert.NoError(t, err)
		assert.True(t, expectedKey.Equals(aggKey))
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
		ok, aggKey, err = aggregator.VerifyAggregate(signers, agg)
		assert.NoError(t, err)
		assert.True(t, ok)
		// check aggregated public key
		expectedKey, err = crypto.AggregateBLSPublicKeys(pks[:])
		assert.NoError(t, err)
		assert.True(t, expectedKey.Equals(aggKey))
		// check signers
		sort.Ints(signers)
		for i := 0; i < signersNum; i++ {
			assert.Equal(t, i, signers[i])
		}
	})

	// Unhappy paths
	t.Run("invalid inputs", func(t *testing.T) {
		msg, tag, sigs, pks := createAggregationData(t, rand, signersNum)
		aggregator, err := NewSignatureAggregatorSameMessage(msg, tag, pks)
		require.NoError(t, err)
		// invalid indices for different methods
		for _, index := range []int{-1, signersNum} {
			// loop through invalid index inputs
			ok, err := aggregator.Verify(index, sigs[0])
			assert.False(t, ok)
			assert.True(t, IsInvalidSignerIdxError(err))

			ok, err = aggregator.VerifyAndAdd(index, sigs[0])
			assert.False(t, ok)
			assert.True(t, IsInvalidSignerIdxError(err))

			err = aggregator.TrustedAdd(index, sigs[0])
			assert.True(t, IsInvalidSignerIdxError(err))

			ok, err = aggregator.HasSignature(index)
			assert.False(t, ok)
			assert.True(t, IsInvalidSignerIdxError(err))

			ok, aggKey, err := aggregator.VerifyAggregate([]int{index}, sigs[0])
			assert.False(t, ok)
			assert.Nil(t, aggKey)
			assert.True(t, IsInvalidSignerIdxError(err))
		}
		// empty list on VerifyAggregate
		ok, aggKey, err := aggregator.VerifyAggregate([]int{}, sigs[0])
		assert.False(t, ok)
		assert.Nil(t, aggKey)
		assert.True(t, IsInsufficientSignaturesError(err))
	})

	t.Run("duplicate signers", func(t *testing.T) {
		msg, tag, sigs, pks := createAggregationData(t, rand, signersNum)
		aggregator, err := NewSignatureAggregatorSameMessage(msg, tag, pks)
		require.NoError(t, err)

		// first add non-duplicate signatures
		for i, sig := range sigs {
			err := aggregator.TrustedAdd(i, sig)
			require.NoError(t, err)
		}
		// add duplicate signers which expects errors
		for i := range sigs {
			// `TrustedAdd`
			err := aggregator.TrustedAdd(i, sigs[i]) // same signature for same index
			assert.True(t, IsDuplicatedSignerIdxError(err))
			err = aggregator.TrustedAdd(i, sigs[(i+1)%signersNum]) // different signature for same index
			assert.True(t, IsDuplicatedSignerIdxError(err))
			// `VerifyAndAdd``
			ok, err := aggregator.VerifyAndAdd(i, sigs[i]) // same signature for same index
			assert.False(t, ok)
			assert.True(t, IsDuplicatedSignerIdxError(err))
			ok, err = aggregator.VerifyAndAdd(i, sigs[(i+1)%signersNum]) // different signature for same index
			assert.False(t, ok)
			assert.True(t, IsDuplicatedSignerIdxError(err))
		}
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

	// 1: No signature has been added.
	t.Run("aggregate with no signatures", func(t *testing.T) {
		msg, tag, _, pks := createAggregationData(t, rand, 1)
		aggregator, err := NewSignatureAggregatorSameMessage(msg, tag, pks)
		require.NoError(t, err)
		// Aggregation should error with sentinel InsufficientSignaturesError
		signers, agg, err := aggregator.Aggregate()
		assert.Error(t, err)
		assert.True(t, IsInsufficientSignaturesError(err))
		assert.Nil(t, agg)
		assert.Nil(t, signers)

	})

	//  2. A signature added via `TrustedAdd` has an invalid structure (fails to deserialize)
	//      2.a. aggregated public key is not identity
	//      2.b. aggregated public key is identity
	t.Run("invalid signature serialization", func(t *testing.T) {
		msg, tag, sigs, pks := createAggregationData(t, rand, 2)
		invalidStructureSig := (crypto.Signature)([]byte{0, 0})

		t.Run("with non-identity aggregated public key", func(t *testing.T) {
			aggregator, err := NewSignatureAggregatorSameMessage(msg, tag, pks)
			require.NoError(t, err)

			// add invalid signature for signer with index 0
			// sanity check : methods that check validity should reject it
			ok, err := aggregator.Verify(0, invalidStructureSig) // stand-alone verification
			require.NoError(t, err)
			assert.False(t, ok)
			ok, err = aggregator.VerifyAndAdd(0, invalidStructureSig) // verification plus addition
			require.NoError(t, err)
			assert.False(t, ok)
			// check signature is still not added
			ok, err = aggregator.HasSignature(0)
			require.NoError(t, err)
			assert.False(t, ok)

			// TrustedAdd should accept the invalid signature
			err = aggregator.TrustedAdd(0, invalidStructureSig)
			require.NoError(t, err)

			// Aggregation should error with sentinel InvalidSignatureIncludedError
			// aggregated public key is not identity (equal to pk[0])
			signers, agg, err := aggregator.Aggregate()
			assert.Error(t, err)
			assert.True(t, IsInvalidSignatureIncludedError(err))
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

			aggregator, err := NewSignatureAggregatorSameMessage(msg, tag, pks)
			require.NoError(t, err)

			// add the invalid signature on index 0
			err = aggregator.TrustedAdd(0, invalidStructureSig)
			require.NoError(t, err)

			// add a second signature for index 1
			err = aggregator.TrustedAdd(1, sigs[1])
			require.NoError(t, err)

			// Aggregation should error with sentinel InvalidSignatureIncludedError
			// aggregated public key is identity
			signers, agg, err := aggregator.Aggregate()
			assert.Error(t, err)
			assert.True(t, IsInvalidSignatureIncludedError(err))
			assert.Nil(t, agg)
			assert.Nil(t, signers)
		})
	})

	//  3. Signatures serialization is valid but some signatures are invalid w.r.t their respective public keys.
	//      3.a. aggregated public key is not identity
	//      3.b. aggregated public key is identity
	t.Run("correct serialization and invalid signature", func(t *testing.T) {
		msg, tag, sigs, pks := createAggregationData(t, rand, 2)

		t.Run("with non-identity aggregated public key", func(t *testing.T) {
			aggregator, err := NewSignatureAggregatorSameMessage(msg, tag, pks)
			require.NoError(t, err)

			// first, add a valid signature
			ok, err := aggregator.VerifyAndAdd(0, sigs[0])
			require.NoError(t, err)
			assert.True(t, ok)

			// add invalid signature for signer with index 1
			// sanity check: methods that check validity should reject it
			ok, err = aggregator.Verify(1, sigs[0]) // stand-alone verification
			require.NoError(t, err)
			assert.False(t, ok)
			ok, err = aggregator.VerifyAndAdd(1, sigs[0]) // verification plus addition
			require.NoError(t, err)
			assert.False(t, ok)
			// check signature is still not added
			ok, err = aggregator.HasSignature(1)
			require.NoError(t, err)
			assert.False(t, ok)

			// TrustedAdd should accept invalid signature
			err = aggregator.TrustedAdd(1, sigs[0])
			require.NoError(t, err)

			// Aggregation should error with sentinel InvalidSignatureIncludedError
			// aggregated public key is not identity (equal to pk[0] + pk[1])
			signers, agg, err := aggregator.Aggregate()
			assert.Error(t, err)
			assert.True(t, IsInvalidSignatureIncludedError(err))
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

			aggregator, err := NewSignatureAggregatorSameMessage(msg, tag, pks)
			require.NoError(t, err)

			// add a valid signature
			err = aggregator.TrustedAdd(0, sigs[0])
			require.NoError(t, err)

			// add an invalid signature via `TrustedAdd`
			err = aggregator.TrustedAdd(1, sigs[0])
			require.NoError(t, err)

			// Aggregation should error with sentinel ErrIdentityPublicKey
			// aggregated public key is identity
			signers, agg, err := aggregator.Aggregate()
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrIdentityPublicKey))
			assert.Nil(t, agg)
			assert.Nil(t, signers)
		})
	})

	// 4. All signatures are valid but aggregated key is identity
	t.Run("all valid signatures and identity aggregated key", func(t *testing.T) {
		msg, tag, sigs, pks := createAggregationData(t, rand, 2)

		// public key at index 1 is opposite of public key at index 0 (pks[1] = -pks[0])
		// so that aggregation of pks[0] and pks[1] is identity
		// this is a shortcut given PoPs are not checked in this test
		pkBytes := pks[0].Encode()
		negatePoint(pkBytes)
		var err error
		pks[1], err = crypto.DecodePublicKey(crypto.BLSBLS12381, pkBytes)
		require.NoError(t, err)

		// given how pks[1] was constructed,
		// sig[1]= -sigs[0] is a valid signature for signer with index 1
		copy(sigs[1], sigs[0])
		negatePoint(sigs[1])

		aggregator, err := NewSignatureAggregatorSameMessage(msg, tag, pks)
		require.NoError(t, err)

		// add a valid signature for index 0
		ok, err := aggregator.VerifyAndAdd(0, sigs[0])
		require.NoError(t, err)
		assert.True(t, ok)

		// add a valid signature for index 1
		ok, err = aggregator.VerifyAndAdd(1, sigs[1])
		require.NoError(t, err)
		assert.True(t, ok)

		// Aggregation should error with sentinel ErrIdentityPublicKey
		signers, agg, err := aggregator.Aggregate()
		assert.Error(t, err)
		assert.True(t, errors.Is(err, ErrIdentityPublicKey))
		assert.Nil(t, agg)
		assert.Nil(t, signers)
	})
}

func TestKeyAggregator(t *testing.T) {
	rand := getPRG(t)

	signersNum := 20
	// create keys
	indices := make([]int, 0, signersNum)
	keys := make([]crypto.PublicKey, 0, signersNum)
	seed := make([]byte, crypto.KeyGenSeedMinLen)
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

	// constructor edge cases
	t.Run("constructor", func(t *testing.T) {
		// wrong key types
		seed := make([]byte, crypto.KeyGenSeedMinLen)
		_, err = rand.Read(seed)
		require.NoError(t, err)
		sk, err := crypto.GeneratePrivateKey(crypto.ECDSAP256, seed)
		require.NoError(t, err)
		// wrong key type
		_, err = NewPublicKeyAggregator([]crypto.PublicKey{sk.PublicKey()})
		assert.Error(t, err)
		_, err = NewPublicKeyAggregator([]crypto.PublicKey{nil})
		assert.Error(t, err)
		// empty keys
		_, err = NewPublicKeyAggregator([]crypto.PublicKey{})
		assert.Error(t, err)
	})

	// aggregation edge cases
	t.Run("empty signers", func(t *testing.T) {
		// empty input
		_, err = aggregator.KeyAggregate(indices[:0])
		assert.Error(t, err)
		// invalid signer
		_, err = aggregator.KeyAggregate([]int{-1})
		assert.True(t, IsInvalidSignerIdxError(err))

		_, err = aggregator.KeyAggregate([]int{signersNum})
		assert.True(t, IsInvalidSignerIdxError(err))

		aggregateTwice := func(l1, h1, l2, h2 int) {
			var key, expectedKey crypto.PublicKey
			key, err = aggregator.KeyAggregate(indices[l1:h1])
			require.NoError(t, err)
			expectedKey, err = crypto.AggregateBLSPublicKeys(keys[l1:h1])
			require.NoError(t, err)
			assert.True(t, key.Equals(expectedKey))

			key, err = aggregator.KeyAggregate(indices[l2:h2])
			require.NoError(t, err)
			expectedKey, err = crypto.AggregateBLSPublicKeys(keys[l2:h2])
			require.NoError(t, err)
			assert.True(t, key.Equals(expectedKey))
		}

		// No overlaps
		aggregateTwice(1, 5, 7, 13)

		// big overlap (greedy algorithm)
		aggregateTwice(1, 9, 2, 10)

		// small overlap (aggregate from scratch)
		aggregateTwice(1, 9, 8, 11)

		// same signers
		aggregateTwice(1, 9, 1, 9)
	})

	t.Run("greedy algorithm with randomized intervals", func(t *testing.T) {
		// iterate over different random cases to make sure
		// the delta algorithm works
		rounds := 30
		for i := 0; i < rounds; i++ {
			go func() { // test module concurrency
				low := rand.Intn(signersNum - 1)
				high := low + 1 + rand.Intn(signersNum-1-low)
				var key, expectedKey crypto.PublicKey
				var err error
				key, err = aggregator.KeyAggregate(indices[low:high])

				require.NoError(t, err)
				if low == high {
					expectedKey = crypto.IdentityBLSPublicKey()
				} else {
					expectedKey, err = crypto.AggregateBLSPublicKeys(keys[low:high])
					require.NoError(t, err)
				}
				assert.True(t, key.Equals(expectedKey))
			}()
		}
	})
}
