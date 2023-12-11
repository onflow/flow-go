package epochs

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sethvargo/go-retry"

	"github.com/rs/zerolog"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/network"

	"github.com/onflow/flow-go/module"
)

const (
	waitForSealedRetryInterval = 3 * time.Second
	waitForSealedMaxDuration   = 5 * time.Minute
)

var (
	// errTransactionExpired is returned when a transaction expires before it is incorporated into a block.
	errTransactionExpired = errors.New("transaction expired")
	// errTransactionReverted is returned when a transaction is executed, but the execution reverts.
	errTransactionReverted = errors.New("transaction execution reverted")
)

// BaseClient represents the core fields and methods needed to create
// a client to a contract on the Flow Network.
type BaseClient struct {
	Log zerolog.Logger // default logger

	FlowClient module.SDKClientWrapper // flow access node client

	AccountAddress  sdk.Address      // account belonging to node interacting with the contract
	AccountKeyIndex uint             // account key index
	Signer          sdkcrypto.Signer // signer used to sign transactions
}

// NewBaseClient creates a instance of BaseClient
func NewBaseClient(
	log zerolog.Logger,
	flowClient module.SDKClientWrapper,
	accountAddress string,
	accountKeyIndex uint,
	signer sdkcrypto.Signer,
) *BaseClient {

	return &BaseClient{
		Log:             log,
		FlowClient:      flowClient,
		AccountKeyIndex: accountKeyIndex,
		Signer:          signer,
		AccountAddress:  sdk.HexToAddress(accountAddress),
	}
}

// GetAccount returns the current state for the account associated with the BaseClient.
// Error returns:
//   - network.TransientError for any errors from the underlying client
//   - generic error in case of unexpected critical failure
func (c *BaseClient) GetAccount(ctx context.Context) (*sdk.Account, error) {

	// get account from access node for given address
	account, err := c.FlowClient.GetAccount(ctx, c.AccountAddress)
	if err != nil {
		// we consider all errors from client network calls to be transient and non-critical
		return nil, network.NewTransientErrorf("could not get account: %w", err)
	}

	// check if account key index within range of keys
	if int(c.AccountKeyIndex) >= len(account.Keys) {
		return nil, fmt.Errorf("given account key index exceeds the number of keys for this account (%d>=%d)",
			c.AccountKeyIndex, len(account.Keys))
	}

	return account, nil
}

// SendTransaction submits a transaction to Flow. Requires transaction to be signed.
// Error returns:
//   - network.TransientError for any errors from the underlying client
//   - generic error in case of unexpected critical failure
func (c *BaseClient) SendTransaction(ctx context.Context, tx *sdk.Transaction) (sdk.Identifier, error) {

	// check if the transaction has a signature
	if len(tx.EnvelopeSignatures) == 0 {
		return sdk.EmptyID, fmt.Errorf("can not submit an unsigned transaction")
	}

	// submit transaction to client
	err := c.FlowClient.SendTransaction(ctx, *tx)
	if err != nil {
		// we consider all errors from client network calls to be transient and non-critical
		return sdk.EmptyID, network.NewTransientErrorf("failed to send transaction: %w", err)
	}

	return tx.ID(), nil
}

// WaitForSealed waits for a transaction to be sealed
// Error returns:
//   - network.TransientError for any errors from the underlying client, if the retry period has been exceeded
//   - errTransactionExpired if the transaction has expired
//   - errTransactionReverted if the transaction execution reverted
//   - generic error in case of unexpected critical failure
func (c *BaseClient) WaitForSealed(ctx context.Context, txID sdk.Identifier, started time.Time) error {

	log := c.Log.With().Str("tx_id", txID.Hex()).Logger()

	backoff := retry.NewConstant(waitForSealedRetryInterval)
	backoff = retry.WithMaxDuration(waitForSealedMaxDuration, backoff)

	attempts := 0
	err := retry.Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		log = c.Log.With().Int("attempt", attempts).Float64("time_elapsed_s", time.Since(started).Seconds()).Logger()

		result, err := c.FlowClient.GetTransactionResult(ctx, txID)
		if err != nil {
			// we consider all errors from client network calls to be transient and non-critical
			err = network.NewTransientErrorf("could not get transaction result: %w", err)
			log.Err(err).Msg("retrying getting transaction result...")
			return retry.RetryableError(err)
		}

		if result.Error != nil {
			return fmt.Errorf("transaction reverted with error=[%s]: %w", result.Error.Error(), errTransactionReverted)
		}

		log.Info().Str("status", result.Status.String()).Msg("got transaction result")

		// if the transaction has expired we skip waiting for seal
		if result.Status == sdk.TransactionStatusExpired {
			return errTransactionExpired
		}

		if result.Status == sdk.TransactionStatusSealed {
			return nil
		}

		return retry.RetryableError(network.NewTransientErrorf("transaction not sealed yet (status=%s)", result.Status))
	})
	if err != nil {
		return fmt.Errorf("transaction (id=%s) failed to be sealed successfully after %s: %w", txID.String(), time.Since(started), err)
	}

	return nil
}
