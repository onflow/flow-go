package epochs

import (
	"fmt"
	"sync"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

type Staker interface {
	protocol.Consumer
	Refresh() error
	AmIStaked() bool
}

type SealedStateStaker struct {
	events.Noop
	state  protocol.State
	local  module.Local
	log    zerolog.Logger
	staked bool
	mutex  sync.RWMutex
}

func NewSealedStateStaker(log zerolog.Logger, state protocol.State, local module.Local) *SealedStateStaker {
	return &SealedStateStaker{
		state:  state,
		local:  local,
		log:    log,
		staked: false,
	}
}

// EpochTransition
func (s *SealedStateStaker) EpochTransition(newEpoch uint64, first *flow.Header) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	staked, err := protocol.IsNodeStakedAtBlockID(s.state, first.ID(), s.local.NodeID())
	if err != nil {
		log.Warn().Err(err).Msgf("cannot check is node is staked for epoch %d", newEpoch)
	}
	s.staked = staked
}

func (s *SealedStateStaker) Refresh() error {
	sealedState := s.state.Sealed()

	epochIndex, err := sealedState.Epochs().Current().Counter()
	if err != nil {
		return fmt.Errorf("cannot get current epoch counter: %w", err)
	}

	header, err := sealedState.Head()
	if err != nil {
		return fmt.Errorf("cannot get latest sealed header: %w", err)
	}

	s.EpochTransition(epochIndex, header)

	return nil
}

func (s *SealedStateStaker) AmIStaked() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.staked
}
