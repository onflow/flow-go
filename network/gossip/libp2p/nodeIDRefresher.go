package libp2p

import (
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
)

// NodeIDRefresher derives the latest list of flow Identities with which the network should be communicating based on the
// current epoch phase
type NodeIDRefresher struct {
	events.Noop
	logger   zerolog.Logger
	state    protocol.ReadOnlyState
	callBack func(flow.IdentityList) error // callBack to call when the id list has changed
}

func NewNodeIDRefresher(logger zerolog.Logger, state protocol.ReadOnlyState, callBack func(list flow.IdentityList) error) *NodeIDRefresher {
	return &NodeIDRefresher{
		logger:   logger,
		state:    state,
		callBack: callBack,
	}
}

func (listener *NodeIDRefresher) EpochSetupPhaseStarted(newEpoch uint64, first *flow.Header) {
	log := listener.logger.With().Uint64("new_epoch", newEpoch).Logger()

	// get the new set of IDs
	newIDs, err := IDsFromState(listener.state)
	if err != nil {
		log.Err(err).Msg("failed to update network ids on epoch transition")
		return
	}

	// call the registered callback
	err = listener.callBack(newIDs)
	if err != nil {
		log.Err(err).Msg("failed to update network ids on epoch transition")
		return
	}
}

// IDsFromState returns the identities that the network should be using based on the current epoch phase
func IDsFromState(state protocol.ReadOnlyState) (flow.IdentityList, error) {

	// epoch ids from this epoch
	ids, err := state.Final().Identities(filter.Any)
	if err != nil {
		return nil, fmt.Errorf("failed to get current epoch ids: %w", err)
	}

	// current epoch phase
	phase, err := state.Final().Phase()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve epoch phase: %w", err)
	}

	// if node is in epoch setup or epoch committed phase, include the next epoch identities as well
	if phase == flow.EpochPhaseSetup || phase == flow.EpochPhaseCommitted {
		nextEpochIDs, err := state.Final().Epochs().Next().InitialIdentities()
		if err != nil {
			return nil, fmt.Errorf("failed to get next epoch ids: %w", err)
		}
		ids = ids.Union(nextEpochIDs)
	}

	return ids, err
}
