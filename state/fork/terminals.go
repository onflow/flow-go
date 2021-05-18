package fork

import (
	"fmt"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
)

// IncludingBlock returns a Terminal implementation where we explicitly
// specify the ID of the lowest block that should be visited
type IncludingBlock flow.Identifier

// LowestHeightToVisit computes the height of the lowest block that should be visited
func (t IncludingBlock) LowestHeightToVisit(headers storage.Headers) (uint64, error) {
	terminalHeader, err := headers.ByBlockID(flow.Identifier(t))
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve header of terminal block %x: %w", flow.Identifier(t), err)
	}
	return terminalHeader.Height, nil
}

// ConfirmTerminalReached is a self-consistency check that the lowest visited block is
// in fact the expected terminal.
func (t IncludingBlock) ConfirmTerminalReached(headers storage.Headers, lowestVisitedBlock *flow.Header) error {
	if lowestVisitedBlock.ID() != flow.Identifier(t) {
		return fmt.Errorf("last visited block has ID %x but expecting %x", lowestVisitedBlock.ID(), flow.Identifier(t))
	}
	return nil
}

// ExcludingBlock returns a Terminal implementation where we explicitly
// specify the ID of the lowest block that should _not_ be visited anymore
type ExcludingBlock flow.Identifier

// LowestHeightToVisit computes the height of the lowest block that should be visited
func (t ExcludingBlock) LowestHeightToVisit(headers storage.Headers) (uint64, error) {
	id := flow.Identifier(t)
	terminalHeader, err := headers.ByBlockID(id)
	if err != nil {
		return 0, fmt.Errorf("failed to retrieve header of terminal block %x: %w", id, err)
	}
	return terminalHeader.Height + 1, nil
}

// ConfirmTerminalReached is a self-consistency check that the lowest visited block is
// in fact the expected terminal.
func (t ExcludingBlock) ConfirmTerminalReached(headers storage.Headers, lowestVisitedBlock *flow.Header) error {
	if lowestVisitedBlock.ParentID != flow.Identifier(t) {
		return fmt.Errorf("parent of last visited block has ID %x but expecting %x", lowestVisitedBlock.ParentID, flow.Identifier(t))
	}
	return nil
}

// IncludingHeight returns a Terminal implementation where we
// specify the height of the lowest block that should be visited
type IncludingHeight uint64

// LowestHeightToVisit computes the height of the lowest block that should be visited
func (t IncludingHeight) LowestHeightToVisit(storage.Headers) (uint64, error) {
	return uint64(t), nil
}

// ConfirmTerminalReached is a self-consistency check that the lowest visited block is
// in fact the expected terminal.
func (t IncludingHeight) ConfirmTerminalReached(headers storage.Headers, lowestVisitedBlock *flow.Header) error {
	if lowestVisitedBlock.Height != uint64(t) {
		return fmt.Errorf("expecting terminal block with height %d but got %d", uint64(t), lowestVisitedBlock.Height)
	}
	return nil
}

// ExcludingHeight returns a Terminal implementation where we
// specify the Height of the lowest block that should _not_ be visited anymore
type ExcludingHeight uint64

// LowestHeightToVisit computes the height of the lowest block that should be visited
func (t ExcludingHeight) LowestHeightToVisit(storage.Headers) (uint64, error) {
	return uint64(t) + 1, nil
}

// ConfirmTerminalReached is a self-consistency check that the lowest visited block is
// in fact the expected terminal.
func (t ExcludingHeight) ConfirmTerminalReached(headers storage.Headers, lowestVisitedBlock *flow.Header) error {
	if lowestVisitedBlock.Height != uint64(t)+1 {
		return fmt.Errorf("expecting terminal block with height %d but got %d", uint64(t)+1, lowestVisitedBlock.Height)
	}
	return nil
}
