package badger

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
)

// StateRoot is the root information required to bootstrap the protocol state
type StateRoot struct {
	block          *flow.Block
	result         *flow.ExecutionResult
	seal           *flow.Seal
	epochFirstView uint64
}

func NewStateRoot(block *flow.Block, result *flow.ExecutionResult, seal *flow.Seal, epochFirstView uint64) (*StateRoot, error) {
	err := validate(block, result, seal, epochFirstView)
	if err != nil {
		return nil, fmt.Errorf("inconsistent state root: %w", err)
	}
	return &StateRoot{
		block:          block,
		result:         result,
		seal:           seal,
		epochFirstView: epochFirstView,
	}, nil
}

// validate checks internal consistency of state root
func validate(block *flow.Block, result *flow.ExecutionResult, seal *flow.Seal, epochFirstView uint64) error {
	if result.BlockID != block.ID() {
		return fmt.Errorf("root execution result for wrong block (%x != %x)", result.BlockID, block.ID())
	}

	if seal.BlockID != block.ID() {
		return fmt.Errorf("root block seal for wrong block (%x != %x)", seal.BlockID, block.ID())
	}

	if seal.ResultID != result.ID() {
		return fmt.Errorf("root block seal for wrong execution result (%x != %x)", seal.ResultID, result.ID())
	}

	// We should have exactly two service events, one epoch setup and one epoch commit.
	if len(seal.ServiceEvents) != 2 {
		return fmt.Errorf("root block seal must contain two system events (have %d)", len(seal.ServiceEvents))
	}
	setup, valid := seal.ServiceEvents[0].Event.(*flow.EpochSetup)
	if !valid {
		return fmt.Errorf("first service event should be epoch setup (%T)", seal.ServiceEvents[0])
	}
	commit, valid := seal.ServiceEvents[1].Event.(*flow.EpochCommit)
	if !valid {
		return fmt.Errorf("second event should be epoch commit (%T)", seal.ServiceEvents[1])
	}

	// They should both have the same epoch counter to be valid.
	if setup.Counter != commit.Counter {
		return fmt.Errorf("epoch setup counter differs from epoch commit counter (%d != %d)", setup.Counter, commit.Counter)
	}

	// the root block's view must be within the Epoch
	if epochFirstView > block.Header.View {
		return fmt.Errorf("root block has lower view than first view of epoch")
	}
	if block.Header.View >= setup.FinalView {
		return fmt.Errorf("final view of epoch less than first block view")
	}

	// They should also both be valid within themselves
	err := validSetup(setup)
	if err != nil {
		return fmt.Errorf("invalid epoch setup event: %w", err)
	}
	err = validCommit(commit, setup)
	if err != nil {
		return fmt.Errorf("invalid epoch commit event: %w", err)
	}

	// Validate the root block and its payload
	// NOTE: we might need to relax these restrictions and find a way to
	// process the payload of the root block once we implement epochs

	// the root block should have an empty guarantee payload
	if len(block.Payload.Guarantees) > 0 {
		return fmt.Errorf("root block must not have guarantees")
	}

	// the root block should have an empty seal payload
	if len(block.Payload.Seals) > 0 {
		return fmt.Errorf("root block must not have seals")
	}

	return nil
}

func (s StateRoot) EpochSetupEvent() *flow.EpochSetup {
	// CAUTION: the epoch setup event as emitted by the epoch smart contract does
	// NOT include the Epoch's first view. Instead, this information is added
	// locally (trusted) when stored to simplify epoch queries.

	// copy service event from seal (to not modify seal in-place)
	// and set Epoch's first view on the copy
	es := s.seal.ServiceEvents[0].Event.(*flow.EpochSetup)
	var event flow.EpochSetup = *es
	event.FirstView = s.epochFirstView

	return &event
}

func (s StateRoot) EpochCommitEvent() *flow.EpochCommit {
	return s.seal.ServiceEvents[1].Event.(*flow.EpochCommit)
}

func (s StateRoot) Block() *flow.Block {
	return s.block
}

func (s StateRoot) Seal() *flow.Seal {
	return s.seal
}

func (s StateRoot) Result() *flow.ExecutionResult {
	return s.result
}
