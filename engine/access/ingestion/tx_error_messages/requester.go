package tx_error_messages

import (
	"context"
	"errors"
	"fmt"
	"time"

	execproto "github.com/onflow/flow/protobuf/go/flow/execution"
	"github.com/rs/zerolog"
	"github.com/sethvargo/go-retry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc/convert"

	"github.com/onflow/flow-go/engine/access/rpc/backend"
	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
)

// RequesterConfig contains the retry settings for the tx error messages fetch.
type RequesterConfig struct {
	// the initial delay used in the exponential backoff for failed tx error messages download
	// retries.
	RetryDelay time.Duration
	// the max delay used in the exponential backoff for failed tx error messages download.
	MaxRetryDelay time.Duration
}

type Requester struct {
	logger                     zerolog.Logger
	config                     *RequesterConfig
	backend                    *backend.Backend
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider
	executionResult            *flow.ExecutionResult
}

func NewRequester(
	logger zerolog.Logger,
	config *RequesterConfig,
	backend *backend.Backend,
	execNodeIdentitiesProvider *rpc.ExecutionNodeIdentitiesProvider,
	executionResult *flow.ExecutionResult,
) *Requester {
	return &Requester{
		logger:                     logger,
		config:                     config,
		backend:                    backend,
		execNodeIdentitiesProvider: execNodeIdentitiesProvider,
		executionResult:            executionResult,
	}
}

// Request fetches transaction error messages for the specific
// execution result this requester was configured with.
func (r *Requester) Request(ctx context.Context) ([]flow.TransactionResultErrorMessage, error) {
	backoff := retry.NewExponential(r.config.RetryDelay)
	backoff = retry.WithCappedDuration(r.config.MaxRetryDelay, backoff)
	backoff = retry.WithJitterPercent(15, backoff)

	blockID := r.executionResult.BlockID
	resultID := r.executionResult.ID()

	var (
		errMessages []flow.TransactionResultErrorMessage
		lastErr     error
	)

	attempt := 0
	err := retry.Do(ctx, backoff, func(context.Context) error {
		if attempt > 0 {
			r.logger.Debug().
				Str("block_id", blockID.String()).
				Str("result_id", resultID.String()).
				Uint64("attempt", uint64(attempt)).
				Msgf("retrying download")
		}
		attempt++

		var err error
		errMessages, err = r.request(ctx, blockID, resultID)
		if errors.Is(err, rpc.ErrNoENsFoundForExecutionResult) || status.Code(err) != codes.Canceled {
			lastErr = err
			return retry.RetryableError(err)
		}

		lastErr = err
		return err
	})

	if err != nil {
		return nil, lastErr
	}
	return errMessages, nil
}

func (r *Requester) request(
	ctx context.Context,
	blockID flow.Identifier,
	resultID flow.Identifier,
) ([]flow.TransactionResultErrorMessage, error) {
	execNodes, err := r.execNodeIdentitiesProvider.ExecutionNodesForResultID(blockID, resultID)
	if err != nil {
		r.logger.Error().Err(err).
			Str("block_id", blockID.String()).
			Str("result_id", resultID.String()).
			Msg("failed to find execution nodes for specific result ID")
		return nil, fmt.Errorf("could not find execution nodes for result %v in block %v: %w", resultID, blockID, err)
	}

	r.logger.Debug().
		Msgf("transaction error messages for block %s are being downloaded", blockID)

	req := &execproto.GetTransactionErrorMessagesByBlockIDRequest{
		BlockId: convert.IdentifierToMessage(blockID),
	}

	resp, execNode, err := r.backend.GetTransactionErrorMessagesFromAnyEN(ctx, execNodes, req)
	if err != nil {
		r.logger.Error().Err(err).Msg("failed to get transaction error messages from execution nodes")
		return nil, err
	}

	errorMessages := r.convertResponse(resp, execNode)
	return errorMessages, nil
}

func (r *Requester) convertResponse(
	responseMessages []*execproto.GetTransactionErrorMessagesResponse_Result,
	execNode *flow.IdentitySkeleton,
) []flow.TransactionResultErrorMessage {
	errorMessages := make([]flow.TransactionResultErrorMessage, 0, len(responseMessages))
	for _, value := range responseMessages {
		errorMessage := flow.TransactionResultErrorMessage{
			ErrorMessage:  value.ErrorMessage,
			TransactionID: convert.MessageToIdentifier(value.TransactionId),
			Index:         value.Index,
			ExecutorID:    execNode.NodeID,
		}
		errorMessages = append(errorMessages, errorMessage)
	}

	return errorMessages
}
