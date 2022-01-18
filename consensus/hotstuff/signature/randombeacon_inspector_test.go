package signature

import (
	"errors"
	mrand "math/rand"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module/signature"
)

func TestRandomBeaconInspector(t *testing.T) {
	suite.Run(t, new(randomBeaconSuite))
}

type randomBeaconSuite struct {
	suite.Suite
	n                         int
	threshold                 int
	kmac                      hash.Hasher
	signers                   []int
	thresholdSignatureMessage []byte
	skShares                  []crypto.PrivateKey
	pkShares                  []crypto.PublicKey
	pkGroup                   crypto.PublicKey
}

func (rs *randomBeaconSuite) SetupTest() {
	rs.n = 10
	rs.threshold = signature.RandomBeaconThreshold(rs.n)

	// generate threshold keys
	mrand.Seed(time.Now().UnixNano())
	seed := make([]byte, crypto.SeedMinLenDKG)
	_, err := mrand.Read(seed)
	require.NoError(rs.T(), err)
	rs.skShares, rs.pkShares, rs.pkGroup, err = crypto.BLSThresholdKeyGen(rs.n, rs.threshold, seed)
	require.NoError(rs.T(), err)

	// generate signature shares
	rs.signers = make([]int, 0, rs.n)

	// hasher
	rs.kmac = crypto.NewBLSKMAC(encoding.RandomBeaconTag)
	rs.thresholdSignatureMessage = []byte("random_message")

	// fill the signers list and shuffle it
	for i := 0; i < rs.n; i++ {
		rs.signers = append(rs.signers, i)
	}
	mrand.Shuffle(rs.n, func(i, j int) {
		rs.signers[i], rs.signers[j] = rs.signers[j], rs.signers[i]
	})
}

func (rs *randomBeaconSuite) TestHappyPath() {
	follower, err := NewRandomBeaconInspector(rs.pkGroup, rs.pkShares, rs.threshold, rs.thresholdSignatureMessage)
	require.NoError(rs.T(), err)

	// check EnoughShares
	enough := follower.EnoughShares()
	assert.False(rs.T(), enough)
	var wg sync.WaitGroup
	// create (t) signatures of the first randomly chosen signers
	// (1 signature short of the threshold)
	for j := 0; j < rs.threshold; j++ {
		wg.Add(1)
		// test thread safety
		go func(j int) {
			defer wg.Done()
			i := rs.signers[j]
			share, err := rs.skShares[i].Sign(rs.thresholdSignatureMessage, rs.kmac)
			require.NoError(rs.T(), err)
			// Verify
			err = follower.Verify(i, share)
			assert.NoError(rs.T(), err)
			// TrustedAdd
			enough, err := follower.TrustedAdd(i, share)
			assert.NoError(rs.T(), err)
			assert.False(rs.T(), enough)
			// check EnoughSignature
			assert.False(rs.T(), follower.EnoughShares(), "threshold shouldn't be reached")
		}(j)
	}
	wg.Wait()
	// add the last required signature to get (t+1) shares
	i := rs.signers[rs.threshold]
	share, err := rs.skShares[i].Sign(rs.thresholdSignatureMessage, rs.kmac)
	require.NoError(rs.T(), err)
	err = follower.Verify(i, share)
	assert.NoError(rs.T(), err)
	enough, err = follower.TrustedAdd(i, share)
	assert.NoError(rs.T(), err)
	assert.True(rs.T(), enough)
	// check EnoughSignature
	assert.True(rs.T(), follower.EnoughShares())

	// add a share when threshold is reached
	if rs.threshold+1 < rs.n {
		i := rs.signers[rs.threshold+1]
		share, err := rs.skShares[i].Sign(rs.thresholdSignatureMessage, rs.kmac)
		require.NoError(rs.T(), err)
		// Trusted Add
		enough, err := follower.TrustedAdd(i, share)
		assert.NoError(rs.T(), err)
		assert.True(rs.T(), enough)
	}
	// reconstruct the threshold signature
	thresholdsignature, err := follower.Reconstruct()
	require.NoError(rs.T(), err)
	// VerifyThresholdSignature
	verif, err := rs.pkGroup.Verify(thresholdsignature, rs.thresholdSignatureMessage, rs.kmac)
	require.NoError(rs.T(), err)
	assert.True(rs.T(), verif)
}

