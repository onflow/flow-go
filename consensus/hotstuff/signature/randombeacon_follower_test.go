package signature

import (
	"errors"
	mrand "math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module/signature"
)

func TestRandomBeaconFollower(t *testing.T) {
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
	kmac := crypto.NewBLSKMAC(encoding.RandomBeaconTag)
	thresholdSignatureMessage := []byte("random_message")

	// fill the signers list and shuffle it
	for i := 0; i < n; i++ {
		signers = append(signers, i)
	}
	mrand.Shuffle(n, func(i, j int) {
		signers[i], signers[j] = signers[j], signers[i]
	})

	t.Run("happy path", func(t *testing.T) {
		follower, err := NewRandomBeaconFollower(pkGroup, pkShares, threshold, thresholdSignatureMessage)
		require.NoError(t, err)

		// check EnoughShares
		enough := follower.EnoughShares()
		assert.False(t, enough)
		var wg sync.WaitGroup
		// create (t) signatures of the first randomly chosen signers
		// (1 signature short of the threshold)
		for j := 0; j < threshold; j++ {
			wg.Add(1)
			// test thread safety
			go func(j int) {
				defer wg.Done()
				i := signers[j]
				share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
				require.NoError(t, err)
				// Verify
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
		assert.True(t, verif)
	})

	t.Run("duplicate signer", func(t *testing.T) {
		follower, err := NewRandomBeaconFollower(pkGroup, pkShares, threshold, thresholdSignatureMessage)
		require.NoError(t, err)

		// Create a share and add it
		i := 0
		share, err := skShares[i].Sign(thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		enough, err := follower.TrustedAdd(i, share)
		assert.NoError(t, err)
		assert.False(t, enough)

		// Add an existing share
		// TrustedAdd
		enough, err = follower.TrustedAdd(i, share)
		assert.Error(t, err)
		assert.True(t, engine.IsDuplicatedEntryError(err))
		assert.False(t, enough)
	})

	t.Run("Invalid index", func(t *testing.T) {
		follower, err := NewRandomBeaconFollower(pkGroup, pkShares, threshold, thresholdSignatureMessage)
		require.NoError(t, err)

		share, err := skShares[0].Sign(thresholdSignatureMessage, kmac)
		require.NoError(t, err)
		// invalid index
		invalidIndex := len(pkShares) + 1
		// Verify
		err = follower.Verify(invalidIndex, share)
		assert.Error(t, err)
		assert.True(t, engine.IsInvalidInputError(err))
		// TrustedAdd
		enough, err := follower.TrustedAdd(invalidIndex, share)
		assert.Error(t, err)
		assert.True(t, engine.IsInvalidInputError(err))
		assert.False(t, enough)
	})

	t.Run("invalid signature", func(t *testing.T) {
		follower, err := NewRandomBeaconFollower(pkGroup, pkShares, threshold, thresholdSignatureMessage)
		require.NoError(t, err)
		index := mrand.Intn(n) // random signer
		share, err := skShares[index].Sign(thresholdSignatureMessage, kmac)
		require.NoError(t, err)

		// alter signature - signature is not a valid point
		share[4] ^= 1
		// Verify
		err = follower.Verify(index, share)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, signature.ErrInvalidFormat))
		// restore share
		share[4] ^= 1

		// valid curve point but invalid signature
		otherIndex := (index + 1) % len(pkShares) // otherIndex is different than index
		// VerifyShare
		err = follower.Verify(otherIndex, share)
		assert.Error(t, err)
		assert.True(t, errors.Is(err, signature.ErrInvalidFormat))
	})

	t.Run("constructor errors", func(t *testing.T) {
		// invalid keys size
		pkSharesInvalid := make([]crypto.PublicKey, crypto.ThresholdSignMaxSize+1)
		follower, err := NewRandomBeaconFollower(pkGroup, pkSharesInvalid, threshold, thresholdSignatureMessage)
		assert.Error(t, err)
		assert.True(t, engine.IsInvalidInputError(err))
		assert.Nil(t, follower)
		// invalid threshold
		follower, err = NewRandomBeaconFollower(pkGroup, pkShares, len(pkShares)+1, thresholdSignatureMessage)
		assert.Error(t, err)
		assert.True(t, engine.IsInvalidInputError(err))
		assert.Nil(t, follower)
	})
}
