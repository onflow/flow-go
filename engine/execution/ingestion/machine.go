package ingestion

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/common/requester"
	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation"
	"github.com/onflow/flow-go/engine/execution/ingestion/stop"
	"github.com/onflow/flow-go/engine/execution/ingestion/uploader"
	"github.com/onflow/flow-go/engine/execution/provider"
	"github.com/onflow/flow-go/engine/execution/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/events"
	"github.com/onflow/flow-go/storage"
)

// Machine forwards blocks and collections to the core for processing.
type Machine struct {
	events.Noop        // satisfy protocol events consumer interface
	log                zerolog.Logger
	core               *Core
	throttle           Throttle
	broadcaster        provider.ProviderEngine
	uploader           *uploader.Manager
	execState          state.ExecutionState
	computationManager computation.ComputationManager
}

type CollectionRequester interface {
	module.ReadyDoneAware
	WithHandle(requester.HandleFunc)
}

func NewMachine(
	logger zerolog.Logger,
	protocolEvents *events.Distributor,
	collectionRequester CollectionRequester,

	collectionFetcher CollectionFetcher,
	headers storage.Headers,
	blocks storage.Blocks,
	collections storage.Collections,
	execState state.ExecutionState,
	state protocol.State,
	metrics module.ExecutionMetrics,
	computationManager computation.ComputationManager,
	broadcaster provider.ProviderEngine,
	uploader *uploader.Manager,
	stopControl *stop.StopControl,
) (*Machine, module.ReadyDoneAware, error) {

	e := &Machine{
		log:                logger.With().Str("engine", "ingestion_machine").Logger(),
		broadcaster:        broadcaster,
		uploader:           uploader,
		execState:          execState,
		computationManager: computationManager,
	}

	throttle, err := NewBlockThrottle(
		logger,
		state,
		execState,
		headers,
	)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create block throttle: %w", err)
	}

	core, err := NewCore(
		logger,
		throttle,
		execState,
		stopControl,
		blocks,
		collections,
		e,
		collectionFetcher,
		e,
		metrics,
	)

	if err != nil {
		return nil, nil, fmt.Errorf("failed to create ingestion core: %w", err)
	}

	e.throttle = throttle
	e.core = core

	protocolEvents.AddConsumer(e)
	collectionRequester.WithHandle(func(originID flow.Identifier, entity flow.Entity) {
		collection, ok := entity.(*flow.Collection)
		if !ok {
			e.log.Error().Msgf("invalid entity type (%T)", entity)
			return
		}
		e.core.OnCollection(collection)
	})

	return e, core, nil
}

// Protocol Events implementation
func (e *Machine) BlockProcessable(header *flow.Header, qc *flow.QuorumCertificate) {
	err := e.throttle.OnBlock(qc.BlockID, header.Height)
	if err != nil {
		e.log.Fatal().Err(err).Msgf("error processing block %v (qc.BlockID: %v, blockID: %v)",
			header.Height, qc.BlockID, header.ID())
	}
}

func (e *Machine) BlockFinalized(b *flow.Header) {
	e.throttle.OnBlockFinalized(b.Height)
}

// EventConsumer implementation
var _ EventConsumer = (*Machine)(nil)

func (e *Machine) BeforeComputationResultSaved(
	ctx context.Context,
	result *execution.ComputationResult,
) {
	err := e.uploader.Upload(ctx, result)
	if err != nil {
		e.log.Err(err).Msg("error while uploading block")
		// continue processing. uploads should not block execution
	}
}

func (e *Machine) OnComputationResultSaved(
	ctx context.Context,
	result *execution.ComputationResult,
) string {
	header := result.BlockExecutionResult.ExecutableBlock.Block.Header
	broadcasted, err := e.broadcaster.BroadcastExecutionReceipt(
		ctx, header.Height, result.ExecutionReceipt)
	if err != nil {
		e.log.Err(err).Msg("critical: failed to broadcast the receipt")
	}
	return fmt.Sprintf("broadcasted: %v", broadcasted)
}

// BlockExecutor implementation
var _ BlockExecutor = (*Machine)(nil)

func (e *Machine) ExecuteBlock(ctx context.Context, executableBlock *entity.ExecutableBlock) (*execution.ComputationResult, error) {
	parentID := executableBlock.Block.Header.ParentID
	parentErID, err := e.execState.GetExecutionResultID(ctx, parentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get parent execution result ID %v: %w", parentID, err)
	}

	snapshot := e.execState.NewStorageSnapshot(*executableBlock.StartState,
		executableBlock.Block.Header.ParentID,
		executableBlock.Block.Header.Height-1,
	)

	computationResult, err := e.computationManager.ComputeBlock(
		ctx,
		parentErID,
		executableBlock,
		snapshot)
	if err != nil {
		return nil, fmt.Errorf("failed to compute block: %w", err)
	}

	return computationResult, nil
}
