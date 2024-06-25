package pebble

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

type Params struct {
	state *State
}

var _ protocol.Params = (*Params)(nil)

func (p Params) ChainID() (flow.ChainID, error) {

	// retrieve root header
	root, err := p.FinalizedRoot()
	if err != nil {
		return "", fmt.Errorf("could not get root: %w", err)
	}

	return root.ChainID, nil
}

func (p Params) SporkID() (flow.Identifier, error) {

	var sporkID flow.Identifier
	err := operation.RetrieveSporkID(&sporkID)(p.state.db)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get spork id: %w", err)
	}

	return sporkID, nil
}

func (p Params) SporkRootBlockHeight() (uint64, error) {
	var sporkRootBlockHeight uint64
	err := operation.RetrieveSporkRootBlockHeight(&sporkRootBlockHeight)(p.state.db)
	if err != nil {
		return 0, fmt.Errorf("could not get spork root block height: %w", err)
	}

	return sporkRootBlockHeight, nil
}

func (p Params) ProtocolVersion() (uint, error) {

	var version uint
	err := operation.RetrieveProtocolVersion(&version)(p.state.db)
	if err != nil {
		return 0, fmt.Errorf("could not get protocol version: %w", err)
	}

	return version, nil
}

func (p Params) EpochCommitSafetyThreshold() (uint64, error) {

	var threshold uint64
	err := operation.RetrieveEpochCommitSafetyThreshold(&threshold)(p.state.db)
	if err != nil {
		return 0, fmt.Errorf("could not get epoch commit safety threshold")
	}
	return threshold, nil
}

func (p Params) EpochFallbackTriggered() (bool, error) {
	var triggered bool
	err := operation.CheckEpochEmergencyFallbackTriggered(&triggered)(p.state.db)
	if err != nil {
		return false, fmt.Errorf("could not check epoch fallback triggered: %w", err)
	}
	return triggered, nil
}

func (p Params) FinalizedRoot() (*flow.Header, error) {

	// look up root block ID
	var rootID flow.Identifier
	err := operation.LookupBlockHeight(p.state.finalizedRootHeight, &rootID)(p.state.db)
	if err != nil {
		return nil, fmt.Errorf("could not look up root header: %w", err)
	}

	// retrieve root header
	header, err := p.state.headers.ByBlockID(rootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root header: %w", err)
	}

	return header, nil
}

func (p Params) SealedRoot() (*flow.Header, error) {
	// look up root block ID
	var rootID flow.Identifier
	err := operation.LookupBlockHeight(p.state.sealedRootHeight, &rootID)(p.state.db)

	if err != nil {
		return nil, fmt.Errorf("could not look up root header: %w", err)
	}

	// retrieve root header
	header, err := p.state.headers.ByBlockID(rootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root header: %w", err)
	}

	return header, nil
}

func (p Params) Seal() (*flow.Seal, error) {

	// look up root header
	var rootID flow.Identifier
	err := operation.LookupBlockHeight(p.state.finalizedRootHeight, &rootID)(p.state.db)
	if err != nil {
		return nil, fmt.Errorf("could not look up root header: %w", err)
	}

	// retrieve the root seal
	seal, err := p.state.seals.HighestInFork(rootID)
	if err != nil {
		return nil, fmt.Errorf("could not retrieve root seal: %w", err)
	}

	return seal, nil
}
