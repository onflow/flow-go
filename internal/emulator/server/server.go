package server

import (
	"context"
	"fmt"
	"log"
	"net"

	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/bamboo-node/grpc/services/observe"
	"github.com/dapperlabs/bamboo-node/internal/emulator/data"
	"github.com/dapperlabs/bamboo-node/internal/emulator/nodes/access"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
)

// Server is a gRPC server that implements the Bamboo Access API.
type Server struct {
	accessNode *access.Node
}

// NewServer returns a new Bamboo emulator server.
func NewServer(accessNode *access.Node) *Server {
	return &Server{
		accessNode: accessNode,
	}
}

// SendTransaction submits a transaction to the network.
func (s *Server) SendTransaction(ctx context.Context, req *observe.SendTransactionRequest) (*observe.SendTransactionResponse, error) {
	txMsg := req.GetTransaction()

	payerSig := txMsg.GetPayerSignature()

	tx := &data.Transaction{
		ToAddress:    crypto.BytesToAddress(txMsg.GetToAddress()),
		Script:       txMsg.GetScript(),
		Nonce:        txMsg.GetNonce(),
		ComputeLimit: txMsg.GetComputeLimit(),
		PayerSignature: &crypto.Signature{
			Account: crypto.BytesToAddress(payerSig.GetAccountAddress()),
			Sig:     payerSig.GetSignature(),
		},
		Status: data.TxPending,
	}

	err := s.accessNode.SendTransaction(tx)
	if err != nil {
		switch err.(type) {
		case *access.DuplicateTransactionError:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &observe.SendTransactionResponse{
		Hash: tx.Hash().Bytes(),
	}, nil
}

// GetBlockByHash gets a block by hash.
func (s *Server) GetBlockByHash(ctx context.Context, req *observe.GetBlockByHashRequest) (*observe.GetBlockByHashResponse, error) {
	hash := crypto.BytesToHash(req.GetHash())

	block, err := s.accessNode.GetBlockByHash(hash)
	if err != nil {
		switch err.(type) {
		case *access.BlockNotFoundByHashError:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	timestamp, err := ptypes.TimestampProto(block.Timestamp)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &observe.GetBlockByHashResponse{
		Block: &observe.Block{
			Hash:              block.Hash().Bytes(),
			PrevBlockHash:     block.PrevBlockHash.Bytes(),
			Number:            block.Number,
			Timestamp:         timestamp,
			TransactionHashes: crypto.HashesToBytes(block.TransactionHashes),
			Status:            observe.Block_Status(block.Status),
		},
	}, nil
}

// GetBlockByNumber gets a block by number.
func (s *Server) GetBlockByNumber(ctx context.Context, req *observe.GetBlockByNumberRequest) (*observe.GetBlockByNumberResponse, error) {
	number := req.GetNumber()

	block, err := s.accessNode.GetBlockByNumber(number)
	if err != nil {
		switch err.(type) {
		case *access.BlockNotFoundByNumberError:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	timestamp, err := ptypes.TimestampProto(block.Timestamp)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &observe.GetBlockByNumberResponse{
		Block: &observe.Block{
			Hash:              block.Hash().Bytes(),
			PrevBlockHash:     block.PrevBlockHash.Bytes(),
			Number:            block.Number,
			Timestamp:         timestamp,
			TransactionHashes: crypto.HashesToBytes(block.TransactionHashes),
			Status:            observe.Block_Status(block.Status),
		},
	}, nil
}

// GetLatestBlock gets the latest sealed block.
func (s *Server) GetLatestBlock(ctx context.Context, req *observe.GetLatestBlockRequest) (*observe.GetLatestBlockResponse, error) {
	block := s.accessNode.GetLatestBlock()

	timestamp, err := ptypes.TimestampProto(block.Timestamp)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &observe.GetLatestBlockResponse{
		Block: &observe.Block{
			Hash:              block.Hash().Bytes(),
			PrevBlockHash:     block.PrevBlockHash.Bytes(),
			Number:            block.Number,
			Timestamp:         timestamp,
			TransactionHashes: crypto.HashesToBytes(block.TransactionHashes),
			Status:            observe.Block_Status(block.Status),
		},
	}, nil
}

// GetTransactions gets a transaction by hash.
func (s *Server) GetTransaction(ctx context.Context, req *observe.GetTransactionRequest) (*observe.GetTransactionResponse, error) {
	hash := crypto.BytesToHash(req.GetHash())

	tx, err := s.accessNode.GetTransaction(hash)
	if err != nil {
		switch err.(type) {
		case *access.TransactionNotFoundError:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &observe.GetTransactionResponse{
		Transaction: &observe.GetTransactionResponse_Transaction{
			ToAddress:    tx.ToAddress.Bytes(),
			Script:       tx.Script,
			Nonce:        tx.Nonce,
			ComputeLimit: tx.ComputeLimit,
			ComputeUsed:  tx.ComputeUsed,
			PayerSignature: &observe.Signature{
				AccountAddress: tx.PayerSignature.Account.Bytes(),
				Signature:      tx.PayerSignature.Sig,
			},
			Status: observe.GetTransactionResponse_Transaction_Status(tx.Status),
		},
	}, nil
}

// GetAccount returns the balance of an address.
func (s *Server) GetAccount(ctx context.Context, req *observe.GetAccountRequest) (*observe.GetAccountResponse, error) {
	address := crypto.BytesToAddress(req.GetAddress())

	account, err := s.accessNode.GetAccount(address)
	if err != nil {
		switch err.(type) {
		case *access.AccountNotFoundError:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return &observe.GetAccountResponse{
		Account: &observe.GetAccountResponse_Account{
			Address:    account.Address.Bytes(),
			Balance:    account.Balance,
			Code:       account.Code,
			PublicKeys: account.PublicKeys,
		},
	}, nil
}

// CallContract performs a contract call.
func (s *Server) CallContract(context.Context, *observe.CallContractRequest) (*observe.CallContractResponse, error) {
	// TODO: implement CallContract
	return nil, nil
}

func (s *Server) Start(port int) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()

	observe.RegisterObserveServiceServer(grpcServer, s)

	grpcServer.Serve(lis)
}
