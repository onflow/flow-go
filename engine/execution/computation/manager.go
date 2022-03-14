package computation

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/ipfs/go-cid"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"

	"github.com/onflow/flow-go/engine/execution/computation/computer/uploader"

	"github.com/onflow/flow-go/engine/execution"
	"github.com/onflow/flow-go/engine/execution/computation/computer"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/fvm/programs"
	"github.com/onflow/flow-go/fvm/state"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/mempool/entity"
	"github.com/onflow/flow-go/module/state_synchronization"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/utils/logging"
)

var uploadEnabled = true

func SetUploaderEnabled(enabled bool) {
	uploadEnabled = enabled

	log.Info().Msgf("changed uploadEnabled to %v", enabled)
}

type VirtualMachine interface {
	Run(fvm.Context, fvm.Procedure, state.View, *programs.Programs) error
	GetAccount(fvm.Context, flow.Address, state.View, *programs.Programs) (*flow.Account, error)
}

type ComputationManager interface {
	ExecuteScript([]byte, [][]byte, *flow.Header, state.View) ([]byte, error)
	ComputeBlock(
		ctx context.Context,
		block *entity.ExecutableBlock,
		view state.View,
	) (*execution.ComputationResult, error)
	GetAccount(addr flow.Address, header *flow.Header, view state.View) (*flow.Account, error)
}

var DefaultScriptLogThreshold = 1 * time.Second

const MaxScriptErrorMessageSize = 1000 // 1000 chars

// Manager manages computation and execution
type Manager struct {
	log                zerolog.Logger
	metrics            module.ExecutionMetrics
	me                 module.Local
	protoState         protocol.State
	vm                 VirtualMachine
	vmCtx              fvm.Context
	blockComputer      computer.BlockComputer
	programsCache      *ProgramsCache
	scriptLogThreshold time.Duration
	uploaders          []uploader.Uploader
	eds                state_synchronization.ExecutionDataService
	edCache            state_synchronization.ExecutionDataCIDCache
}

func New(
	logger zerolog.Logger,
	metrics module.ExecutionMetrics,
	tracer module.Tracer,
	me module.Local,
	protoState protocol.State,
	vm VirtualMachine,
	vmCtx fvm.Context,
	programsCacheSize uint,
	committer computer.ViewCommitter,
	scriptLogThreshold time.Duration,
	uploaders []uploader.Uploader,
	eds state_synchronization.ExecutionDataService,
	edCache state_synchronization.ExecutionDataCIDCache,
) (*Manager, error) {
	log := logger.With().Str("engine", "computation").Logger()

	blockComputer, err := computer.NewBlockComputer(
		vm,
		vmCtx,
		metrics,
		tracer,
		log.With().Str("component", "block_computer").Logger(),
		committer,
	)

	if err != nil {
		return nil, fmt.Errorf("cannot create block computer: %w", err)
	}

	programsCache, err := NewProgramsCache(programsCacheSize)
	if err != nil {
		return nil, fmt.Errorf("cannot create programs cache: %w", err)
	}

	e := Manager{
		log:                log,
		metrics:            metrics,
		me:                 me,
		protoState:         protoState,
		vm:                 vm,
		vmCtx:              vmCtx,
		blockComputer:      blockComputer,
		programsCache:      programsCache,
		scriptLogThreshold: scriptLogThreshold,
		uploaders:          uploaders,
		eds:                eds,
		edCache:            edCache,
	}

	return &e, nil
}

func (e *Manager) getChildProgramsOrEmpty(blockID flow.Identifier) *programs.Programs {
	blockPrograms := e.programsCache.Get(blockID)
	if blockPrograms == nil {
		return programs.NewEmptyPrograms()
	}
	return blockPrograms.ChildPrograms()
}

