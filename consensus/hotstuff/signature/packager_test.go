package signature

import (
	"testing"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
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

	// check that packed signer IDs
	expectedSignerIDs := []flow.Identifier{
		committee[0], // A
		committee[2], // C
		committee[3], // D
		committee[5], // F
	}
	require.Equal(t, expectedSignerIDs, signerIDs)
}
