package verification

import (
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/consensus/hotstuff"
	"github.com/onflow/flow-go/consensus/hotstuff/helper"
	"github.com/onflow/flow-go/consensus/hotstuff/mocks"
	hotsignature "github.com/onflow/flow-go/consensus/hotstuff/signature"
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/local"
	modulemock "github.com/onflow/flow-go/module/mock"
	"github.com/onflow/flow-go/module/signature"
	storagemock "github.com/onflow/flow-go/storage/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

const epochCounter = uint64(42)

func MakeSigners(t *testing.T,
	signerIDs []flow.Identifier,
	stakingKeys []crypto.PrivateKey,
	beaconKeys []crypto.PrivateKey, tag string) []hotstuff.Signer {

	// generate our consensus node identities
	require.NotEmpty(t, signerIDs)

	var signers []hotstuff.Signer
	if len(beaconKeys) != len(stakingKeys) {
		for i, signerID := range signerIDs {
			signer := MakeStakingSigner(t, signerID, stakingKeys[i], tag)
			signers = append(signers, signer)
		}
	} else {
		for i, signerID := range signerIDs {
			signer := MakeBeaconSigner(t, signerID, stakingKeys[i], beaconKeys[i])
			signers = append(signers, signer)
		}
	}

	return signers
}

func MakeStakingSigner(t *testing.T, signerID flow.Identifier, priv crypto.PrivateKey, tag string) *SingleSigner {
	local, err := local.New(nil, priv)
	require.NoError(t, err)
	staking := signature.NewAggregationProvider(tag, local)
	signer := NewSingleSigner(staking, signerID)
	return signer
}

func MakeBeaconSigner(t *testing.T,
	signerID flow.Identifier,
	stakingPriv crypto.PrivateKey,
	beaconPriv crypto.PrivateKey) *CombinedSigner {

	nodeID := unittest.IdentityFixture()
	nodeID.NodeID = signerID
	nodeID.StakingPubKey = stakingPriv.PublicKey()
	local, err := local.New(nodeID, stakingPriv)
	require.NoError(t, err)

	epochLookup := &modulemock.EpochLookup{}
	epochLookup.On("EpochForViewWithFallback", mock.Anything).Return(epochCounter, nil)

	dkgKey := unittest.DKGParticipantPriv()
	keys := &storagemock.DKGKeys{}
	// there is DKG key for this epoch
	keys.On("RetrieveMyDKGPrivateInfo", epochCounter).Return(dkgKey, true, nil)

	beaconKeyStore := hotsignature.NewEpochAwareRandomBeaconKeyStore(epochLookup, keys)

	signer := NewCombinedSigner(local, beaconKeyStore)

	return signer
}

func MakeHotstuffCommitteeState(t *testing.T, identities flow.IdentityList, beaconEnabled bool, epochCounter uint64) (hotstuff.Committee, []crypto.PrivateKey, []crypto.PrivateKey) {

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
		beaconSKs, beaconPKs, beaconGroupPK, err = crypto.ThresholdSignKeyGen(len(identities),
			signature.RandomBeaconThreshold(len(identities)), seed)
		require.NoError(t, err)

		dkg := &mocks.DKG{}
		committee.On("DKG", mock.Anything).Return(dkg, nil)
		dkg.On("Counter").Return(epochCounter)
		dkg.On("Size").Return(uint(len(identities)))
		dkg.On("GroupKey").Return(beaconGroupPK)
		for i, node := range identities {
			share := beaconPKs[i]
			dkg.On("KeyShare", node.NodeID).Return(share, nil)
			dkg.On("Index", node.NodeID).Return(uint(i), nil)
		}
	}

	return committee, stakingKeys, beaconSKs
}
