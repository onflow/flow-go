package backend

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/state_stream"
	"github.com/onflow/flow-go/engine/access/subscription"
	"github.com/onflow/flow-go/engine/access/subscription/tracker"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/execution"
	"github.com/onflow/flow-go/module/executiondatasync/optimistic_sync"
	"github.com/onflow/flow-go/module/state_synchronization/indexer"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/storage"
)

// Config defines the configurable options for the ingress server.
type Config struct {
	state_stream.EventFilterConfig

	// ListenAddr is the address the GRPC server will listen on as host:port
	ListenAddr string

	// MaxExecutionDataMsgSize is the max message size for block execution data API
	MaxExecutionDataMsgSize uint

	// RpcMetricsEnabled specifies whether to enable the GRPC metrics
	RpcMetricsEnabled bool

	// MaxGlobalStreams defines the global max number of streams that can be open at the same time.
	MaxGlobalStreams uint32

	// RegisterIDsRequestLimit defines the max number of register IDs that can be received in a single request.
	RegisterIDsRequestLimit uint32

	// ExecutionDataCacheSize is the max number of objects for the execution data cache.
	ExecutionDataCacheSize uint32

	// ClientSendTimeout is the timeout for sending a message to the client. After the timeout,
	// the stream is closed with an error.
	ClientSendTimeout time.Duration

	// ClientSendBufferSize is the size of the response buffer for sending messages to the client.
	ClientSendBufferSize uint

	// ResponseLimit is the max responses per second allowed on a stream. After exceeding the limit,
	// the stream is paused until more capacity is available. Searches of past data can be CPU
	// intensive, so this helps manage the impact.
	ResponseLimit float64

	// HeartbeatInterval specifies the block interval at which heartbeat messages should be sent.
	HeartbeatInterval uint64
}

type StateStreamBackend struct {
	tracker.ExecutionDataTracker

	ExecutionDataBackend
	EventsBackend
	AccountStatusesBackend

	log                  zerolog.Logger
	state                protocol.State
	headers              storage.Headers
	seals                storage.Seals
	registers            *execution.RegistersAsyncStore
	registerRequestLimit int
}

func New(
	log zerolog.Logger,
	state protocol.State,
	headers storage.Headers,
	seals storage.Seals,
	registers *execution.RegistersAsyncStore,
	registerIDsRequestLimit int,
	subscriptionFactory *subscription.SubscriptionHandler,
	executionDataTracker tracker.ExecutionDataTracker,
	executionResultProvider optimistic_sync.ExecutionResultInfoProvider,
	executionStateCache optimistic_sync.ExecutionStateCache,
	nodeRootBlock *flow.Header,
) (*StateStreamBackend, error) {
	logger := log.With().Str("module", "state_stream_api").Logger()

	b := &StateStreamBackend{
		ExecutionDataTracker: executionDataTracker,
		log:                  logger,
		state:                state,
		headers:              headers,
		seals:                seals,
		registers:            registers,
		registerRequestLimit: registerIDsRequestLimit,
	}

	b.ExecutionDataBackend = *NewExecutionDataBackend(
		log,
		state,
		headers,
		subscriptionFactory,
		executionDataTracker,
		executionResultProvider,
		executionStateCache,
		nodeRootBlock,
	)

	b.EventsBackend = *NewEventsBackend(
		logger,
		state,
		headers,
		nodeRootBlock,
		executionDataTracker,
		executionResultProvider,
		executionStateCache,
		subscriptionFactory,
	)

	b.AccountStatusesBackend = *NewAccountStatusesBackend(
		logger,
		state,
		headers,
		nodeRootBlock,
		executionDataTracker,
		executionResultProvider,
		executionStateCache,
		subscriptionFactory,
	)

	return b, nil
}

// GetRegisterValues returns the register values for the given register IDs at the given block height.
//
// CAUTION: this layer SIMPLIFIES the ERROR HANDLING convention
//   - All errors returned are guaranteed to be benign. The node can continue normal operations after such errors.
//   - To prevent delivering incorrect results to clients in case of an error, all other return values should be discarded.
//
// Expected sentinel errors providing details to clients about failed requests:
//   - [access.InvalidRequestError]: If the request had invalid arguments.
//   - [access.DataNotFoundError]: When data required to process the request is not available.
//   - [access.OutOfRangeError]: If the data for the requested height is outside the node's available range.
//   - [access.PreconditionFailedError]: When the register's database isn't initialized yet.
func (b *StateStreamBackend) GetRegisterValues(
	ctx context.Context,
	ids flow.RegisterIDs,
	height uint64,
	criteria optimistic_sync.Criteria,
) ([]flow.RegisterValue, *accessmodel.ExecutorMetadata, error) {
	if len(ids) > b.registerRequestLimit {
		return nil, nil, access.NewInvalidRequestError(
			fmt.Errorf("number of register IDs exceeds limit of %d", b.registerRequestLimit))
	}

	header, err := b.headers.ByHeight(height)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("failed to find header by height %d: %w", height, err)
		return nil, nil, access.NewDataNotFoundError("header", err)
	}

	// TODO: move this whole function to a separate small backend
	execResultInfo, err := b.EventsBackend.executionResultProvider.ExecutionResultInfo(header.ID(), criteria)
	if err != nil {
		err = fmt.Errorf("failed to get execution result info for block: %w", err)
		switch {
		case errors.Is(err, storage.ErrNotFound):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		case optimistic_sync.IsExecutionResultNotReadyError(err):
			return nil, nil, access.NewDataNotFoundError("execution data", err)
		case optimistic_sync.IsAgreeingExecutorsCountExceededError(err):
			return nil, nil, access.NewInvalidRequestError(err)
		case optimistic_sync.IsUnknownRequiredExecutorError(err):
			return nil, nil, access.NewInvalidRequestError(err)
		case optimistic_sync.IsCriteriaNotMetError(err):
			return nil, nil, access.NewInvalidRequestError(err)
		default:
			return nil, nil, access.RequireNoError(ctx, err)
		}
	}

	executionResultID := execResultInfo.ExecutionResultID
	snapshot, err := b.EventsBackend.executionStateCache.Snapshot(executionResultID)
	if err != nil {
		err = access.RequireErrorIs(ctx, err, storage.ErrNotFound)
		err = fmt.Errorf("failed to find snapshot by execution result ID %s: %w", executionResultID.String(), err)
		return nil, nil, access.NewDataNotFoundError("snapshot", err)
	}

	registers, err := snapshot.Registers()
	if err != nil {
		err = access.RequireErrorIs(ctx, err, indexer.ErrIndexNotInitialized)
		err = fmt.Errorf("failed to get registers storage from snapshot: %w", err)
		return nil, nil, access.NewPreconditionFailedError(err)
	}

	result := make([]flow.RegisterValue, len(ids))
	for i, regID := range ids {
		val, err := registers.Get(regID, height)
		if err != nil {
			err = fmt.Errorf("failed to get register by the register ID at a given block height: %w", err)
			switch {
			case errors.Is(err, storage.ErrNotFound):
				return nil, nil, access.NewDataNotFoundError("registers", err)
			case errors.Is(err, storage.ErrHeightNotIndexed):
				return nil, nil, access.NewOutOfRangeError(err)
			default:
				return nil, nil, access.RequireNoError(ctx, err)
			}

		}
		result[i] = val
	}

	metadata := &accessmodel.ExecutorMetadata{
		ExecutionResultID: executionResultID,
		ExecutorIDs:       execResultInfo.ExecutionNodes.NodeIDs(),
	}

	return result, metadata, nil
}
