package signature

import (
	mrand "math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/module/signature"
)

func testCentralizedStatefulAPI(t *testing.T) {
	n := 10
	threshold := signature.RandomBeaconThreshold(n)

	// generate threshold keys
	mrand.Seed(time.Now().UnixNano())
	seed := make([]byte, crypto.SeedMinLenDKG)
	_, err := mrand.Read(seed)
	require.NoError(t, err)
	skShares, pkShares, pkGroup, err := crypto.BLSThresholdKeyGen(n, threshold, seed)
	require.NoError(t, err)

	// generate signature shares
	signers := make([]int, 0, n)

	// hasher
	kmac := crypto.NewBLSKMAC("random tag")
	thresholdSignatureMessage := []byte("random_message")

	// fill the signers list and shuffle it
	for i := 0; i < n; i++ {
		signers = append(signers, i)
	}
	mrand.Shuffle(n, func(i, j int) {
		signers[i], signers[j] = signers[j], signers[i]
	})

	t.Run("happy path", func(t *testing.T) {
		// create the stateful threshold signer
		follower, err := NewRandomBeaconFollower(pkGroup, pkShares, threshold, thresholdSignatureMessage)
		require.NoError(t, err)

		// check EnoughShares
		enough := follower.EnoughShares()
		assert.False(t, enough)
		var wg sync.WaitGroup
		// create (t) signatures of the first randomly chosen signers
		// ( 1 signature short of the threshold)
		for j := 0; j < threshold; j++ {
			wg.Add(1)
			// test thread safety
			go func(j int) {
				defer wg.Done()
				i := signers[j]
				share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
				require.NoError(t, err)
				// VerifyShare
				err = follower.Verify(i, share)
				assert.NoError(t, err)
				// TrustedAdd
				enough, err := follower.TrustedAdd(i, share)
				assert.NoError(t, err)
				assert.False(t, enough)
				// check EnoughSignature
				assert.False(t, follower.EnoughShares(), "threshold shouldn't be reached")
			}(j)
		}
		wg.Wait()
		// add the last required signature to get (t+1) shares
		i := signers[threshold]
		share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		err = follower.Verify(i, share)
		assert.NoError(t, err)
		enough, err = follower.TrustedAdd(i, share)
		assert.NoError(t, err)
		assert.True(t, enough)
		// check EnoughSignature
		assert.True(t, follower.EnoughShares())

		// add a share when threshold is reached
		if threshold+1 < n {
			i := signers[threshold+1]
			share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
			require.NoError(t, err)
			// Trusted Add
			enough, err := follower.TrustedAdd(i, share)
			assert.NoError(t, err)
			assert.True(t, enough)
		}
		// reconstruct the threshold signature
		thresholdsignature, err := follower.Reconstruct()
		require.NoError(t, err)
		// VerifyThresholdSignature
		verif, err := pkGroup.Verify(thresholdsignature, thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		assert.False(t, verif)
	})

	/*t.Run("duplicate signer", func(t *testing.T) {
		// create the stateful threshold signer
		ts, err := NewBLSThresholdSignatureFollower(pkGroup, pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
		require.NoError(t, err)

		// Create a share and add it
		i := 0
		share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		enough, err := ts.TrustedAdd(i, share)
		assert.NoError(t, err)
		assert.False(t, enough)

		// Add an existing share

		// VerifyAndAdd
		verif, enough, err := ts.VerifyAndAdd(i, share)
		assert.Error(t, err)
		assert.False(t, IsInvalidInputsError(err))
		assert.False(t, verif)
		assert.False(t, enough)
		// TrustedAdd
		enough, err = ts.TrustedAdd(i, share)
		assert.Error(t, err)
		assert.False(t, IsInvalidInputsError(err))
		assert.False(t, enough)
	})

	t.Run("Invalid index", func(t *testing.T) {
		// create the stateful threshold signer
		ts, err := NewBLSThresholdSignatureFollower(pkGroup, pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
		require.NoError(t, err)

		share, err := skShares[0].Sign(thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		// invalid index
		invalidIndex := len(pkShares) + 1
		// VerifShare
		verif, err := ts.VerifyShare(invalidIndex, share)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, verif)
		// TrustedAdd
		enough, err := ts.TrustedAdd(invalidIndex, share)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, enough)
		// VerifyAndAdd
		verif, enough, err = ts.VerifyAndAdd(invalidIndex, share)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, verif)
		assert.False(t, enough)
		// HasShare
		verif, err = ts.HasShare(invalidIndex)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.False(t, verif)
	})

	t.Run("invalid signature", func(t *testing.T) {
		index := mrand.Intn(n)
		ts, err := NewBLSThresholdSignatureFollower(pkGroup, pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
		require.NoError(t, err)
		share, err := skShares[index].Sign(thresholdSignatureMessage, kmac)
		require.NoError(t, err)

		// alter signature - signature is not a valid point
		share[4] ^= 1
		// VerifShare
		verif, err := ts.VerifyShare(index, share)
		assert.NoError(t, err)
		assert.False(t, verif)
		// VerifyAndAdd
		verif, enough, err := ts.VerifyAndAdd(index, share)
		assert.NoError(t, err)
		assert.False(t, verif)
		assert.False(t, enough)
		// check share was not added
		verif, err = ts.HasShare(index)
		assert.NoError(t, err)
		assert.False(t, verif)
		// restore share
		share[4] ^= 1

		// valid curve point but invalid signature
		otherIndex := (index + 1) % len(pkShares) // otherIndex is different than index
		// VerifShare
		verif, err = ts.VerifyShare(otherIndex, share)
		assert.NoError(t, err)
		assert.False(t, verif)
		// VerifyAndAdd
		verif, enough, err = ts.VerifyAndAdd(otherIndex, share)
		assert.NoError(t, err)
		assert.False(t, verif)
		assert.False(t, enough)
		// check share was not added
		verif, err = ts.HasShare(otherIndex)
		assert.NoError(t, err)
		assert.False(t, verif)
	})

	t.Run("constructor errors", func(t *testing.T) {
		// invalid keys size
		index := mrand.Intn(n)
		pkSharesInvalid := make([]PublicKey, ThresholdSignMaxSize+1)
		tsFollower, err := NewBLSThresholdSignatureFollower(pkGroup, pkSharesInvalid, threshold, thresholdSignatureMessage, thresholdSignatureTag)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, tsFollower)
		// non BLS key share
		seed := make([]byte, KeyGenSeedMinLenECDSAP256)
		_, err = rand.Read(seed)
		require.NoError(t, err)
		skEcdsa, err := GeneratePrivateKey(ECDSAP256, seed)
		require.NoError(t, err)
		tmp := pkShares[0]
		pkShares[0] = skEcdsa.PublicKey()
		tsFollower, err = NewBLSThresholdSignatureFollower(pkGroup, pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, tsFollower)
		pkShares[0] = tmp // restore valid keys
		// non BLS group key
		tsFollower, err = NewBLSThresholdSignatureFollower(skEcdsa.PublicKey(), pkShares, threshold, thresholdSignatureMessage, thresholdSignatureTag)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, tsFollower)
		// non BLS private key
		tsParticipant, err := NewBLSThresholdSignatureParticipant(pkGroup, pkShares, threshold, index, skEcdsa, thresholdSignatureMessage, thresholdSignatureTag)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, tsParticipant)
		// invalid current index
		tsParticipant, err = NewBLSThresholdSignatureParticipant(pkGroup, pkShares, threshold, len(pkShares)+1, skShares[index], thresholdSignatureMessage, thresholdSignatureTag)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, tsParticipant)
		// invalid threshold
		tsFollower, err = NewBLSThresholdSignatureFollower(pkGroup, pkShares, len(pkShares)+1, thresholdSignatureMessage, thresholdSignatureTag)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, tsFollower)
		// inconsistent private and public key
		indexSwap := (index + 1) % len(pkShares) // indexSwap is different than index
		pkShares[index], pkShares[indexSwap] = pkShares[indexSwap], pkShares[index]
		tsParticipant, err = NewBLSThresholdSignatureParticipant(pkGroup, pkShares, len(pkShares)+1, index, skShares[index], thresholdSignatureMessage, thresholdSignatureTag)
		assert.Error(t, err)
		assert.True(t, IsInvalidInputsError(err))
		assert.Nil(t, tsParticipant)
		pkShares[index], pkShares[indexSwap] = pkShares[indexSwap], pkShares[index] // restore keys
	})*/
}
