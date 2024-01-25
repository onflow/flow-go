package computation

import (
	"context"
	"fmt"

	"github.com/onflow/cadence/runtime"
	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/engine/execution/computation/query"
	"github.com/onflow/flow-go/fvm"
	reusableRuntime "github.com/onflow/flow-go/fvm/runtime"
	"github.com/onflow/flow-go/fvm/storage/derived"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/executiondatasync/provider"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/logging"
)

const (
	ReusableCadenceRuntimePoolSize = 1000
)

type ComputationManager interface {
	ExecuteScript(
		ctx context.Context,
		script []byte,
		arguments [][]byte,
		blockHeader *flow.Header,
		snapshot snapshot.StorageSnapshot,
	) (
		[]byte,
		error,
	)

	ComputeBlock(
		ctx context.Context,
		parentBlockExecutionResultID flow.Identifier,
		block *entity.ExecutableBlock,
		snapshot snapshot.StorageSnapshot,
	) (
		*execution.ComputationResult,
		error,
	)

	GetAccount(
		ctx context.Context,
		addr flow.Address,
		header *flow.Header,
		snapshot snapshot.StorageSnapshot,
	) (
		*flow.Account,
		error,
	)
}

type ComputationConfig struct {
	query.QueryConfig
	CadenceTracing       bool
	ExtensiveTracing     bool
	DerivedDataCacheSize uint
	MaxConcurrency       int

	// When NewCustomVirtualMachine is nil, the manager will create a standard
	// fvm virtual machine via fvm.NewVirtualMachine.  Otherwise, the manager
	// will create a virtual machine using this function.
	//
	// Note that this is primarily used for testing.
	NewCustomVirtualMachine func() fvm.VM
}

// Manager manages computation and execution
type Manager struct {
	log              zerolog.Logger
	vm               fvm.VM
	blockComputer    computer.BlockComputer
	queryExecutor    query.Executor
	derivedChainData *derived.DerivedChainData
}

var _ ComputationManager = &Manager{}

func New(
	logger zerolog.Logger,
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	me module.Local,
	protoState protocol.State,
	vmCtx fvm.Context,
	committer computer.ViewCommitter,
	executionDataProvider provider.Provider,
	params ComputationConfig,
) (*Manager, error) {
	log := logger.With().Str("engine", "computation").Logger()

	var vm fvm.VM
	if params.NewCustomVirtualMachine != nil {
		vm = params.NewCustomVirtualMachine()
	} else {
		vm = fvm.NewVirtualMachine()
	}

	chainID := vmCtx.Chain.ChainID()
	options := DefaultFVMOptions(chainID, params.CadenceTracing, params.ExtensiveTracing)
	vmCtx = fvm.NewContextFromParent(vmCtx, options...)

	blockComputer, err := computer.NewBlockComputer(
		vm,
		vmCtx,
		metrics,
		tracer,
		log.With().Str("component", "block_computer").Logger(),
		committer,
		me,
		executionDataProvider,
		nil, // TODO(ramtin): update me with proper consumers
		protoState,
		params.MaxConcurrency,
	)

	if err != nil {
		return nil, fmt.Errorf("cannot create block computer: %w", err)
	}

	derivedChainData, err := derived.NewDerivedChainData(params.DerivedDataCacheSize)
	if err != nil {
		return nil, fmt.Errorf("cannot create derived data cache: %w", err)
	}

	queryExecutor := query.NewQueryExecutor(
		params.QueryConfig,
		logger,
		metrics,
		vm,
		vmCtx,
		derivedChainData,
		query.NewProtocolStateWrapper(protoState),
	)

	e := Manager{
		log:              log,
		vm:               vm,
		blockComputer:    blockComputer,
		queryExecutor:    queryExecutor,
		derivedChainData: derivedChainData,
	}

	return &e, nil
}

func (e *Manager) VM() fvm.VM {
	return e.vm
}

func (e *Manager) ComputeBlock(
	ctx context.Context,
	parentBlockExecutionResultID flow.Identifier,
	block *entity.ExecutableBlock,
	snapshot snapshot.StorageSnapshot,
) (*execution.ComputationResult, error) {

	e.log.Debug().
		Hex("block_id", logging.Entity(block.Block)).
		Msg("received complete block")

	derivedBlockData := e.derivedChainData.GetOrCreateDerivedBlockData(
		block.ID(),
		block.ParentID())

	result, err := e.blockComputer.ExecuteBlock(
		ctx,
		parentBlockExecutionResultID,
		block,
		snapshot,
		derivedBlockData)
	if err != nil {
		e.log.Error().
			Hex("block_id", logging.Entity(block.Block)).
			Msg("failed to compute block result")

		return nil, fmt.Errorf("failed to execute block: %w", err)
	}

	e.log.Debug().
		Hex("block_id", logging.Entity(result.ExecutableBlock.Block)).
		Msg("computed block result")

	return result, nil
}

func (e *Manager) ExecuteScript(
	ctx context.Context,
	code []byte,
	arguments [][]byte,
	blockHeader *flow.Header,
	snapshot snapshot.StorageSnapshot,
) ([]byte, error) {
	return e.queryExecutor.ExecuteScript(ctx,
		code,
		arguments,
		blockHeader,
		snapshot)
}

func (e *Manager) GetAccount(
	ctx context.Context,
	address flow.Address,
	blockHeader *flow.Header,
	snapshot snapshot.StorageSnapshot,
) (
	*flow.Account,
	error,
) {
	return e.queryExecutor.GetAccount(
		ctx,
		address,
		blockHeader,
		snapshot)
}

func (e *Manager) QueryExecutor() query.Executor {
	return e.queryExecutor
}

func DefaultFVMOptions(chainID flow.ChainID, cadenceTracing bool, extensiveTracing bool) []fvm.Option {
	options := []fvm.Option{
		fvm.WithChain(chainID.Chain()),
		fvm.WithReusableCadenceRuntimePool(
			reusableRuntime.NewReusableCadenceRuntimePool(
				ReusableCadenceRuntimePoolSize,
				runtime.Config{
					TracingEnabled: cadenceTracing,
					// Attachments are enabled everywhere except for Mainnet
					AttachmentsEnabled: chainID != flow.Mainnet,
				},
			)),
		fvm.WithEVMEnabled(true),
	}

	if extensiveTracing {
		options = append(options, fvm.WithExtensiveTracing())
	}

	return options
}
