package server

import (
	"context"
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
	"github.com/dapperlabs/bamboo-node/grpc/services/observe"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	"github.com/dapperlabs/bamboo-node/internal/emulator/core"
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

	err := s.blockchain.SubmitTransaction(tx)
	if err != nil {
		switch err.(type) {
		case *core.ErrTransactionReverted:
			s.logger.
				WithField("txHash", tx.Hash()).
				Infof("💸  Transaction #%d submitted to network", tx.Nonce)
			s.logger.WithError(err).Warnf("⚠️  Transaction #%d reverted", tx.Nonce)
		case *core.ErrDuplicateTransaction:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case *core.ErrInvalidTransactionSignature:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		s.logger.
			WithField("txHash", tx.Hash()).
			Infof("💸  Transaction #%d submitted to network", tx.Nonce)
	}

	hash := s.blockchain.CommitBlock()
	block, _ := s.blockchain.GetBlockByHash(hash)

	s.logger.WithFields(log.Fields{
		"blockNum":  block.Number,
		"blockHash": block.Hash(),
		"blockSize": len(block.TransactionHashes),
	}).Infof("️⛏  Block #%d mined", block.Number)

	response := &observe.SendTransactionResponse{
		Hash: tx.Hash().Bytes(),
	}

	return response, nil
}

// GetBlockByHash gets a block by hash.
func (s *EmulatorServer) GetBlockByHash(ctx context.Context, req *observe.GetBlockByHashRequest) (*observe.GetBlockByHashResponse, error) {
	hash := crypto.BytesToHash(req.GetHash())
	block, err := s.blockchain.GetBlockByHash(hash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	s.logger.WithFields(log.Fields{
		"blockNum":  block.Number,
		"blockHash": hash,
		"blockSize": len(block.TransactionHashes),
	}).Debugf("🎁  GetBlockByHash called")

	response := &observe.GetBlockByHashResponse{
		Block: block.ToMessage(),
	}

	return response, nil
}

// GetBlockByNumber gets a block by number.
func (s *EmulatorServer) GetBlockByNumber(ctx context.Context, req *observe.GetBlockByNumberRequest) (*observe.GetBlockByNumberResponse, error) {
	number := req.GetNumber()
	block, err := s.blockchain.GetBlockByNumber(number)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	s.logger.WithFields(log.Fields{
		"blockNum":  number,
		"blockHash": block.Hash(),
		"blockSize": len(block.TransactionHashes),
	}).Debugf("🎁  GetBlockByNumber called")

	response := &observe.GetBlockByNumberResponse{
		Block: block.ToMessage(),
	}

	return response, nil

}

// GetLatestBlock gets the latest sealed block.
func (s *EmulatorServer) GetLatestBlock(ctx context.Context, req *observe.GetLatestBlockRequest) (*observe.GetLatestBlockResponse, error) {
	block := s.blockchain.GetLatestBlock()

	s.logger.WithFields(log.Fields{
		"blockNum":  block.Number,
		"blockHash": block.Hash(),
		"blockSize": len(block.TransactionHashes),
	}).Debugf("🎁  GetLatestBlock called")

	response := &observe.GetLatestBlockResponse{
		Block: block.ToMessage(),
	}

	return response, nil
}

// GetTransaction gets a transaction by hash.
func (s *EmulatorServer) GetTransaction(ctx context.Context, req *observe.GetTransactionRequest) (*observe.GetTransactionResponse, error) {
	hash := crypto.BytesToHash(req.GetHash())
	tx, err := s.blockchain.GetTransaction(hash)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	s.logger.
		WithField("txHash", hash).
		Debugf("💵  GetTransaction called")

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
	account, err := s.blockchain.GetAccount(address)
	if err != nil {
		return nil, status.Error(codes.NotFound, err.Error())
	}

	s.logger.
		WithField("address", address).
		Debugf("👤  GetAccount called")

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
	s.logger.Debugf("📞  Contract script called")

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
