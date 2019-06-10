package server

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-emulator/gen/grpc/access/apiv1"
)

type bambooAccessAPIServer struct{}

// SendTransaction submits a transaction to the network.
func (s *bambooAccessAPIServer) SendTransaction(context.Context, *apiv1.SendTransactionRequest) (*apiv1.SendTransactionResponse, error) {
	return nil, nil
}

// GetBlockByHash gets a block by hash.
func (s *bambooAccessAPIServer) GetBlockByHash(context.Context, *apiv1.GetBlockByHashRequest) (*apiv1.GetBlockByHashResponse, error) {
	return nil, nil
}

// GetBlockByNumber gets a block by number.
func (s *bambooAccessAPIServer) GetBlockByNumber(context.Context, *apiv1.GetBlockByNumberRequest) (*apiv1.GetBlockByNumberResponse, error) {
	return nil, nil
}

// GetLatestBlock gets the latest sealed block.
func (s *bambooAccessAPIServer) GetLatestBlock(context.Context, *apiv1.GetLatestBlockRequest) (*apiv1.GetLatestBlockResponse, error) {
	return nil, nil
}

// GetTransactions gets a transaction by hash.
func (s *bambooAccessAPIServer) GetTransaction(context.Context, *apiv1.GetTransactionRequest) (*apiv1.GetTransactionResponse, error) {
	return nil, nil
}

// GetBalance returns the balance of an address.
func (s *bambooAccessAPIServer) GetBalance(context.Context, *apiv1.GetBalanceRequest) (*apiv1.GetBalanceResponse, error) {
	return nil, nil
}

// CallContract performs a contract call.
func (s *bambooAccessAPIServer) CallContract(context.Context, *apiv1.CallContractRequest) (*apiv1.CallContractResponse, error) {
	return nil, nil
}

func Start(port int) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	apiv1.RegisterBambooAccessAPIServer(grpcServer, &bambooAccessAPIServer{})

	grpcServer.Serve(lis)
}
