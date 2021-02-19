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
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

type Engine struct {
	notifications.NoopConsumer // satisfy the FinalizationConsumer interface

	unit      *engine.Unit
	log       zerolog.Logger
	state     protocol.State
	execState state.ExecutionState
}

func New(
	logger zerolog.Logger,
	state protocol.State,
	execState state.ExecutionState,
) *Engine {
	return &Engine{
		unit:      engine.NewUnit(),
		log:       logger.With().Str("engine", "checker").Logger(),
		state:     state,
		execState: execState,
	}
}

func (e *Engine) Ready() <-chan struct{} {
	// make sure we will run into a crashloop if result gets inconsistent
	// with sealed result.
	err := e.checkLastSealed()
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
	err := e.checkLastSealed()
	if err != nil {
		e.log.Fatal().Err(err).Msg("execution consistency check failed")
	}
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
