package observerclient

import (
	"context"

	"github.com/dapperlabs/flow-go/protobuf/services/observation"
)

var _ observation.ObserveServiceServer = NullObserverClient{}

// NullObserverClient is an implementation of the GRPC ObserveService
// Execution and Collection node provide actual implementations of only selected API calls
// This struct is inherited to provide such selected implementation while returning nil for the other calls
type NullObserverClient struct {
}

func (n NullObserverClient) Ping(context.Context, *observation.PingRequest) (*observation.PingResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetLatestBlockHeader(context.Context, *observation.GetLatestBlockHeaderRequest) (*observation.BlockHeaderResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetBlockHeaderByID(context.Context, *observation.GetBlockHeaderByIDRequest) (*observation.BlockHeaderResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetBlockHeaderByHeight(context.Context, *observation.GetBlockHeaderByHeightRequest) (*observation.BlockHeaderResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetLatestBlock(context.Context, *observation.GetLatestBlockRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetBlockByID(context.Context, *observation.GetBlockByIDRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetBlockByHeight(context.Context, *observation.GetBlockByHeightRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetCollectionByID(context.Context, *observation.GetCollectionByIDRequest) (*observation.CollectionResponse, error) {
	return nil, nil
}

func (n NullObserverClient) SendTransaction(context.Context, *observation.SendTransactionRequest) (*observation.SendTransactionResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetTransaction(context.Context, *observation.GetTransactionRequest) (*observation.TransactionResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetTransactionResult(context.Context, *observation.GetTransactionRequest) (*observation.TransactionResultResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetAccount(context.Context, *observation.GetAccountRequest) (*observation.GetAccountResponse, error) {
	return nil, nil
}

func (n NullObserverClient) ExecuteScript(context.Context, *observation.ExecuteScriptRequest) (*observation.ExecuteScriptResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetEvents(context.Context, *observation.GetEventsRequest) (*observation.GetEventsResponse, error) {
	return nil, nil
}
