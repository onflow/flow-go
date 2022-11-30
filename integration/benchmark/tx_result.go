package benchmark

import (
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-retry"

	flowsdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/access"
)

// WaitForTransactionResult waits for the transaction to get into the terminal state and returns the result.
func WaitForTransactionResult(ctx context.Context, client access.Client, txID flowsdk.Identifier, requireSealed bool) (*flowsdk.TransactionResult, error) {
	var b retry.Backoff
	b = retry.NewConstant(500 * time.Millisecond)
	b = retry.WithMaxDuration(60*time.Second, b)

	var result *flowsdk.TransactionResult
	err := retry.Do(ctx, b, func(ctx context.Context) (err error) {
		result, err = client.GetTransactionResult(ctx, txID)
		if err != nil {
			return err
		}
		if result.Error != nil {
			return result.Error
		}

		switch result.Status {
		case flowsdk.TransactionStatusExecuted:
			if requireSealed {
				return retry.RetryableError(fmt.Errorf("waiting for transaction sealed: %s", txID))
			}
			return nil
		case flowsdk.TransactionStatusSealed:
			return nil
		case flowsdk.TransactionStatusPending, flowsdk.TransactionStatusFinalized:
			return retry.RetryableError(fmt.Errorf("waiting for transaction execution: %s", txID))
		default:
			return fmt.Errorf("unexpected transaction status: %s", result.Status)
		}
	})
	if err != nil {
		return &flowsdk.TransactionResult{Error: err}, err
	}
	return result, result.Error
}
