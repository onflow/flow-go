package server

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/bamboo-node/grpc/services/observe"
)

func (s *EmulatorServer) Ping(ctx context.Context, req *observe.PingRequest) (*observe.PingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// SendTransaction submits a transaction to the network.
func (s *EmulatorServer) SendTransaction(ctx context.Context, req *observe.SendTransactionRequest) (*observe.SendTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetBlockByHash gets a block by hash.
func (s *EmulatorServer) GetBlockByHash(ctx context.Context, req *observe.GetBlockByHashRequest) (*observe.GetBlockByHashResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetBlockByNumber gets a block by number.
func (s *EmulatorServer) GetBlockByNumber(ctx context.Context, req *observe.GetBlockByNumberRequest) (*observe.GetBlockByNumberResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetLatestBlock gets the latest sealed block.
func (s *EmulatorServer) GetLatestBlock(ctx context.Context, req *observe.GetLatestBlockRequest) (*observe.GetLatestBlockResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetTransaction gets a transaction by hash.
func (s *EmulatorServer) GetTransaction(ctx context.Context, req *observe.GetTransactionRequest) (*observe.GetTransactionResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// GetAccount returns the info associated with an address.
func (s *EmulatorServer) GetAccount(ctx context.Context, req *observe.GetAccountRequest) (*observe.GetAccountResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// CallContract performs a contract call.
func (s *EmulatorServer) CallContract(ctx context.Context, req *observe.CallContractRequest) (*observe.CallContractResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}
