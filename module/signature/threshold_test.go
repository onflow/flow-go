package signature

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

const NUM_Threshold_TEST = 3
const NUM_Threshold_BENCH = 254

// createThresholdsB creates a set of Threshold keys for benchmarking; as we might generate
// many of them, we don't run a real Threshold here, but instead use randomly generated
// keys. The performance of the algorithm remains the same, even if the resulting
// signature will not be valid for any group key we know.
func createThresholdsB(b *testing.B, n uint) []*ThresholdProvider {
	signers := make([]*ThresholdProvider, 0, int(n))
	for i := 0; i < int(n); i++ {
		bls := createAggregationB(b)
		signer := NewThresholdProvider("only_testing", bls.priv)
		signers = append(signers, signer)
	}
	return signers
}

// createKDGsT creates a set of Thresholds with real key shares and a real group key.
func createThresholdsT(t *testing.T, n uint) ([]*ThresholdProvider, crypto.PublicKey) {
	beaconKeys, groupKey, _ := unittest.RunDKG(t, int(n))
	signers := make([]*ThresholdProvider, 0, int(n))
	for i := 0; i < int(n); i++ {
		signer := NewThresholdProvider("only_testing", beaconKeys[i])
		signers = append(signers, signer)
	}
	return signers, groupKey
}

func TestThresholdSignVerify(t *testing.T) {

	// create signer and message to be signed
	signers, _ := createThresholdsT(t, 3)
	signer := signers[0]
	altSigner := signers[1]
	msg := createMSGT(t)

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

func TestThresholdCombineVerifyThreshold(t *testing.T) {

	// create signers and message to be signed
	signers, groupKey := createThresholdsT(t, NUM_Threshold_TEST)
	_, altGroupKey := createThresholdsT(t, NUM_Threshold_TEST)
	msg := createMSGT(t)

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
	var sufficient int
	for sufficient = 0; !crypto.EnoughShares(len(signers), sufficient); sufficient++ {
		// just count up sufficient until we have enough shares
	}
	threshold, err := signers[0].Combine(uint(len(signers)), shares[:sufficient], indices[:sufficient])
	require.NoError(t, err, "should be able to create threshold signature with sufficient shares")

	// should not generate valid signature with insufficient signers
	var insufficient int
	for insufficient = len(shares); crypto.EnoughShares(len(signers), insufficient); insufficient-- {
		// just count down insufficient until it's no longer enough shares
	}
	_, err = signers[0].Combine(uint(len(signers)), shares[:insufficient], indices[:insufficient])
	require.Error(t, err, "should not be able to create threshold signature with insufficient shares")

	// should not be able to generate signature with missing indices
	_, err = signers[0].Combine(uint(len(signers)), shares, indices[:len(indices)-1])
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
	threshold, err = signers[0].Combine(NUM_Threshold_TEST, shares, indices)
	require.NoError(t, err)
	valid, err = signers[0].VerifyThreshold(msg, threshold, groupKey)
	require.NoError(t, err)
	assert.False(t, valid, "threshold signature should not be valid with swapped indices")
}

func BenchmarkThresholdCombination(b *testing.B) {

	// stop timer and reset to zero
	b.StopTimer()
	b.ResetTimer()

	// generate the desired fake Threshold participants and create signatures
	msg := createMSGB(b)
	signers := createThresholdsB(b, NUM_Threshold_BENCH)
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
		_, err := signers[0].Combine(uint(len(signers)), sigs, indices)
		if err != nil {
			b.Fatal(err)
		}
	}
}
