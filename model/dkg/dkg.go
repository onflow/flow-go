package dkg

import (
	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/encodable"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// DKGData represents all the output data from the DKG process, including private information.
// It is used while running the DKG during bootstrapping.
type DKGData struct {
	PrivKeyShares []crypto.PrivateKey
	PubGroupKey   crypto.PublicKey
	PubKeyShares  []crypto.PublicKey
}

// bootstrap.DKGParticipantPriv is the canonical structure for encoding private node DKG information.
type DKGParticipantPriv struct {
	NodeID              flow.Identifier
	RandomBeaconPrivKey encodable.RandomBeaconPrivKey
	GroupIndex          int
}

func ToDKGLookup(dkg DKGData, identities flow.IdentityList) map[flow.Identifier]flow.DKGParticipant {

	lookup := make(map[flow.Identifier]flow.DKGParticipant)
	participants := identities.Filter(filter.HasRole(flow.RoleConsensus))
	for i, keyShare := range dkg.PubKeyShares {
		identity := participants[i]
		lookup[identity.NodeID] = flow.DKGParticipant{
			Index:    uint(i),
			KeyShare: keyShare,
		}
	}

	return lookup
}
