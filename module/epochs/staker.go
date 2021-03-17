package epochs

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/logging"
)

type Staker interface {
	AmIStakedAt(blockID flow.Identifier) bool
}

type SealedStateStaker struct {
	state protocol.State
	local module.Local
	log   zerolog.Logger
}

func NewSealedStateStaker(log zerolog.Logger, state protocol.State, local module.Local) *SealedStateStaker {
	return &SealedStateStaker{
		state: state,
		local: local,
		log:   log,
	}
}

func (s *SealedStateStaker) AmIStakedAt(blockID flow.Identifier) bool {

	identity, err := s.state.AtBlockID(blockID).Identity(s.local.NodeID())
	if err != nil {
		s.log.Warn().Err(err).Hex("block_id", logging.ID(blockID)).Msgf("could not retrieve my own identity while checking for being staked")
		return false
	}

	staked := identity.Stake > 0

	return staked
}
