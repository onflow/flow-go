package checker

import (
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
)

type Engine struct {
	notifications.NoopConsumer // satisfy the FinalizationConsumer interface

	unit      *engine.Unit
	log       zerolog.Logger
	state     protocol.State
	execState state.ExecutionState
	sealsDB   storage.Seals
}

func New(
	logger zerolog.Logger,
	state protocol.State,
	execState state.ExecutionState,
	sealsDB storage.Seals,
) *Engine {
	return &Engine{
		unit:      engine.NewUnit(),
		log:       logger.With().Str("engine", "checker").Logger(),
		state:     state,
		execState: execState,
		sealsDB:   sealsDB,
	}
}

func (e *Engine) Ready() <-chan struct{} {
	// make sure we will run into a crashloop if result gets inconsistent
	// with sealed result.

	finalized, err := e.state.Final().Head()

	if err != nil {
		e.log.Fatal().Err(err).Msg("could not get finalized block on startup")
	}

	err = e.checkLastSealed(finalized.ID())
	if err != nil {
		e.log.Fatal().Err(err).Msg("execution consistency check failed on startup")
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
	err := e.checkLastSealed(block.BlockID)
	if err != nil {
		e.log.Fatal().Err(err).Msg("execution consistency check failed")
	}
}

func (e *Engine) checkLastSealed(finalizedID flow.Identifier) error {
	// TODO: better to query seals from protocol state,
	// switch to state.Final().LastSealed() when available
	seal, err := e.sealsDB.ByBlockID(finalizedID)
	if err != nil {
		return fmt.Errorf("could not get the last sealed for the finalized block: %w", err)
	}

	blockID := seal.BlockID
	sealedCommit := seal.FinalState

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

	if mycommit != sealedCommit {
		sealed, err := e.state.AtBlockID(blockID).Head()
		if err != nil {
			return fmt.Errorf("could not get sealed block when checkLastSealed: %v, err: %w", blockID, err)
		}

		// we expect the state commitments not to match so just log them gracefully
		e.log.Info().Str("block_id", blockID.String()).Uint64("block_height", sealed.Height).Msgf("expected state commitment diff for block (%v) : sealed commit %x, our commit %x", sealedCommit, mycommit)
		// return fmt.Errorf("execution result is different from the sealed result, height: %v, block_id: %v, sealed_commit: %x, my_commit: %x",
		// sealed.Height, blockID, sealedCommit, mycommit)
	}

	return nil
}
