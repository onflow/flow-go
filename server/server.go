package server

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-emulator/gen/grpc/services/accessv1"
)

type bambooAccessAPIServer struct{}

// SendTransaction submits a transaction to the network.
func (s *bambooAccessAPIServer) SendTransaction(context.Context, *accessv1.SendTransactionRequest) (*accessv1.SendTransactionResponse, error) {
	return nil, nil
}

// GetBlockByHash gets a block by hash.
func (s *bambooAccessAPIServer) GetBlockByHash(context.Context, *accessv1.GetBlockByHashRequest) (*accessv1.GetBlockByHashResponse, error) {
	return nil, nil
}

// GetBlockByNumber gets a block by number.
func (s *bambooAccessAPIServer) GetBlockByNumber(context.Context, *accessv1.GetBlockByNumberRequest) (*accessv1.GetBlockByNumberResponse, error) {
	return nil, nil
}

// GetLatestBlock gets the latest sealed block.
func (s *bambooAccessAPIServer) GetLatestBlock(context.Context, *accessv1.GetLatestBlockRequest) (*accessv1.GetLatestBlockResponse, error) {
	return nil, nil
}

// GetTransactions gets a transaction by hash.
func (s *bambooAccessAPIServer) GetTransaction(context.Context, *accessv1.GetTransactionRequest) (*accessv1.GetTransactionResponse, error) {
	return nil, nil
}

// GetBalance returns the balance of an address.
func (s *bambooAccessAPIServer) GetBalance(context.Context, *accessv1.GetBalanceRequest) (*accessv1.GetBalanceResponse, error) {
	return nil, nil
}

// CallContract performs a contract call.
func (s *bambooAccessAPIServer) CallContract(context.Context, *accessv1.CallContractRequest) (*accessv1.CallContractResponse, error) {
	return nil, nil
}

// Start runs the Bamboo Access API gRPC server.
func Start(port int) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	accessv1.RegisterBambooAccessAPIServer(grpcServer, &bambooAccessAPIServer{})

	grpcServer.Serve(lis)
}
