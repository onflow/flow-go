package server

import (
	"context"
	"fmt"
	"log"
	"net"

	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/gen/grpc/services/accessv1"
	"github.com/dapperlabs/bamboo-emulator/nodes/access"
)

// Server is a gRPC server that implements the Bamboo Access API.
type Server interface {
	Start(port int)
}

type server struct {
	accessNode access.Node
}

// NewServer returns a new Bamboo emulator server.
func NewServer(accessNode access.Node) Server {
	return &server{
		accessNode: accessNode,
	}
}

// SendTransaction submits a transaction to the network.
func (s *server) SendTransaction(ctx context.Context, req *accessv1.SendTransactionRequest) (*accessv1.SendTransactionResponse, error) {
	txMsg := req.GetTransaction()

	tx := &data.Transaction{
		// TODO: convert address from bytes
		// ToAddress:      txMsg.GetTo(),
		Script:         txMsg.GetScript(),
		Nonce:          txMsg.GetNonce(),
		ComputeLimit:   txMsg.GetCompute(),
		PayerSignature: txMsg.GetPayerSignature(),
		Status:         data.TxPending,
	}

	s.accessNode.SendTransaction(tx)

	return &accessv1.SendTransactionResponse{}, nil
}

// GetBlockByHash gets a block by hash.
func (s *server) GetBlockByHash(ctx context.Context, req *accessv1.GetBlockByHashRequest) (*accessv1.GetBlockByHashResponse, error) {
	var hash data.Hash

	s.accessNode.GetBlockByHash(hash)

	return nil, nil
}

// GetBlockByNumber gets a block by number.
func (s *server) GetBlockByNumber(ctx context.Context, req *accessv1.GetBlockByNumberRequest) (*accessv1.GetBlockByNumberResponse, error) {
	number := req.GetNumber()

	s.accessNode.GetBlockByNumber(number)

	return nil, nil
}

// GetLatestBlock gets the latest sealed block.
func (s *server) GetLatestBlock(context.Context, *accessv1.GetLatestBlockRequest) (*accessv1.GetLatestBlockResponse, error) {
	s.accessNode.GetLatestBlock()

	return nil, nil
}

// GetTransactions gets a transaction by hash.
func (s *server) GetTransaction(context.Context, *accessv1.GetTransactionRequest) (*accessv1.GetTransactionResponse, error) {
	var hash data.Hash

	s.accessNode.GetTransaction(hash)

	return nil, nil
}

// GetBalance returns the balance of an agddress.
func (s *server) GetBalance(context.Context, *accessv1.GetBalanceRequest) (*accessv1.GetBalanceResponse, error) {
	var address data.Address

	s.accessNode.GetBalance(address)

	return nil, nil
}

// CallContract performs a contract call.
func (s *server) CallContract(context.Context, *accessv1.CallContractRequest) (*accessv1.CallContractResponse, error) {
	return nil, nil
}

func (s *server) Start(port int) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	accessv1.RegisterBambooAccessAPIServer(grpcServer, s)

	grpcServer.Serve(lis)
}
