package module

import (
	"context"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
)

// SDKClientWrapper is a temporary solution to mocking the `sdk.Client` interface from `flow-go-sdk`
type SDKClientWrapper interface {
	GetAccount(context.Context, sdk.Address) (*sdk.Account, error)
	GetAccountAtLatestBlock(context.Context, sdk.Address) (*sdk.Account, error)
	SendTransaction(context.Context, sdk.Transaction) error
	GetLatestBlock(context.Context, bool) (*sdk.Block, error)
	GetTransactionResult(context.Context, sdk.Identifier) (*sdk.TransactionResult, error)
	ExecuteScriptAtLatestBlock(context.Context, []byte, []cadence.Value) (cadence.Value, error)
	ExecuteScriptAtBlockID(context.Context, sdk.Identifier, []byte, []cadence.Value) (cadence.Value, error)
}