func (e *Manager) ExecuteScript(code []byte, arguments [][]byte, blockHeader *flow.Header, view state.View) ([]byte, error) {

	startedAt := time.Now()

	// allocate a random ID to be able to track this script when its done,
	// scripts might not be unique so we use this extra tracker to follow their logs
	// TODO: this is a temporary measure, we could remove this in the future
	trackerID := rand.Uint32()
	e.log.Info().Hex("script_hex", code).Uint32("trackerID", trackerID).Msg("script is sent for execution")

	defer func() {
		e.log.Info().Uint32("trackerID", trackerID).Msg("script execution is complete")
	}()

	blockCtx := fvm.NewContextFromParent(e.vmCtx, fvm.WithBlockHeader(blockHeader))

	script := fvm.Script(code).WithArguments(arguments...)

	programs := e.getChildProgramsOrEmpty(blockHeader.ID())

	err := func() (err error) {

		start := time.Now()

		defer func() {

			prepareLog := func() *zerolog.Event {

				args := make([]string, 0)
				for _, a := range arguments {
					args = append(args, hex.EncodeToString(a))
				}
				return e.log.Error().
					Hex("script_hex", code).
					Str("args", strings.Join(args[:], ","))
			}

			elapsed := time.Since(start)

			if r := recover(); r != nil {
				prepareLog().
					Interface("recovered", r).
					Msg("script execution caused runtime panic")

				err = fmt.Errorf("cadence runtime error: %s", r)
				return
			}
			if elapsed >= e.scriptLogThreshold {
				prepareLog().
					Dur("duration", elapsed).
					Msg("script execution exceeded threshold")
			}
		}()

		return e.vm.Run(blockCtx, script, view, programs)
	}()
	if err != nil {
		return nil, fmt.Errorf("failed to execute script (internal error): %w", err)
	}

	if script.Err != nil {
		scriptErrMsg := script.Err.Error()
		if len(scriptErrMsg) > MaxScriptErrorMessageSize {
			split := int(MaxScriptErrorMessageSize/2) - 1
			var sb strings.Builder
			sb.WriteString(scriptErrMsg[:split])
			sb.WriteString(" ... ")
			sb.WriteString(scriptErrMsg[len(scriptErrMsg)-split:])
			scriptErrMsg = sb.String()
		}

		return nil, fmt.Errorf("failed to execute script at block (%s): %s", blockHeader.ID(), scriptErrMsg)
	}

	encodedValue, err := jsoncdc.Encode(script.Value)
	if err != nil {
		return nil, fmt.Errorf("failed to encode runtime value: %w", err)
	}

	e.metrics.ExecutionScriptExecuted(time.Since(startedAt), script.GasUsed)

	return encodedValue, nil
}

func (e *Manager) ComputeBlock(
	ctx context.Context,
	block *entity.ExecutableBlock,
	view state.View,
) (*execution.ComputationResult, error) {

	e.log.Debug().
		Hex("block_id", logging.Entity(block.Block)).
		Msg("received complete block")

	var blockPrograms *programs.Programs
	fromCache := e.programsCache.Get(block.ParentID())

	if fromCache == nil {
		blockPrograms = programs.NewEmptyPrograms()
	} else {
		blockPrograms = fromCache.ChildPrograms()
	}

	result, err := e.blockComputer.ExecuteBlock(ctx, block, view, blockPrograms)
	if err != nil {
		e.log.Error().
			Hex("block_id", logging.Entity(block.Block)).
			Msg("failed to compute block result")

		return nil, fmt.Errorf("failed to execute block: %w", err)
	}

	toInsert := blockPrograms

	// if we have item from cache and there were no changes
	// insert it under new block, to prevent long chains
	if fromCache != nil && !blockPrograms.HasChanges() {
		toInsert = fromCache
	}

	e.programsCache.Set(block.ID(), toInsert)

	group, uploadCtx := errgroup.WithContext(ctx)
	var rootID flow.Identifier
	var blobTree [][]cid.Cid

	group.Go(func() error {
		var collections []*flow.Collection
		for _, collection := range result.ExecutableBlock.Collections() {
			collections = append(collections, &flow.Collection{
				Transactions: collection.Transactions,
			})
		}

		ed := &state_synchronization.ExecutionData{
			BlockID:     block.ID(),
			Collections: collections,
			Events:      result.Events,
			TrieUpdates: result.TrieUpdates,
		}

		var err error
		rootID, blobTree, err = e.eds.Add(uploadCtx, ed)

		return err
	})

	if uploadEnabled {
		for _, uploader := range e.uploaders {
			uploader := uploader

			group.Go(func() error {
				return uploader.Upload(result)
			})
		}
	}

	err = group.Wait()

	if err != nil {
		return nil, fmt.Errorf("failed to upload block result: %w", err)
	}

	e.log.Debug().
		Hex("block_id", logging.Entity(result.ExecutableBlock.Block)).
		Msg("computed block result")

	e.edCache.Insert(block.Block.Header, blobTree)
	result.ExecutionDataID = rootID

	return result, nil
}

func (e *Manager) GetAccount(address flow.Address, blockHeader *flow.Header, view state.View) (*flow.Account, error) {
	blockCtx := fvm.NewContextFromParent(e.vmCtx, fvm.WithBlockHeader(blockHeader))

	programs := e.getChildProgramsOrEmpty(blockHeader.ID())

	account, err := e.vm.GetAccount(blockCtx, address, view, programs)
	if err != nil {
		return nil, fmt.Errorf("failed to get account (%s) at block (%s): %w", address.String(), blockHeader.ID(), err)
	}

	return account, nil
}
