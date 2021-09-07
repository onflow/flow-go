package signature

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEncodeDecodeStakingSig(t *testing.T) {
	sig := unittest.SignatureFixture()
	encoded := EncodeSingleSig(hotstuff.SigTypeStaking, sig)
	decodedType, decodedSig, err := DecodeSingleSig(encoded)
	require.NoError(t, err)
	require.Equal(t, hotstuff.SigTypeStaking, decodedType)
	require.Equal(t, sig, decodedSig)
}

func TestEncodeDecodeRandomBeaconSig(t *testing.T) {
	sig := unittest.SignatureFixture()
	encoded := EncodeSingleSig(hotstuff.SigTypeRandomBeacon, sig)
	decodedType, decodedSig, err := DecodeSingleSig(encoded)
	require.NoError(t, err)
	require.Equal(t, hotstuff.SigTypeRandomBeacon, decodedType)
	require.Equal(t, sig, decodedSig)
}

// encode with invalid sig type, then decode will fail
func TestEncodeDecodeInvalidSig(t *testing.T) {
	sig := unittest.SignatureFixture()

	for i := int(hotstuff.SigTypeRandomBeacon) + 1; i < int(hotstuff.SigTypeRandomBeacon)+5; i++ {
		sigType := hotstuff.SigType(i)
		encoded := EncodeSingleSig(sigType, sig)
		_, _, err := DecodeSingleSig(encoded)
		require.Error(t, err)
	}
}

func TestDecodeEmptySig(t *testing.T) {
	_, _, err := DecodeSingleSig([]byte{})
	require.Error(t, err)
}

// encode two different sigs wth the same type, the encoded sig should be different
func TestEncodeTwoSigsDifferent(t *testing.T) {
	sigs := unittest.SignaturesFixture(2)
	sig1, sig2 := sigs[0], sigs[1]
	encodedSig1 := EncodeSingleSig(hotstuff.SigTypeStaking, sig1)
	encodedSig2 := EncodeSingleSig(hotstuff.SigTypeStaking, sig2)
	require.NotEqual(t, encodedSig1, encodedSig2)
}

// encode the same sig with the different type, the encoded sig should be different
func TestEncodeSameSigWithDifferentTypeShouldBeDifferen(t *testing.T) {
	sig := unittest.SignatureFixture()
	encodedAsStaking := EncodeSingleSig(hotstuff.SigTypeStaking, sig)
	encodedAsRandomBeacon := EncodeSingleSig(hotstuff.SigTypeRandomBeacon, sig)
	require.NotEqual(t, encodedAsStaking, encodedAsRandomBeacon)
}
