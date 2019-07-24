package server

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/bamboo-node/grpc/services/observe"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
)

// Ping pings the Observation API server for a response.
func (s *EmulatorServer) Ping(ctx context.Context, req *observe.PingRequest) (*observe.PingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// SendTransaction submits a transaction to the network.
func (s *EmulatorServer) SendTransaction(ctx context.Context, req *observe.SendTransactionRequest) (*observe.SendTransactionResponse, error) {
	txMsg := req.GetTransaction()
	payerSig := txMsg.GetPayerSignature()

	tx := &types.SignedTransaction{
		Script:       txMsg.GetScript(),
		Nonce:        txMsg.GetNonce(),
		ComputeLimit: txMsg.GetComputeLimit(),
		Timestamp:    time.Now(),
		PayerSignature: crypto.Signature{
			Account: crypto.BytesToAddress(payerSig.GetAccountAddress()),
			// TODO: update this (default signature for now)
			Sig: crypto.Sig{},
		},
		Status: types.TransactionPending,
	}

	s.transactionsIn <- tx
	response := &observe.SendTransactionResponse{
		Hash: tx.Hash().Bytes(),
	}

	return response, nil
}

// GetBlockByHash gets a block by hash.
func (s *EmulatorServer) GetBlockByHash(ctx context.Context, req *observe.GetBlockByHashRequest) (*observe.GetBlockByHashResponse, error) {
	hash := crypto.BytesToHash(req.GetHash())
	block := s.blockchain.GetBlockByHash(hash)
	timestamp, _ := ptypes.TimestampProto(block.Timestamp)

	blockMsg := &observe.Block{
		Hash:              block.Hash().Bytes(),
		Number:            block.Height,
		PrevBlockHash:     block.PreviousBlockHash.Bytes(),
		Timestamp:         timestamp,
		TransactionHashes: crypto.HashesToBytes(block.TransactionHashes),
	}

	response := &observe.GetBlockByHashResponse{
		Block: blockMsg,
	}

	return response, nil
}

// GetBlockByNumber gets a block by number.
func (s *EmulatorServer) GetBlockByNumber(ctx context.Context, req *observe.GetBlockByNumberRequest) (*observe.GetBlockByNumberResponse, error) {
	height := req.GetNumber()
	block := s.blockchain.GetBlockByHeight(height)
	timestamp, _ := ptypes.TimestampProto(block.Timestamp)

	blockMsg := &observe.Block{
		Hash:              block.Hash().Bytes(),
		Number:            block.Height,
		PrevBlockHash:     block.PreviousBlockHash.Bytes(),
		Timestamp:         timestamp,
		TransactionHashes: crypto.HashesToBytes(block.TransactionHashes),
	}

	response := &observe.GetBlockByNumberResponse{
		Block: blockMsg,
	}

	return response, nil

}

// GetLatestBlock gets the latest sealed block.
func (s *EmulatorServer) GetLatestBlock(ctx context.Context, req *observe.GetLatestBlockRequest) (*observe.GetLatestBlockResponse, error) {
	block := s.blockchain.GetLatestBlock()
	timestamp, _ := ptypes.TimestampProto(block.Timestamp)

	blockMsg := &observe.Block{
		Hash:              block.Hash().Bytes(),
		Number:            block.Height,
		PrevBlockHash:     block.PreviousBlockHash.Bytes(),
		Timestamp:         timestamp,
		TransactionHashes: crypto.HashesToBytes(block.TransactionHashes),
	}

	response := &observe.GetLatestBlockResponse{
		Block: blockMsg,
	}

	return response, nil
}

// GetTransaction gets a transaction by hash.
func (s *EmulatorServer) GetTransaction(ctx context.Context, req *observe.GetTransactionRequest) (*observe.GetTransactionResponse, error) {
	hash := crypto.BytesToHash(req.GetHash())
	tx := s.blockchain.GetTransaction(hash)

	txMsg := &observe.GetTransactionResponse_Transaction{
		Script:       tx.Script,
		Nonce:        tx.Nonce,
		ComputeLimit: tx.ComputeLimit,
		ComputeUsed:  tx.ComputeUsed,
		PayerSignature: &observe.Signature{
			AccountAddress: tx.PayerSignature.Account.Bytes(),
			// TODO: update this (default signature bytes for now)
			Signature: tx.PayerSignature.Sig[:],
		},
		Status: observe.GetTransactionResponse_Transaction_Status(tx.Status),
	}

	response := &observe.GetTransactionResponse{
		Transaction: txMsg,
	}

	return response, nil
}

// GetAccount returns the info associated with an address.
func (s *EmulatorServer) GetAccount(ctx context.Context, req *observe.GetAccountRequest) (*observe.GetAccountResponse, error) {
	address := crypto.BytesToAddress(req.GetAddress())
	account := s.blockchain.GetAccount(address)

	accMsg := &observe.GetAccountResponse_Account{
		Address:    account.Address.Bytes(),
		Balance:    account.Balance,
		Code:       account.Code,
		PublicKeys: account.PublicKeys,
	}

	response := &observe.GetAccountResponse{
		Account: accMsg,
	}

	return response, nil
}

// CallContract performs a contract call.
func (s *EmulatorServer) CallContract(ctx context.Context, req *observe.CallContractRequest) (*observe.CallContractResponse, error) {
	script := req.GetScript()
	value, _ := s.blockchain.CallScript(script)
	// TODO: add error handling besides just this
	if value == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid script")
	}

	// TODO: change this to whatever interface -> byte encoding decided on
	valueMsg := []byte(fmt.Sprintf("%v", value.(interface{})))

	response := &observe.CallContractResponse{
		Script: valueMsg,
	}

	return response, nil
}
