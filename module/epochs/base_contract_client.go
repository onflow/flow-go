package epochs

import (
	"context"
	"fmt"
	"time"

	"github.com/rs/zerolog"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/module"
)

// BaseContractClient represents the core fields and methods needed to create
// a client to a contract on the Flow Network.
type BaseContractClient struct {
	log zerolog.Logger // default logger

	ContractAddress string           // contract address
	AccountKeyIndex uint             // account key index
	Signer          sdkcrypto.Signer // signer used to sign transactions
	Account         *sdk.Account     // account belonging to node interacting with the contract

	Client module.SDKClientWrapper // flow access node client
}

// NewBaseContractClient ...
func NewBaseContractClient(log zerolog.Logger,
	flowClient module.SDKClientWrapper,
	accountAddress string,
	accountKeyIndex uint,
	signer sdkcrypto.Signer,
	contractAddress string) (*BaseContractClient, error) {

	// get account from access node for given address
	account, err := flowClient.GetAccount(context.Background(), sdk.HexToAddress(accountAddress))
	if err != nil {
		return nil, fmt.Errorf("could not get account: %w", err)
	}

	// check if account key index within range of keys
	if len(account.Keys) <= int(accountKeyIndex) {
		return nil, fmt.Errorf("given account key index is bigger than the number of keys for this account")
	}

	return &BaseContractClient{
		ContractAddress: contractAddress,
		Client:          flowClient,
		AccountKeyIndex: accountKeyIndex,
		Signer:          signer,
		Account:         account,
	}, nil
}

// SendTransaction submits a transaction to Flow. Requires transaction to be signed.
func (c *BaseContractClient) SendTransaction(ctx context.Context, tx *sdk.Transaction) (sdk.Identifier, error) {

	// check if the transaction has a signature
	if len(tx.EnvelopeSignatures) == 0 {
		return sdk.EmptyID, fmt.Errorf("can not submit an unsigned transaction")
	}

	// submit trnsaction to client
	err := c.Client.SendTransaction(ctx, *tx)
	if err != nil {
		return sdk.EmptyID, fmt.Errorf("failed to send transaction: %w", err)
	}

	return tx.ID(), nil
}

// WaitForSealed waits for a transaction to be sealed
func (c *BaseContractClient) WaitForSealed(ctx context.Context, txID sdk.Identifier, started time.Time) error {

	attempts := 1
	for {
		log := c.log.With().Int("attempt", attempts).Float64("time_elapsed", time.Since(started).Seconds()).Logger()

		result, err := c.Client.GetTransactionResult(ctx, txID)
		if err != nil {
			log.Error().Err(err).Msg("could not get transaction result")
			continue
		}
		if result.Error != nil {
			return fmt.Errorf("error executing transaction: %w", result.Error)
		}
		log.Info().Str("status", result.Status.String()).Msg("got transaction result")

		// if the transaction has expired we skip waiting for seal
		if result.Status == sdk.TransactionStatusExpired {
			return fmt.Errorf("broadcast dkg message transaction has expired")
		}

		if result.Status == sdk.TransactionStatusSealed {
			return nil
		}

		attempts++
		time.Sleep(TransactionStatusRetryTimeout)
	}
}
