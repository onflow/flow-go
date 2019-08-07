package server

import (
	"bytes"
	"context"
	"encoding/gob"
	"reflect"
	"time"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	crypto "github.com/dapperlabs/bamboo-node/pkg/crypto/oldcrypto"
	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"
	"github.com/dapperlabs/bamboo-node/pkg/grpc/shared"
	"github.com/dapperlabs/bamboo-node/pkg/types"

	"github.com/dapperlabs/bamboo-node/internal/emulator/core"
)

// Ping the Observation API server for a response.
func (s *EmulatorServer) Ping(ctx context.Context, req *observe.PingRequest) (*observe.PingResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")
}

// SendTransaction submits a transaction to the network.
func (s *EmulatorServer) SendTransaction(ctx context.Context, req *observe.SendTransactionRequest) (*observe.SendTransactionResponse, error) {
	txMsg := req.GetTransaction()
	payerSig := txMsg.GetPayerSignature()

	// TODO: take timestamp from SignedTransaction message
	tx := &types.SignedTransaction{
		Script:       txMsg.GetScript(),
		Nonce:        txMsg.GetNonce(),
		ComputeLimit: txMsg.GetComputeLimit(),
		Timestamp:    time.Now(),
		PayerSignature: types.AccountSignature{
			Account: types.BytesToAddress(payerSig.GetAccount()),
			// TODO: update this (default signature for now)
			PublicKey: []byte{},
			Signature: []byte{},
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
		case *core.ErrInvalidSignaturePublicKey:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case *core.ErrInvalidSignatureAccount:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		s.logger.
			WithField("txHash", tx.Hash()).
			Infof("💸  Transaction #%d submitted to network", tx.Nonce)
	}

	block := s.blockchain.CommitBlock()

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
		switch err.(type) {
		case *core.ErrBlockNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
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
		switch err.(type) {
		case *core.ErrBlockNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
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
		switch err.(type) {
		case *core.ErrTransactionNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	s.logger.
		WithField("txHash", hash).
		Debugf("💵  GetTransaction called")

	// TODO: add timestamp for SignTransaction response
	txMsg := &shared.SignedTransaction{
		Script:       tx.Script,
		Nonce:        tx.Nonce,
		ComputeLimit: tx.ComputeLimit,
		ComputeUsed:  tx.ComputeUsed,
		PayerSignature: &shared.AccountSignature{
			Account: tx.PayerSignature.Account.Bytes(),
			// TODO: update this (default signature bytes for now)
			Signature: tx.PayerSignature.Signature,
		},
		Status: shared.TransactionStatus(tx.Status),
	}

	response := &observe.GetTransactionResponse{
		Transaction: txMsg,
	}

	return response, nil
}

// GetAccount returns the info associated with an address.
func (s *EmulatorServer) GetAccount(ctx context.Context, req *observe.GetAccountRequest) (*observe.GetAccountResponse, error) {
	address := types.BytesToAddress(req.GetAddress())
	account, err := s.blockchain.GetAccount(address)
	if err != nil {
		switch err.(type) {
		case *core.ErrAccountNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
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

// CallScript performs a call.
func (s *EmulatorServer) CallScript(ctx context.Context, req *observe.CallScriptRequest) (*observe.CallScriptResponse, error) {
	script := req.GetScript()
	value, err := s.blockchain.CallScript(script)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if value == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid script")
	}

	s.logger.Debugf("📞  Contract script called")

	// TODO: change this to whatever interface -> byte encoding decided on
	var valueBuf bytes.Buffer
	enc := gob.NewEncoder(&valueBuf)
	enc.Encode(value)

	response := &observe.CallScriptResponse{
		// TODO: standardize types to be language-agnostic
		Type:  reflect.TypeOf(value).String(),
		Value: valueBuf.Bytes(),
	}

	return response, nil
}
