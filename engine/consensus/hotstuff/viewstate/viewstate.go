package viewstate

import (
	"errors"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/model/flow/identity"
	"github.com/dapperlabs/flow-go/module"
	"github.com/dapperlabs/flow-go/protocol"
	"github.com/rs/zerolog"
)

type ViewState struct {
	ps  protocol.State
	me  module.Local
	log zerolog.Logger
}

// IsSelf(id types.ID) bool
// IsSelfLeaderForView(view uint64) bool
// LeaderForView(view uint64) types.ID
// GetSelfIdxForBlockID(blockID flow.Identifier) uint32
// GetIdxOfPubKeyForBlockID(blockID flow.Identifier) uint32

func (vs *ViewState) IsSelf(id flow.Identifier) bool {
	return id == vs.me.NodeID()
}

func (vs *ViewState) getIdentitiesByViewAndRole(view uint64, role flow.Role) (flow.IdentityList, error) {
	identities, err := vs.ps.AtNumber(view).Identities(identity.HasRole(role))

	return identities, err
}

func (vs *ViewState) GetSelfIdxForBlockID(blockID flow.Identifier) (uint32, error) {
	return vs.GetIdxOfPubKeyForBlockID(blockID, vs.me.NodeID())
}

func (vs *ViewState) GetIdxOfPubKeyForBlockID(blockID flow.Identifier, pubkey flow.Identifier) (uint32, error) {
	log := vs.log.With().
		Hex("block_id", blockID[:]).
		Logger()

	identities, err := vs.ps.AtBlockID(blockID).Identities(identity.HasRole(flow.RoleConsensus))
	if err != nil {
		log.Error().Err(err).Msg("couldn't get consensus identities while doing GetSelfIdxForBlockID")
		return 0, err
	}

	for i, identity := range identities {
		if identity.NodeID == pubkey {
			return uint32(i), nil
		}
	}

	err = errors.New("couldn't find self in consensus identities while doing GetSelfIdxForBlockID")
	log.Error().Err(err)
	return 0, err
}
