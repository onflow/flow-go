package module

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	emulator "github.com/onflow/flow-emulator"

	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go/model/flow"
)

// EmulatorClient is a wrapper around the emulator to implement the same interface
// used by the SDK client. Used for testing against the emulator.
type EmulatorClient struct {
	blockchain *emulator.Blockchain
}

func NewEmulatorClient(blockchain *emulator.Blockchain) *EmulatorClient {
	client := &EmulatorClient{
		blockchain: blockchain,
	}
	return client
}

func (c *EmulatorClient) GetAccount(ctx context.Context, address sdk.Address, opts ...grpc.CallOption) (*sdk.Account, error) {
	return c.blockchain.GetAccount(address)
}

func (c *EmulatorClient) GetAccountAtLatestBlock(ctx context.Context, address sdk.Address, opts ...grpc.CallOption) (*sdk.Account, error) {
	return c.blockchain.GetAccount(address)
}

func (c *EmulatorClient) SendTransaction(ctx context.Context, tx sdk.Transaction, opts ...grpc.CallOption) error {
	_, err := c.Submit(&tx)
	return err
}

func (c *EmulatorClient) GetLatestBlock(ctx context.Context, isSealed bool, opts ...grpc.CallOption) (*sdk.Block, error) {
	block, err := c.blockchain.GetLatestBlock()
	if err != nil {
		return nil, err
	}
	blockID := block.ID()

	var id sdk.Identifier
	copy(id[:], blockID[:])

	sdkBlock := &sdk.Block{
		BlockHeader: sdk.BlockHeader{ID: id},
	}

	return sdkBlock, nil
}

func (c *EmulatorClient) GetTransactionResult(ctx context.Context, txID sdk.Identifier, opts ...grpc.CallOption) (*sdk.TransactionResult, error) {
	return c.blockchain.GetTransactionResult(txID)
}

func (c *EmulatorClient) ExecuteScriptAtLatestBlock(ctx context.Context, script []byte, args []cadence.Value, opts ...grpc.CallOption) (cadence.Value, error) {

	arguments := [][]byte{}
	for _, arg := range args {
		val, err := jsoncdc.Encode(arg)
		if err != nil {
			return nil, fmt.Errorf("could not encode arguments: %w", err)
		}
		arguments = append(arguments, val)
	}

	scriptResult, err := c.blockchain.ExecuteScript(script, arguments)
	if err != nil {
		return nil, err
	}

	return scriptResult.Value, nil
}

func (c *EmulatorClient) ExecuteScriptAtBlockID(ctx context.Context, blockID sdk.Identifier, script []byte, args []cadence.Value, opts ...grpc.CallOption) (cadence.Value, error) {

	arguments := [][]byte{}
	for _, arg := range args {
		val, err := jsoncdc.Encode(arg)
		if err != nil {
			return nil, fmt.Errorf("could not encode arguments: %w", err)
		}
		arguments = append(arguments, val)
	}

	// get block by ID
	block, err := c.blockchain.GetBlockByID(blockID)
	if err != nil {
		return nil, err
	}

	scriptResult, err := c.blockchain.ExecuteScriptAtBlock(script, arguments, block.Header.Height)
	if err != nil {
		return nil, err
	}

	if scriptResult.Error != nil {
		return nil, fmt.Errorf("error in script: %w", scriptResult.Error)
	}

	return scriptResult.Value, nil
}

func (c *EmulatorClient) Submit(tx *sdk.Transaction) (*flow.Block, error) {
	// submit the signed transaction
	err := c.blockchain.AddTransaction(*tx)
	if err != nil {
		return nil, err
	}

	block, _, err := c.blockchain.ExecuteAndCommitBlock()
	if err != nil {
		return nil, err
	}

	return block, nil
}
