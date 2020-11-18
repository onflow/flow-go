package libp2p

import (
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
	"github.com/onflow/flow-go/state/protocol"
)

// NodeIDRefresher derives the latest list of flow identities with which the
// network should be communicating based on identity table changes in the
// protocol state.
type NodeIDRefresher struct {
	logger   zerolog.Logger
	state    protocol.ReadOnlyState
	callBack func(flow.IdentityList) error // callBack to call when the id list has changed
}

func NewNodeIDRefresher(logger zerolog.Logger, state protocol.ReadOnlyState, callBack func(list flow.IdentityList) error) *NodeIDRefresher {
	return &NodeIDRefresher{
		logger:   logger.With().Str("component", "network-refresher").Logger(),
		state:    state,
		callBack: callBack,
	}
}

func (listener *NodeIDRefresher) getLogger() zerolog.Logger {

	log := listener.logger

	// retrieve some contextual information for logging
	final := listener.state.Final()
	head, err := final.Head()
	if err != nil {
		log.Error().Err(err).Msg("failed to get finalized header")
		return log
	}
	log = log.With().Uint64("final_height", head.Height).Logger()

	phase, err := listener.state.Final().Phase()
	if err != nil {
		log.Error().Err(err).Msg("failed to get epoch phase")
		return log
	}
	log = log.With().Str("epoch_phase", phase.String()).Logger()

	return log
}

// OnIdentityTableChanged updates the networking layer's list of nodes to connect
// to when the identity table changes in the protocol state.
func (listener *NodeIDRefresher) OnIdentityTableChanged() {

	log := listener.getLogger()

	log.Info().Msg("updating network ids upon identity table change")

	// get the new set of IDs
	newIDs, err := IDsFromState(listener.state)
	if err != nil {
		log.Err(err).Msg("failed to determine new identity table after identity table change")
		return
	}

	// call the registered callback
	err = listener.callBack(newIDs)
	if err != nil {
		log.Err(err).Msg("failed to update network ids on identity table change")
		return
	}

	log.Info().Msg("successfully updated network ids upon identity table change")
}

// IDsFromState returns which nodes should be connected to based on the latest
// finalized state. The protocol state includes nodes from the previous/next
// epoch that should be included in network communication. We omit any nodes
// that have been ejected.
func IDsFromState(state protocol.ReadOnlyState) (flow.IdentityList, error) {
	return state.Final().Identities(filter.Not(filter.Ejected))
}
