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

func (n NullObserverClient) GetLatestBlock(context.Context, *observation.GetLatestBlockRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetBlockByID(context.Context, *observation.GetBlockByIDRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetBlockByHeight(context.Context, *observation.GetBlockByHeightRequest) (*observation.BlockResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetLatestBlockDetails(context.Context, *observation.GetLatestBlockDetailsRequest) (*observation.BlockDetailsResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetBlockDetailsByID(context.Context, *observation.GetBlockDetailsByIDRequest) (*observation.BlockDetailsResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetBlockDetailsByHeight(context.Context, *observation.GetBlockDetailsByHeightRequest) (*observation.BlockDetailsResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetCollectionByID(context.Context, *observation.GetCollectionByIDRequest) (*observation.GetCollectionResponse, error) {
	return nil, nil
}

func (n NullObserverClient) SendTransaction(context.Context, *observation.SendTransactionRequest) (*observation.SendTransactionResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetTransaction(context.Context, *observation.GetTransactionRequest) (*observation.GetTransactionResponse, error) {
	return nil, nil
}

func (n NullObserverClient) GetTransactionResult(context.Context, *observation.GetTransactionRequest) (*observation.GetTransactionResultResponse, error) {
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
