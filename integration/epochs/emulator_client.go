package epochs

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	emulator "github.com/onflow/flow-emulator"
	sdk "github.com/onflow/flow-go-sdk"
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
	return c.Submit(&tx)
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

func (c *EmulatorClient) Submit(tx *sdk.Transaction) error {
	// submit the signed transaction
	err := c.blockchain.AddTransaction(*tx)
	if err != nil {
		return err
	}

	result, err := c.blockchain.ExecuteNextTransaction()
	if err != nil {
		return err
	}

	if !result.Succeeded() {
		return fmt.Errorf("transaction did not succeeded")
	}

	_, err = c.blockchain.CommitBlock()
	if err != nil {
		return err
	}

	return nil
}
