package verification

import (
	"testing"
	"crypto/rand"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/mocks"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/local"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/state/dkg"
	dkgmock "github.com/dapperlabs/flow-go/state/dkg/mocks"
)

func MakeSigners(t *testing.T, committee hotstuff.Committee, dkg dkg.State, signerIDs []flow.Identifier, stakingKeys []crypto.PrivateKey, beaconKeys []crypto.PrivateKey) []hotstuff.Signer {

	// generate our consensus node identities
	require.NotEmpty(t, signerIDs)

	var signers []hotstuff.Signer
	if len(beaconKeys) != len(stakingKeys) {
		for i, signerID := range signerIDs {
			signer := MakeStakingSigner(t, committee, signerID, stakingKeys[i])
			signers = append(signers, signer)
		}
	} else {
		for i, signerID := range signerIDs {
			signer := MakeBeaconSigner(t, committee, dkg, signerID, stakingKeys[i], beaconKeys[i])
			signers = append(signers, signer)
		}
	}

	return signers
}

func MakeStakingSigner(t *testing.T, committee hotstuff.Committee, signerID flow.Identifier, priv crypto.PrivateKey) *SingleSigner {
	local, err := local.New(nil, priv)
	require.NoError(t, err)
	staking := signature.NewAggregationProvider("test_staking", local)
	signer := NewSingleSigner(committee, staking, signerID)
	return signer
}

func MakeBeaconSigner(t *testing.T, committee hotstuff.Committee, dkg dkg.State, signerID flow.Identifier, stakingPriv crypto.PrivateKey, beaconPriv crypto.PrivateKey) *CombinedSigner {
	local, err := local.New(nil, stakingPriv)
	require.NoError(t, err)
	staking := signature.NewAggregationProvider("test_staking", local)
	beacon := signature.NewThresholdProvider("test_beacon", beaconPriv)
	combiner := signature.NewCombiner()
	signer := NewCombinedSigner(committee, dkg, staking, beacon, combiner, signerID)
	return signer
}

func MakeHotstuffCommitteeState(t *testing.T, identities flow.IdentityList, beaconEnabled bool) (hotstuff.Committee, dkg.State, []crypto.PrivateKey, []crypto.PrivateKey) {

	// initialize the dkg snapshot
	dkg := &dkgmock.State{}

	// program the MembersSnapshot
	committee := &mocks.Committee{}
	committee.On("Identities", mock.Anything, mock.Anything).Return(
		func(blockID flow.Identifier, selector flow.IdentityFilter) flow.IdentityList {
			return identities.Filter(selector)
		},
		nil,
	)
	for _, identity := range identities {
		committee.On("Identity", mock.Anything, identity.NodeID).Return(identity, nil)
	}

	// generate the staking keys
	var stakingKeys []crypto.PrivateKey
	for i := 0; i < len(identities); i++ {
		stakingKey := helper.MakeBLSKey(t)
		identities[i].StakingPubKey = stakingKey.PublicKey()
		stakingKeys = append(stakingKeys, stakingKey)
	}

	// generate the dkg keys (only if beacon is enabled)
	var beaconSKs []crypto.PrivateKey
	var beaconPKs []crypto.PublicKey
	var beaconGroupPK crypto.PublicKey
	if beaconEnabled {
		seed := make([]byte, crypto.SeedMinLenDKG)
		n, err := rand.Read(seed)
		require.NoError(t, err)
		require.Equal(t, n, crypto.SeedMinLenDKG)
		beaconSKs, beaconPKs, beaconGroupPK, err = crypto.ThresholdSignKeyGen(len(identities), seed)
		require.NoError(t, err)
		dkg.On("GroupSize").Return(uint(len(beaconSKs)), nil)
		dkg.On("GroupKey").Return(beaconGroupPK, nil)
		for i, identity := range identities {
			dkg.On("HasParticipant", identity.NodeID).Return(true, nil)
			dkg.On("ParticipantIndex", identity.NodeID).Return(uint(i), nil)
			dkg.On("ParticipantKey", identity.NodeID).Return(beaconPKs[i], nil)
		}
	}
	return committee, dkg, stakingKeys, beaconSKs
}
