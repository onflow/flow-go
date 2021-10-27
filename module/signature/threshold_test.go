// +build relic

package signature

import (
	"crypto/rand"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/module/local"
)

const NUM_THRES_TEST = 4
const NUM_THRES_BENCH = 254

// createThresholdsT creates a set of valid Threshold signature keys.
func createThresholdsT(t *testing.T, n uint) ([]*ThresholdProvider, crypto.PublicKey) {
	seed := make([]byte, crypto.SeedMinLenDKG)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	beaconKeys, _, groupKey, err := crypto.ThresholdSignKeyGen(int(n),
		RandomBeaconThreshold(int(n)), seed)
	require.NoError(t, err)
	signers := make([]*ThresholdProvider, 0, int(n))
	for i := 0; i < int(n); i++ {
		thres := NewThresholdProvider("test_beacon", beaconKeys[i])
		signers = append(signers, thres)
	}
	return signers, groupKey
}

func TestThresholdSignVerify(t *testing.T) {
	for n := uint(2); n < NUM_THRES_TEST; n++ {
		// create signer and message to be signed
		signers, _ := createThresholdsT(t, n)
		signer := signers[0]
		altSigner := signers[1]
		msg := randomByteSliceT(t, 128)

		// generate the signature
		sig, err := signer.Sign(msg)
		require.NoError(t, err)

		// signature should be valid for the original signer
		valid, err := signer.Verify(msg, sig, signer.priv.PublicKey())
		require.NoError(t, err)
		assert.True(t, valid, "signature should be valid for original signer")

		// signature should not be valid for another signer
		valid, err = signer.Verify(msg, sig, altSigner.priv.PublicKey())
		require.NoError(t, err)
		assert.False(t, valid, "signature should be invalid for other signer")

		// signature should not be valid if we change one byte
		sig[0]++
		valid, err = signer.Verify(msg, sig, signer.priv.PublicKey())
		require.NoError(t, err)
		assert.False(t, valid, "signature should be invalid with one byte changed")
		sig[0]--
	}
}

func TestThresholdCombineVerifyThreshold(t *testing.T) {
	for n := uint(2); n < NUM_THRES_TEST; n++ {
		// create signers and message to be signed
		signers, groupKey := createThresholdsT(t, n)
		_, altGroupKey := createThresholdsT(t, n)
		msg := randomByteSliceT(t, 128)

		// create a signature share for each signer and store index
		shares := make([]crypto.Signature, 0, len(signers))
		indices := make([]uint, 0, len(signers))
		for index, signer := range signers {
			share, err := signer.Sign(msg)
			require.NoError(t, err)
			shares = append(shares, share)
			indices = append(indices, uint(index))
		}

		// should generate valid signature with sufficient signers
		var err error
		sufficient := 0
		enoughShares := false
		require.NoError(t, err)
		for !enoughShares {
			// just count up sufficient until we have enough shares
			sufficient++
			enoughShares, err = crypto.EnoughShares(RandomBeaconThreshold(len(signers)), sufficient)
			require.NoError(t, err)
		}
		threshold, err := signers[0].Reconstruct(uint(len(signers)), shares[:sufficient], indices[:sufficient])
		require.NoError(t, err, "should be able to create threshold signature with sufficient shares")

		// should not generate valid signature with insufficient signers
		insufficient := len(shares) + 1
		for enoughShares {
			// just count down insufficient until it's no longer enough shares
			insufficient--
			enoughShares, err = crypto.EnoughShares(RandomBeaconThreshold(len(signers)), insufficient)
			require.NoError(t, err)
		}
		_, err = signers[0].Reconstruct(uint(len(signers)), shares[:insufficient], indices[:insufficient])
		require.Error(t, err, "should not be able to create threshold signature with insufficient shares")

		// should not be able to generate signature with missing indices
		_, err = signers[0].Reconstruct(uint(len(signers)), shares, indices[:len(indices)-1])
		require.Error(t, err, "should not be able to create threshold signature with missing indices")

		// threshold signature should be valid for the group public key
		valid, err := signers[0].VerifyThreshold(msg, threshold, groupKey)
		require.NoError(t, err)
		assert.True(t, valid, "threshold signature should be valid for group key")

		// changing one byte to the signature should invalidate it
		threshold[0]++
		valid, err = signers[0].VerifyThreshold(msg, threshold, groupKey)
		require.NoError(t, err)
		assert.False(t, valid, "threshold signature should not be valid if changed")
		threshold[0]--

		// changing one byte to the message should invalidate the signature
		msg[0]++
		valid, err = signers[0].VerifyThreshold(msg, threshold, groupKey)
		require.NoError(t, err)
		assert.False(t, valid, "threshold signature should not be valid for other message")
		msg[0]--

		// signature should not be valid for another group key
		valid, err = signers[0].VerifyThreshold(msg, threshold, altGroupKey)
		require.NoError(t, err)
		assert.False(t, valid, "threshold signature should not be valid for other group key")

		// should not generate valid signature with swapped indices
		indices[0], indices[1] = indices[1], indices[0]
		threshold, err = signers[0].Reconstruct(n, shares, indices)
		require.NoError(t, err)
		valid, err = signers[0].VerifyThreshold(msg, threshold, groupKey)
		require.NoError(t, err)
		assert.False(t, valid, "threshold signature should not be valid with swapped indices")
	}
}

// createThresholdsB creates a set of fake Threshold signature keys for benchmarking.
// The performance of the threshold signature reconstruction is the same with valid or
// randomly generated keys. Generating fake keys avoids running an expensive key generation
// especially when the total number of participants is high.
func createThresholdsB(b *testing.B, n uint) []*ThresholdProvider {
	signers := make([]*ThresholdProvider, 0, int(n))
	for i := 0; i < int(n); i++ {
		_, priv := createAggregationB(b)
		thres := NewThresholdProvider("test_beacon", priv)
		signers = append(signers, thres)
	}
	return signers
}

func BenchmarkThresholdReconstruction(b *testing.B) {

	// stop timer and reset to zero
	b.StopTimer()
	b.ResetTimer()

	// generate the desired fake threshold signature participants and create signatures
	msg := randomByteSliceB(b)
	signers := createThresholdsB(b, NUM_THRES_BENCH)
	sigs := make([]crypto.Signature, 0, len(signers))
	indices := make([]uint, 0, len(signers))
	for index, signer := range signers {
		sig, err := signer.Sign(msg)
		if err != nil {
			b.Fatal(err)
		}
		sigs = append(sigs, sig)
		indices = append(indices, uint(index))
	}

	// start the timer and run the benchmark on threshold signatures
	b.StartTimer()
	for n := 0; n < b.N; n++ {
		_, err := signers[0].Reconstruct(uint(len(signers)), sigs, indices)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func createAggregationB(b *testing.B) (*AggregationProvider, crypto.PrivateKey) {
	agg, priv, err := createAggregation()
	if err != nil {
		b.Fatal(err)
	}
	return agg, priv
}

func createAggregation() (*AggregationProvider, crypto.PrivateKey, error) {
	seed := make([]byte, crypto.KeyGenSeedMinLenBLSBLS12381)
	n, err := rand.Read(seed)
	if err != nil {
		return nil, nil, err
	}
	if n < len(seed) {
		return nil, nil, fmt.Errorf("insufficient random bytes")
	}
	priv, err := crypto.GeneratePrivateKey(crypto.BLSBLS12381, seed)
	if err != nil {
		return nil, nil, err
	}
	local, err := local.New(nil, priv)
	if err != nil {
		return nil, nil, err
	}
	return NewAggregationProvider("test_staking", local), priv, nil
}
