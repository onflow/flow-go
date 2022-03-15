package epochs

import (
	"context"
	"fmt"
	"time"

	"github.com/sethvargo/go-retry"

	"github.com/rs/zerolog"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/module"
)

const (
	waitForSealedRetryInterval = 3 * time.Second
	waitForSealedMaxDuration   = 5 * time.Minute
)

// BaseClient represents the core fields and methods needed to create
// a client to a contract on the Flow Network.
type BaseClient struct {
	Log zerolog.Logger // default logger

	ContractAddress string                  // contract address
	FlowClient      module.SDKClientWrapper // flow access node client

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
	contractAddress string,
) *BaseClient {

	return &BaseClient{
		Log:             log,
		ContractAddress: contractAddress,
		FlowClient:      flowClient,
		AccountKeyIndex: accountKeyIndex,
		Signer:          signer,
		AccountAddress:  sdk.HexToAddress(accountAddress),
	}
}

func (c *BaseClient) GetAccount(ctx context.Context) (*sdk.Account, error) {

	// get account from access node for given address
	account, err := c.FlowClient.GetAccount(ctx, c.AccountAddress)
	if err != nil {
		return nil, fmt.Errorf("could not get account: %w", err)
	}

	// check if account key index within range of keys
	if len(account.Keys) <= int(c.AccountKeyIndex) {
		return nil, fmt.Errorf("given account key index is bigger than the number of keys for this account")
	}

	return account, nil
}

// SendTransaction submits a transaction to Flow. Requires transaction to be signed.
func (c *BaseClient) SendTransaction(ctx context.Context, tx *sdk.Transaction) (sdk.Identifier, error) {

	// check if the transaction has a signature
	if len(tx.EnvelopeSignatures) == 0 {
		return sdk.EmptyID, fmt.Errorf("can not submit an unsigned transaction")
	}

	// submit transaction to client
	err := c.FlowClient.SendTransaction(ctx, *tx)
	if err != nil {
		return sdk.EmptyID, fmt.Errorf("failed to send transaction: %w", err)
	}

	return tx.ID(), nil
}

// WaitForSealed waits for a transaction to be sealed
func (c *BaseClient) WaitForSealed(ctx context.Context, txID sdk.Identifier, started time.Time) error {

	log := c.Log.With().Str("tx_id", txID.Hex()).Logger()

	backoff, err := retry.NewConstant(waitForSealedRetryInterval)
	if err != nil {
		c.Log.Fatal().Err(err).Msg("failed to create retry mechanism")
	}
	backoff = retry.WithMaxDuration(waitForSealedMaxDuration, backoff)

	attempts := 0
	err = retry.Do(ctx, backoff, func(ctx context.Context) error {
		attempts++
		log = c.Log.With().Int("attempt", attempts).Float64("time_elapsed_s", time.Since(started).Seconds()).Logger()

		result, err := c.FlowClient.GetTransactionResult(ctx, txID)
		if err != nil {
			msg := "could not get transaction result, retrying"
			log.Error().Err(err).Msg(msg)
			return retry.RetryableError(fmt.Errorf(msg))
		}

		if result.Error != nil {
			return fmt.Errorf("error executing transaction: %w", result.Error)
		}

		log.Info().Str("status", result.Status.String()).Msg("got transaction result")

		// if the transaction has expired we skip waiting for seal
		if result.Status == sdk.TransactionStatusExpired {
			return fmt.Errorf("transaction has expired")
		}

		if result.Status == sdk.TransactionStatusSealed {
			return nil
		}

		return retry.RetryableError(fmt.Errorf("waiting for transaction to be sealed retrying"))
	})
	if err != nil {
		return err
	}

	return nil
}
