package tx_error_messages

import (
	"context"
	"errors"
	"time"

	"github.com/sethvargo/go-retry"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/engine/common/rpc"
	"github.com/onflow/flow-go/model/flow"
)

// TransactionErrorMessagesRequesterConfig contains the retry settings for the tx error messages fetch.
type TransactionErrorMessagesRequesterConfig struct {
	// the initial delay used in the exponential backoff for failed tx error messages download
	// retries.
	RetryDelay time.Duration
	// the max delay used in the exponential backoff for failed tx error messages download.
	MaxRetryDelay time.Duration
}

// TransactionErrorMessagesRequester is a thin wrapper over a TxErrorMessagesCore that manages the retrieval
// of transaction error messages with retry and backoff configurations for a specific execution result.
type TransactionErrorMessagesRequester struct {
	core            *TxErrorMessagesCore
	config          *TransactionErrorMessagesRequesterConfig
	executionResult *flow.ExecutionResult
}

func NewTransactionErrorMessagesRequester(
	core *TxErrorMessagesCore,
	config *TransactionErrorMessagesRequesterConfig,
	executionResult *flow.ExecutionResult,
) *TransactionErrorMessagesRequester {
	return &TransactionErrorMessagesRequester{
		core:            core,
		config:          config,
		executionResult: executionResult,
	}
}

// RequestTransactionErrorMessages fetches transaction error messages for the specific
// execution result this requester was configured with.
func (r *TransactionErrorMessagesRequester) RequestTransactionErrorMessages(
	ctx context.Context,
) ([]flow.TransactionResultErrorMessage, error) {
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
			r.core.log.Debug().
				Str("block_id", blockID.String()).
				Str("result_id", resultID.String()).
				Uint64("attempt", uint64(attempt)).
				Msgf("retrying download")
		}
		attempt++

		var err error
		errMessages, err = r.core.FetchErrorMessages(ctx, blockID, resultID)
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
