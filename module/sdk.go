package module

import (
	"context"

	"github.com/onflow/cadence"
	"google.golang.org/grpc"

	sdk "github.com/onflow/flow-go-sdk"
)

// SDKClientWrapper is a temporary solution to mocking the `sdk.Client` interface from `flow-go-sdk`
type SDKClientWrapper interface {
	GetAccount(context.Context, sdk.Address, ...grpc.CallOption) (*sdk.Account, error)
	GetAccountAtLatestBlock(context.Context, sdk.Address, ...grpc.CallOption) (*sdk.Account, error)
	SendTransaction(context.Context, sdk.Transaction, ...grpc.CallOption) error
	GetLatestBlock(context.Context, bool, ...grpc.CallOption) (*sdk.Block, error)
	GetTransactionResult(context.Context, sdk.Identifier, ...grpc.CallOption) (*sdk.TransactionResult, error)
	ExecuteScriptAtLatestBlock(context.Context, []byte, []cadence.Value, ...grpc.CallOption) (cadence.Value, error)
	ExecuteScriptAtBlockID(context.Context, sdk.Identifier, []byte, []cadence.Value, ...grpc.CallOption) (cadence.Value, error)
}
