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

type Server interface {
	Start(port int)
}

func NewServer(accessNode access.Node) Server {
	return &server{
		accessNode: accessNode,
	}
}

type server struct {
	accessNode access.Node
}

// SendTransaction submits a transaction to the network.
func (s *server) SendTransaction(ctx context.Context, req *accessv1.SendTransactionRequest) (*accessv1.SendTransactionResponse, error) {
	txMsg := req.GetTransaction()

	tx := &data.Transaction{
		Status: data.TxPending,
		// TODO: convert address from bytes
		// ToAddress:      txMsg.GetTo(),
		TxData:         txMsg.GetScript(),
		Nonce:          txMsg.GetNonce(),
		PayerSignature: txMsg.GetPayerSignature(),
	}

	s.accessNode.SubmitTransaction(tx)

	return &accessv1.SendTransactionResponse{}, nil
}

// GetBlockByHash gets a block by hash.
func (s *server) GetBlockByHash(context.Context, *accessv1.GetBlockByHashRequest) (*accessv1.GetBlockByHashResponse, error) {
	return nil, nil
}

// GetBlockByNumber gets a block by number.
func (s *server) GetBlockByNumber(context.Context, *accessv1.GetBlockByNumberRequest) (*accessv1.GetBlockByNumberResponse, error) {
	return nil, nil
}

// GetLatestBlock gets the latest sealed block.
func (s *server) GetLatestBlock(context.Context, *accessv1.GetLatestBlockRequest) (*accessv1.GetLatestBlockResponse, error) {
	return nil, nil
}

// GetTransactions gets a transaction by hash.
func (s *server) GetTransaction(context.Context, *accessv1.GetTransactionRequest) (*accessv1.GetTransactionResponse, error) {
	return nil, nil
}

// GetBalance returns the balance of an agddress.
func (s *server) GetBalance(context.Context, *accessv1.GetBalanceRequest) (*accessv1.GetBalanceResponse, error) {
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