func (rs *randomBeaconSuite) TestDuplicateSigner() {
	follower, err := NewRandomBeaconInspector(rs.pkGroup, rs.pkShares, rs.threshold, rs.thresholdSignatureMessage)
	require.NoError(rs.T(), err)

	// Create a share and add it
	i := 0
	share, err := rs.skShares[i].Sign(rs.thresholdSignatureMessage, rs.kmac)
	require.NoError(rs.T(), err)
	enough, err := follower.TrustedAdd(i, share)
	assert.NoError(rs.T(), err)
	assert.False(rs.T(), enough)

	// Add an existing share
	// TrustedAdd
	enough, err = follower.TrustedAdd(i, share)
	assert.Error(rs.T(), err)
	assert.True(rs.T(), engine.IsDuplicatedEntryError(err))
	assert.False(rs.T(), enough)
}

func (rs *randomBeaconSuite) TestInvalidIndex() {
	follower, err := NewRandomBeaconInspector(rs.pkGroup, rs.pkShares, rs.threshold, rs.thresholdSignatureMessage)
	require.NoError(rs.T(), err)

	share, err := rs.skShares[0].Sign(rs.thresholdSignatureMessage, rs.kmac)
	require.NoError(rs.T(), err)
	// invalid index
	for _, invalidIndex := range []int{len(rs.pkShares) + 1, -1} {
		// Verify
		err = follower.Verify(invalidIndex, share)
		assert.Error(rs.T(), err)
		assert.True(rs.T(), engine.IsInvalidInputError(err))
		// TrustedAdd
		enough, err := follower.TrustedAdd(invalidIndex, share)
		assert.Error(rs.T(), err)
		assert.True(rs.T(), engine.IsInvalidInputError(err))
		assert.False(rs.T(), enough)
	}
}

func (rs *randomBeaconSuite) TestInvalidSignature() {
	follower, err := NewRandomBeaconInspector(rs.pkGroup, rs.pkShares, rs.threshold, rs.thresholdSignatureMessage)
	require.NoError(rs.T(), err)
	index := mrand.Intn(rs.n) // random signer
	share, err := rs.skShares[index].Sign(rs.thresholdSignatureMessage, rs.kmac)
	require.NoError(rs.T(), err)

	// alter signature - signature is rs.not a valid point
	share[4] ^= 1
	// Verify
	err = follower.Verify(index, share)
	assert.Error(rs.T(), err)
	assert.True(rs.T(), errors.Is(err, signature.ErrInvalidFormat))
	// restore share
	share[4] ^= 1

	// valid curve point but invalid signature
	otherIndex := (index + 1) % len(rs.pkShares) // otherIndex is different than index
	// VerifyShare
	err = follower.Verify(otherIndex, share)
	assert.Error(rs.T(), err)
	assert.True(rs.T(), errors.Is(err, signature.ErrInvalidFormat))
}

func (rs *randomBeaconSuite) TestConstructorErrors() {
	// invalid keys size
	pkSharesInvalid := make([]crypto.PublicKey, crypto.ThresholdSignMaxSize+1)
	follower, err := NewRandomBeaconInspector(rs.pkGroup, pkSharesInvalid, rs.threshold, rs.thresholdSignatureMessage)
	assert.Error(rs.T(), err)
	assert.True(rs.T(), engine.IsInvalidInputError(err))
	assert.Nil(rs.T(), follower)
	// invalid threshold
	follower, err = NewRandomBeaconInspector(rs.pkGroup, rs.pkShares, len(rs.pkShares)+1, rs.thresholdSignatureMessage)
	assert.Error(rs.T(), err)
	assert.True(rs.T(), engine.IsInvalidInputError(err))
	assert.Nil(rs.T(), follower)
}
