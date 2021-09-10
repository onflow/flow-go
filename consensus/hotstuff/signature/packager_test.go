package signature

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/signature"
	"github.com/onflow/flow-go/utils/unittest"
)

func newPacker(identities flow.IdentityList) *ConsensusSigPackerImpl {
	// mock consensus committee
	committee := &mocks.Committee{}
	committee.On("Identities", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)

	return NewConsensusSigPackerImpl(committee)
}

func makeBlockSigData(committee []flow.Identifier) *hotstuff.BlockSignatureData {
	blockSigData := &hotstuff.BlockSignatureData{
		StakingSigners: []flow.Identifier{
			committee[0], // A
			committee[2], // C
		},
		RandomBeaconSigners: []flow.Identifier{
			committee[3], // D
			committee[5], // F
		},
		AggregatedStakingSig:         unittest.SignatureFixture(),
		AggregatedRandomBeaconSig:    unittest.SignatureFixture(),
		ReconstructedRandomBeaconSig: unittest.SignatureFixture(),
	}
	return blockSigData
}

// test that a packed data can be unpacked
// given the consensus committee [A, B, C, D, E, F]
// [B,D,F] are random beacon nodes
// [A,C,E] are non-random beacon nodes
// aggregated staking sigs are from [A,C]
// aggregated random beacon sigs are from [D,F]
func TestPackUnpack(t *testing.T) {
	identities := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(identities)

	// pack & unpack
	signerIDs, sig, err := packer.Pack(blockID, blockSigData)
	require.NoError(t, err)

	unpacked, err := packer.Unpack(blockID, signerIDs, sig)
	require.NoError(t, err)

	// check that the unpack data match with the original data
	require.Equal(t, blockSigData.StakingSigners, unpacked.StakingSigners)
	require.Equal(t, blockSigData.RandomBeaconSigners, unpacked.RandomBeaconSigners)
	require.Equal(t, blockSigData.AggregatedStakingSig, unpacked.AggregatedStakingSig)
	require.Equal(t, blockSigData.AggregatedRandomBeaconSig, unpacked.AggregatedRandomBeaconSig)
	require.Equal(t, blockSigData.ReconstructedRandomBeaconSig, unpacked.ReconstructedRandomBeaconSig)

	// check the packed signer IDs
	expectedSignerIDs := []flow.Identifier{}
	expectedSignerIDs = append(expectedSignerIDs, blockSigData.StakingSigners...)
	expectedSignerIDs = append(expectedSignerIDs, blockSigData.RandomBeaconSigners...)
	require.Equal(t, expectedSignerIDs, signerIDs)
}

// if the sig data can not be decoded, return ErrInvalidFormat
func TestFailToDecode(t *testing.T) {
	identities := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(identities)

	signerIDs, sig, err := packer.Pack(blockID, blockSigData)
	require.NoError(t, err)

	// prepare invalid data by modifying the valid data
	invalidSigData := sig[1:]

	_, err = packer.Unpack(blockID, signerIDs, invalidSigData)

	require.True(t, errors.Is(err, signature.ErrInvalidFormat))
}

// if the signer IDs doesn't match, return InvalidFormatError
func TestMismatchSignerIDs(t *testing.T) {
	identities := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(identities)

	signerIDs, sig, err := packer.Pack(blockID, blockSigData)
	require.NoError(t, err)

	// prepare invalid signerIDs by modifying the valid signerIDs
	// remove the first signer
	invalidSignerIDs := signerIDs[1:]

	_, err = packer.Unpack(blockID, invalidSignerIDs, sig)

	require.True(t, errors.Is(err, signature.ErrInvalidFormat))

	// prepare invalid signerIDs by modifying the valid signerIDs
	// adding one more signer
	invalidSignerIDs = append(signerIDs, signerIDs[0])
	_, err = packer.Unpack(blockID, invalidSignerIDs, sig)

	require.True(t, errors.Is(err, signature.ErrInvalidFormat))
}

// if sig type doesn't match, return InvalidFormatError
func TestInvalidSigType(t *testing.T) {
	identities := unittest.IdentityListFixture(6, unittest.WithRole(flow.RoleConsensus))
	committee := identities.NodeIDs()

	// prepare data for testing
	blockID := unittest.IdentifierFixture()
	blockSigData := makeBlockSigData(committee)

	// create packer with the committee
	packer := newPacker(identities)

	signerIDs, sig, err := packer.Pack(blockID, blockSigData)
	require.NoError(t, err)

	var data signatureData
	err = packer.encoder.Decode(sig, &data)
	require.NoError(t, err)

	data.SigType[0] = hotstuff.SigTypeRandomBeacon + 3 // invalid type

	encoded, err := packer.encoder.Encode(data)
	require.NoError(t, err)

	_, err = packer.Unpack(blockID, signerIDs, encoded)

	require.True(t, errors.Is(err, signature.ErrInvalidFormat))
}
