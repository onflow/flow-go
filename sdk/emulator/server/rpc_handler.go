package server

import (
	"context"
	"encoding/json"
	"reflect"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/grpc/services/observe"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/pkg/types/proto"
	"github.com/dapperlabs/flow-go/sdk/emulator"
)

// Ping the Observation API server for a response.
func (s *EmulatorServer) Ping(ctx context.Context, req *observe.PingRequest) (*observe.PingResponse, error) {
	response := &observe.PingResponse{
		Address: []byte("pong!"),
	}

	return response, nil
}

// SendTransaction submits a transaction to the network.
func (s *EmulatorServer) SendTransaction(ctx context.Context, req *observe.SendTransactionRequest) (*observe.SendTransactionResponse, error) {
	txMsg := req.GetTransaction()

	tx, err := proto.MessageToTransaction(txMsg)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	err = s.blockchain.SubmitTransaction(&tx)
	if err != nil {
		switch err.(type) {
		case *emulator.ErrTransactionReverted:
			s.logger.
				WithField("txHash", tx.Hash().Hex()).
				Infof("💸  Transaction #%d mined", tx.Nonce)
			s.logger.WithError(err).Warnf("⚠️  Transaction #%d reverted", tx.Nonce)
		case *emulator.ErrDuplicateTransaction:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case *emulator.ErrInvalidSignaturePublicKey:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		case *emulator.ErrInvalidSignatureAccount:
			return nil, status.Error(codes.InvalidArgument, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		s.logger.
			WithField("txHash", tx.Hash().Hex()).
			Infof("💸  Transaction #%d mined ", tx.Nonce)
	}

	block := s.blockchain.CommitBlock()

	s.logger.WithFields(log.Fields{
		"blockNum":  block.Number,
		"blockHash": block.Hash().Hex(),
		"blockSize": len(block.TransactionHashes),
	}).Infof("️⛏  Block #%d mined", block.Number)

	response := &observe.SendTransactionResponse{
		Hash: tx.Hash(),
	}

	return response, nil
}

// GetLatestBlock gets the latest sealed block.
func (s *EmulatorServer) GetLatestBlock(ctx context.Context, req *observe.GetLatestBlockRequest) (*observe.GetLatestBlockResponse, error) {
	block := s.blockchain.GetLatestBlock()

	// create block header for block
	blockHeader := types.BlockHeader{
		Hash:              block.Hash(),
		PreviousBlockHash: block.PreviousBlockHash,
		Number:            block.Number,
		TransactionCount:  uint32(len(block.TransactionHashes)),
	}

	s.logger.WithFields(log.Fields{
		"blockNum":  blockHeader.Number,
		"blockHash": blockHeader.Hash.Hex(),
		"blockSize": blockHeader.TransactionCount,
	}).Debugf("🎁  GetLatestBlock called")

	response := &observe.GetLatestBlockResponse{
		Block: proto.BlockHeaderToMessage(blockHeader),
	}

	return response, nil
}

// GetTransaction gets a transaction by hash.
func (s *EmulatorServer) GetTransaction(ctx context.Context, req *observe.GetTransactionRequest) (*observe.GetTransactionResponse, error) {
	hash := crypto.BytesToHash(req.GetHash())

	tx, err := s.blockchain.GetTransaction(hash)
	if err != nil {
		switch err.(type) {
		case *emulator.ErrTransactionNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	s.logger.
		WithField("txHash", hash.Hex()).
		Debugf("💵  GetTransaction called")

	txMsg := proto.TransactionToMessage(*tx)

	return &observe.GetTransactionResponse{
		Transaction: txMsg,
	}, nil
}

// GetAccount returns the info associated with an address.
func (s *EmulatorServer) GetAccount(ctx context.Context, req *observe.GetAccountRequest) (*observe.GetAccountResponse, error) {
	address := types.BytesToAddress(req.GetAddress())
	account, err := s.blockchain.GetAccount(address)
	if err != nil {
		switch err.(type) {
		case *emulator.ErrAccountNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	s.logger.
		WithField("address", address).
		Debugf("👤  GetAccount called")

	accMsg := proto.AccountToMessage(*account)

	return &observe.GetAccountResponse{
		Account: accMsg,
	}, nil
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
	valueBytes, _ := json.Marshal(value)

	response := &observe.CallScriptResponse{
		// TODO: standardize types to be language-agnostic
		Type:  reflect.TypeOf(value).String(),
		Value: valueBytes,
	}

	return response, nil
}
