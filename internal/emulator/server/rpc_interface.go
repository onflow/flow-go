package server

import (
	"context"

	"github.com/dapperlabs/bamboo-node/grpc/services/observe"
)

// SendTransaction submits a transaction to the network.
func (s *EmulatorServer) SendTransaction(ctx context.Context, req *observe.SendTransactionRequest) (*observe.SendTransactionResponse, error) {
	return nil, nil
}

// GetTransaction gets a transaction by hash.
func (s *EmulatorServer) GetTransaction(ctx context.Context, req *observe.GetTransactionRequest) (*observe.GetTransactionResponse, error) {
	return nil, nil
}

// GetAccount returns the info associated with an address.
func (s *EmulatorServer) GetAccount(ctx context.Context, req *observe.GetAccountRequest) (*observe.GetAccountResponse, error) {
	return nil, nil
}

// CallContract performs a contract call.
func (s *EmulatorServer) CallContract(ctx context.Context, req *observe.CallContractRequest) (*observe.CallContractResponse, error) {
	return nil, nil
}
