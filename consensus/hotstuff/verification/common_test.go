package verification

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/consensus/hotstuff"
	"github.com/dapperlabs/flow-go/consensus/hotstuff/helper"
	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/module/signature"
	"github.com/dapperlabs/flow-go/state/protocol"
	protomock "github.com/dapperlabs/flow-go/state/protocol/mock"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func MakeSigners(t *testing.T, state protocol.State, signerIDs []flow.Identifier, stakingKeys []crypto.PrivateKey, beaconKeys []crypto.PrivateKey) []hotstuff.Signer {

	// generate our consensus node identities
	require.NotEmpty(t, signerIDs)

	var signers []hotstuff.Signer
	if len(beaconKeys) != len(stakingKeys) {
		for i, signerID := range signerIDs {
			signer := MakeStakingSigner(state, signerID, stakingKeys[i])
			signers = append(signers, signer)
		}
	} else {
		for i, signerID := range signerIDs {
			signer := MakeBeaconSigner(state, signerID, stakingKeys[i], beaconKeys[i])
			signers = append(signers, signer)
		}
	}

	return signers
}

func MakeStakingSigner(state protocol.State, signerID flow.Identifier, priv crypto.PrivateKey) *SingleSigner {
	hasher := crypto.NewBLS_KMAC("only_testing")
	staking := signature.NewBLS(hasher, priv)
	signer := NewSingleSigner(state, signerID, staking)
	return signer
}

func MakeBeaconSigner(state protocol.State, signerID flow.Identifier, stakingPriv crypto.PrivateKey, beaconPriv crypto.PrivateKey) *CombinedSigner {
	hasher := crypto.NewBLS_KMAC("only_testing")
	staking := signature.NewBLS(hasher, stakingPriv)
	beacon := signature.NewDKG(hasher, beaconPriv)
	combiner := signature.NewCombiner()
	signer := NewCombinedSigner(state, signerID, staking, beacon, combiner, combiner)
	return signer
}

func MakeProtocolState(t *testing.T, identities flow.IdentityList, beaconEnabled bool) (protocol.State, []crypto.PrivateKey, []crypto.PrivateKey) {

	// program the state snapshot
	snapshot := &protomock.Snapshot{}
	snapshot.On("Identities", mock.Anything).Return(func(filters ...flow.IdentityFilter) flow.IdentityList {
		return identities.Filter(filters...)
	}, nil)
	for _, identity := range identities {
		snapshot.On("Identity", identity.NodeID).Return(identity, nil)
	}

	// generate the staking keys
	var stakingKeys []crypto.PrivateKey
	for i := 0; i < len(identities); i++ {
		stakingKey := helper.MakeBLSKey(t)
		identities[i].StakingPubKey = stakingKey.PublicKey()
		stakingKeys = append(stakingKeys, stakingKey)
	}

	// generate the dkg keys (only if becon is enabled
	var beaconKeys []crypto.PrivateKey
	var dkgKey crypto.PublicKey
	if beaconEnabled {
		beaconKeys, dkgKey, _ = unittest.RunDKGKeys(t, len(identities))
		snapshot.On("DKGKey").Return(dkgKey, nil)
		for i, identity := range identities {
			snapshot.On("DKGIndex", identity.NodeID).Return(uint(i), nil)
			snapshot.On("DKGShare", identity.NodeID).Return(beaconKeys[i].PublicKey(), nil)
		}
	}

	// program the protocol state
	state := &protomock.State{}
	state.On("AtBlockID", mock.Anything).Return(snapshot)
	state.On("Final").Return(snapshot)

	return state, stakingKeys, beaconKeys
}
