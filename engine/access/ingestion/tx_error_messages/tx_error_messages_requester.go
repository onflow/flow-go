package tx_error_messages

import (
	"context"
	"time"

	"github.com/sethvargo/go-retry"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/irrecoverable"
)

// TransactionErrorMessagesRequesterConfig contains the retry settings for the tx error messages fetch.
type TransactionErrorMessagesRequesterConfig struct {
	// the initial timeout for fetching tx error messages from the db/network. The timeout is
	// increased using an incremental backoff until FetchTimeout.
	FetchTimeout time.Duration
	// the max timeout for fetching tx error messages from the db/network.
	MaxFetchTimeout time.Duration
	// the initial delay used in the exponential backoff for failed tx error messages download
	// retries.
	RetryDelay time.Duration
	// the max delay used in the exponential backoff for failed tx error messages download.
	MaxRetryDelay time.Duration
}

type TransactionErrorMessagesRequester struct {
	core   *TxErrorMessagesCore
	config *TransactionErrorMessagesRequesterConfig
}

func NewTransactionErrorMessagesRequester(
	core *TxErrorMessagesCore,
	config *TransactionErrorMessagesRequesterConfig,
) *TransactionErrorMessagesRequester {
	return &TransactionErrorMessagesRequester{
		core:   core,
		config: config,
	}
}

func (r *TransactionErrorMessagesRequester) RequestTransactionErrorMessagesByBlockID(
	ctx irrecoverable.SignalerContext,
	blockID flow.Identifier,
) error {
	backoff := retry.NewExponential(r.config.RetryDelay)
	backoff = retry.WithCappedDuration(r.config.MaxRetryDelay, backoff)
	backoff = retry.WithJitterPercent(15, backoff)
	timeout := retry.NewExponential(r.config.FetchTimeout)
	timeout = retry.WithCappedDuration(r.config.MaxFetchTimeout, timeout)

	attempt := 0
	return retry.Do(ctx, backoff, func(context.Context) error {
		if attempt > 0 {
			r.core.log.Debug().
				Str("block_id", blockID.String()).
				Uint64("attempt", uint64(attempt)).
				Msgf("retrying download")
		}
		attempt++

		fetchTimeout, _ := timeout.Next()
		ctx, cancel := context.WithTimeout(ctx, fetchTimeout)
		defer cancel()

		err := r.core.HandleTransactionResultErrorMessages(ctx, blockID)

		return retry.RetryableError(err)
	})
}
