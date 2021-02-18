package checker

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/consensus/hotstuff/notifications"
	"github.com/onflow/flow-go/engine"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/utils/logging"
)

type Engine struct {
	notifications.NoopConsumer // satisfy the FinalizationConsumer interface

	unit      *engine.Unit
	log       zerolog.Logger
	state     protocol.State
	execState state.ExecutionState
	seals     storage.Seals // to compare the executed result with the sealed result
}

func New(
	logger zerolog.Logger,
	state protocol.State,
	execState state.ExecutionState,
	seals storage.Seals,
) *Engine {
	return &Engine{
		unit:      engine.NewUnit(),
		log:       logger.With().Str("engine", "checker").Logger(),
		state:     state,
		execState: execState,
		seals:     seals,
	}
}

func (e *Engine) Ready() <-chan struct{} {
	// make sure we will run into a crashloop if result gets inconsistent
	// with sealed result.
	err := e.checkLastSealedAndLastExecuted()
	if err != nil {
		e.log.Fatal().Err(err).Msg("execution consistency check failed")
	}
	return e.unit.Ready()
}

func (e *Engine) Done() <-chan struct{} {
	return e.unit.Done()
}

// when a block is finalized check if the last sealed has been executed,
// if it has been executed, check whether if the sealed result is consistent
// with the executed result
func (e *Engine) OnFinalizedBlock(block *model.Block) {
	err := e.checkLastSealed()
	if err != nil {
		e.log.Fatal().Err(err).Msg("execution consistency check failed")
	}
}

// CheckExecutionConsistencyWithBlock when a block is executed, if the block has been sealed, check whether if the sealed result is consistent
// with the executed result
func (e *Engine) CheckExecutionConsistencyWithBlock(block *flow.Block, mycommit flow.StateCommitment) {
	err := e.checkLastExecuted(block.ID(), block.Header.Height, mycommit)
	if err != nil {
		e.log.Fatal().Err(err).
			Hex("block_id", logging.Entity(block)).
			Uint64("height", block.Header.Height).
			Msg("failed to executed block consistency")
	}
}

// check if last sealed block has been executed, if yes, check if the result is consistent
// check if the last executed has been sealed, if yes, check if the result is consistent
func (e *Engine) checkLastSealedAndLastExecuted() error {
	err := e.checkLastSealed()
	if err != nil {
		return fmt.Errorf("failed to check last sealed: %w", err)
	}

	height, blockID, err := e.execState.GetHighestExecutedBlockID(e.unit.Ctx())
	if err != nil {
		// could happen on first startup or remove execution result
		e.log.Warn().Err(err).
			Msg("failed to get highest executed block")
		return nil
	}

	mycommit, err := e.execState.StateCommitmentByBlockID(e.unit.Ctx(), blockID)
	if err != nil {
		return fmt.Errorf("could not get statecommitment for block: %v at height: %v, err: %w",
			blockID, height, err)
	}

	err = e.checkLastExecuted(blockID, height, mycommit)
	if err != nil {
		return fmt.Errorf("could not check last executed, height: %v, blockID: %v, err: %w",
			height, blockID, err)
	}
	return nil
}

func (e *Engine) checkLastSealed() error {
	sealedSnapshot := e.state.Sealed()
	sealed, err := sealedSnapshot.Head()

	if err != nil {
		return fmt.Errorf("could not get sealed block OnFinalizedBlock: %w", err)
	}

	sealedCommit, err := sealedSnapshot.Commit()
	if err != nil {
		return fmt.Errorf("could not get sealed state commitment OnFinalizedBlock: %w", err)
	}

	blockID := sealed.ID()

	mycommit, err := e.execState.StateCommitmentByBlockID(e.unit.Ctx(), blockID)
	if errors.Is(err, storage.ErrNotFound) {
		// have not executed the sealed block yet
		// in other words, this can't detect execution fork, if the execution is behind
		// the sealing
		return nil
	}

	if err != nil {
		return fmt.Errorf("could not get my state commitment OnFinalizedBlock, blockID: %v", blockID)
	}

	if !bytes.Equal(mycommit, sealedCommit) {
		return fmt.Errorf("execution result is different from the sealed result, height: %v, block_id: %v, sealed_commit: %x, my_commit: %x",
			sealed.Height, blockID, sealedCommit, mycommit)
	}

	return nil
}

func (e *Engine) checkLastExecuted(blockID flow.Identifier, height uint64, mycommit flow.StateCommitment) error {
	sealed, err := e.state.Sealed().Head()
	if err != nil {
		return fmt.Errorf("could not get sealed block on checking result consistency: %w", err)
	}

	if height > sealed.Height {
		// executed block has not been sealed yet
		return nil
	}

	seal, err := e.seals.ByBlockID(blockID)
	if err != nil {
		return fmt.Errorf("could not get seal for block: %w", err)
	}

	sealedCommit := seal.FinalState
	if !bytes.Equal(mycommit, sealedCommit) {
		return fmt.Errorf("executed block's result is inconsistent with sealed result, sealed_commit: %x, my_commit: %x",
			sealedCommit, mycommit)
	}

	return nil
}
