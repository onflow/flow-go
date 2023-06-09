package utils

import (
	"context"
	"fmt"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-emulator/adapters"
	emulator "github.com/onflow/flow-emulator/emulator"
	"github.com/rs/zerolog"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/templates"
	"github.com/onflow/flow-go/model/flow"
)

// EmulatorClient is a wrapper around the emulator to implement the same interface
// used by the SDK client. Used for testing against the emulator.
type EmulatorClient struct {
	adapter *adapters.SDKAdapter
}

func NewEmulatorClient(blockchain emulator.Emulator) *EmulatorClient {
	logger := zerolog.Nop()

	adapter := adapters.NewSDKAdapter(&logger, blockchain)
	client := &EmulatorClient{
		adapter: adapter,
	}
	return client
}

func (c *EmulatorClient) GetAccount(ctx context.Context, address sdk.Address) (*sdk.Account, error) {
	return c.adapter.GetAccount(ctx, address)
}

func (c *EmulatorClient) GetAccountAtLatestBlock(ctx context.Context, address sdk.Address) (*sdk.Account, error) {
	return c.adapter.GetAccount(ctx, address)
}

func (c *EmulatorClient) SendTransaction(ctx context.Context, tx sdk.Transaction) error {
	_, err := c.Submit(&tx)
	return err
}

func (c *EmulatorClient) GetLatestBlock(ctx context.Context, isSealed bool) (*sdk.Block, error) {
	block, _, err := c.adapter.GetLatestBlock(ctx, true)
	if err != nil {
		return nil, err
	}
	sdkBlock := &sdk.Block{
		BlockHeader: sdk.BlockHeader{ID: block.ID},
	}

	return sdkBlock, nil
}

func (c *EmulatorClient) GetTransactionResult(ctx context.Context, txID sdk.Identifier) (*sdk.TransactionResult, error) {
	return c.adapter.GetTransactionResult(ctx, txID)
}

func (c *EmulatorClient) ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, args []cadence.Value) (cadence.Value, error) {

	arguments := [][]byte{}
	for _, arg := range args {
		val, err := jsoncdc.Encode(arg)
		if err != nil {
			return nil, fmt.Errorf("could not encode arguments: %w", err)
		}
		arguments = append(arguments, val)
	}

	scriptResult, err := c.adapter.ExecuteScriptAtLatestBlock(ctx, script, arguments)
	if err != nil {
		return nil, err
	}

	value, err := jsoncdc.Decode(nil, scriptResult)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func (c *EmulatorClient) ExecuteScriptAtBlockID(ctx context.Context, blockID sdk.Identifier, script []byte, args []cadence.Value) (cadence.Value, error) {

	arguments := [][]byte{}
	for _, arg := range args {
		val, err := jsoncdc.Encode(arg)
		if err != nil {
			return nil, fmt.Errorf("could not encode arguments: %w", err)
		}
		arguments = append(arguments, val)
	}

	// get block by ID
	block, _, err := c.adapter.GetBlockByID(ctx, blockID)
	if err != nil {
		return nil, err
	}

	scriptResult, err := c.adapter.ExecuteScriptAtBlockHeight(ctx, block.BlockHeader.Height, script, arguments)

	if err != nil {
		return nil, fmt.Errorf("error in script: %w", err)
	}

	value, err := jsoncdc.Decode(nil, scriptResult)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func (c *EmulatorClient) CreateAccount(keys []*sdk.AccountKey, contracts []templates.Contract) (sdk.Address, error) {
	return c.adapter.CreateAccount(context.Background(), keys, contracts)

}

func (c *EmulatorClient) Submit(tx *sdk.Transaction) (*flow.Block, error) {
	// submit the signed transaction
	err := c.adapter.SendTransaction(context.Background(), *tx)
	if err != nil {
		return nil, err
	}

	block, _, err := c.adapter.Emulator().ExecuteAndCommitBlock()
	if err != nil {
		return nil, err
	}

	return block, nil
}
