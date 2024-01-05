package checker

import (
	"context"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Core is the core logic of the checker engine that checks if the execution result matches the sealed result.
type Core struct {
	log       zerolog.Logger
	state     protocol.State
	execState state.ExecutionState
}

func NewCore(
	logger zerolog.Logger,
	state protocol.State,
	execState state.ExecutionState,
) *Core {
	e := &Core{
		log:       logger.With().Str("engine", "checker").Logger(),
		state:     state,
		execState: execState,
	}

	return e
}

// checkMyCommitWithSealedCommit is the main check of the checker engine
func checkMyCommitWithSealedCommit(
	executedBlock *flow.Header,
	myCommit flow.StateCommitment,
	sealedCommit flow.StateCommitment,
) error {
	if myCommit != sealedCommit {
		// mismatch
		return fmt.Errorf("execution result is different from the sealed result, height: %v, block_id: %v, sealed_commit: %v, my_commit: %v",
			executedBlock.Height,
			executedBlock.ID(),
			sealedCommit,
			myCommit,
		)
	}

	// match
	return nil
}

// RunCheck skips when the last sealed has not been executed, and last executed has not been finalized.
func (c *Core) RunCheck() error {
	// find last sealed block
	lastSealedBlock, lastFinal, seal, err := c.findLastSealedBlock()
	if err != nil {
		return err
	}

	mycommitAtLastSealed, err := c.execState.StateCommitmentByBlockID(lastSealedBlock.ID())
	if err == nil {
		// if last sealed block has been executed, then check if they match
		return checkMyCommitWithSealedCommit(lastSealedBlock, mycommitAtLastSealed, seal.FinalState)
	}

	// if last sealed block has not been executed, then check if recent executed block has
	// been sealed already, if yes, check if they match.
	lastExecutedHeight, err := c.findLastExecutedBlockHeight()
	if err != nil {
		return err
	}

	if lastExecutedHeight > lastFinal.Height {
		// last executed block has not been finalized yet,
		// can't check since unfinalized block is also unsealed, skip
		return nil
	}

	// TODO: better to query seals from protocol state,
	// switch to state.Final().LastSealed() when available
	sealedExecuted, seal, err := c.findLatestSealedAtHeight(lastExecutedHeight)
	if err != nil {
		return fmt.Errorf("could not get the last sealed block at height: %v, err: %w", lastExecutedHeight, err)
	}

	sealedCommit := seal.FinalState

	mycommit, err := c.execState.StateCommitmentByBlockID(seal.BlockID)
	if errors.Is(err, storage.ErrNotFound) {
		// have not executed the sealed block yet
		// in other words, this can't detect execution fork, if the execution is behind
		// the sealing
		return nil
	}

	if err != nil {
		return fmt.Errorf("could not get my state commitment OnFinalizedBlock, blockID: %v", seal.BlockID)
	}

	return checkMyCommitWithSealedCommit(sealedExecuted, mycommit, sealedCommit)
}

// findLastSealedBlock finds the last sealed block
func (c *Core) findLastSealedBlock() (*flow.Header, *flow.Header, *flow.Seal, error) {
	finalized := c.state.Final()
	lastFinal, err := finalized.Head()
	if err != nil {
		return nil, nil, nil, err
	}

	_, lastSeal, err := finalized.SealedResult()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not get the last sealed for the finalized block: %w", err)
	}

	lastSealed, err := c.state.AtBlockID(lastSeal.BlockID).Head()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("could not get the last sealed block: %w", err)
	}

	return lastSealed, lastFinal, lastSeal, nil
}

// findLastExecutedBlockHeight finds the last executed block height
func (c *Core) findLastExecutedBlockHeight() (uint64, error) {
	height, _, err := c.execState.GetHighestExecutedBlockID(context.Background())
	if err != nil {
		return 0, fmt.Errorf("could not get the last executed block: %w", err)
	}
	return height, nil
}

// findLatestSealedAtHeight finds the latest sealed block at the given height
func (c *Core) findLatestSealedAtHeight(finalizedHeight uint64) (*flow.Header, *flow.Seal, error) {
	_, seal, err := c.state.AtHeight(finalizedHeight).SealedResult()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get the last sealed for the finalized block: %w", err)
	}

	sealed, err := c.state.AtBlockID(seal.BlockID).Head()
	if err != nil {
		return nil, nil, fmt.Errorf("could not get the last sealed block: %w", err)
	}
	return sealed, seal, nil
}
