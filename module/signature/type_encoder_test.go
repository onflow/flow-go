package signature_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/encoding"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestEncodeDecodeStakingSig(t *testing.T) {
	sig := unittest.SignatureFixture()
	encoded := signature.EncodeSingleSig(encoding.SigTypeStaking, sig)
	decodedType, decodedSig, err := signature.DecodeSingleSig(encoded)
	require.NoError(t, err)
	require.Equal(t, encoding.SigTypeStaking, decodedType)
	require.Equal(t, sig, decodedSig)
}

func TestEncodeDecodeRandomBeaconSig(t *testing.T) {
	sig := unittest.SignatureFixture()
	encoded := signature.EncodeSingleSig(encoding.SigTypeRandomBeacon, sig)
	decodedType, decodedSig, err := signature.DecodeSingleSig(encoded)
	require.NoError(t, err)
	require.Equal(t, encoding.SigTypeRandomBeacon, decodedType)
	require.Equal(t, sig, decodedSig)
}

// encode with invalid sig type, then decode will fail
func TestEncodeDecodeInvalidSig(t *testing.T) {
	sig := unittest.SignatureFixture()

	for i := int(encoding.SigTypeRandomBeacon) + 1; i < int(encoding.SigTypeRandomBeacon)+5; i++ {
		sigType := encoding.SigType(i)
		encoded := signature.EncodeSingleSig(sigType, sig)
		_, _, err := signature.DecodeSingleSig(encoded)
		require.Error(t, err)
	}
}

func TestDecodeEmptySig(t *testing.T) {
	_, _, err := signature.DecodeSingleSig([]byte{})
	require.Error(t, err)
}

// encode two different sigs wth the same type, the encoded sig should be different
func TestEncodeTwoSigsDifferent(t *testing.T) {
	sigs := unittest.SignaturesFixture(2)
	sig1, sig2 := sigs[0], sigs[1]
	encodedSig1 := signature.EncodeSingleSig(encoding.SigTypeStaking, sig1)
	encodedSig2 := signature.EncodeSingleSig(encoding.SigTypeStaking, sig2)
	require.NotEqual(t, encodedSig1, encodedSig2)
}

// encode the same sig with the different type, the encoded sig should be different
func TestEncodeSameSigWithDifferentTypeShouldBeDifferen(t *testing.T) {
	sig := unittest.SignatureFixture()
	encodedAsStaking := signature.EncodeSingleSig(encoding.SigTypeStaking, sig)
	encodedAsRandomBeacon := signature.EncodeSingleSig(encoding.SigTypeRandomBeacon, sig)
	require.NotEqual(t, encodedAsStaking, encodedAsRandomBeacon)
}
