package server

import (
	"context"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/sdk/abi/encoding/values"
	"github.com/dapperlabs/flow-go/sdk/convert"
	"github.com/dapperlabs/flow-go/sdk/emulator"
)

// Backend wraps an emulated blockchain and implements the RPC handlers
// required by the Observation API.
type Backend struct {
	blockchain emulator.EmulatedBlockchainAPI
	logger     *log.Logger
}

// NewBackend returns a new backend.
func NewBackend(blockchain emulator.EmulatedBlockchainAPI, logger *log.Logger) *Backend {
	return &Backend{
		blockchain: blockchain,
		logger:     logger,
	}
}

// Ping the Observation API server for a response.
func (b *Backend) Ping(ctx context.Context, req *observation.PingRequest) (*observation.PingResponse, error) {
	response := &observation.PingResponse{
		Address: []byte("pong!"),
	}

	return response, nil
}

// SendTransaction submits a transaction to the network.
func (b *Backend) SendTransaction(ctx context.Context, req *observation.SendTransactionRequest) (*observation.SendTransactionResponse, error) {
	txMsg := req.GetTransaction()

	tx, err := convert.MessageToTransaction(txMsg)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	result, err := b.blockchain.SubmitTransaction(tx)
	if err != nil {
		switch err.(type) {
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
		b.logger.
			WithField("txHash", tx.Hash().Hex()).
			Infof("💸  Transaction #%d mined ", tx.Nonce)
		if result.Reverted() {
			b.logger.WithError(result.Error).Warnf("⚠️  Transaction #%d reverted", tx.Nonce)
		}
	}

	response := &observation.SendTransactionResponse{
		Hash: tx.Hash(),
	}

	return response, nil
}

// GetLatestBlock gets the latest sealed block.
func (b *Backend) GetLatestBlock(ctx context.Context, req *observation.GetLatestBlockRequest) (*observation.GetLatestBlockResponse, error) {
	block, err := b.blockchain.GetLatestBlock()
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	// create block header for block
	blockHeader := flow.Header{
		Parent: block.PreviousBlockHash,
		Number: block.Number,
	}

	b.logger.WithFields(log.Fields{
		"blockNum":  blockHeader.Number,
		"blockHash": blockHeader.Hash().Hex(),
	}).Debugf("🎁  GetLatestBlock called")

	response := &observation.GetLatestBlockResponse{
		Block: convert.BlockHeaderToMessage(blockHeader),
	}

	return response, nil
}

// GetTransaction gets a transaction by hash.
func (b *Backend) GetTransaction(ctx context.Context, req *observation.GetTransactionRequest) (*observation.GetTransactionResponse, error) {
	hash := crypto.BytesToHash(req.GetHash())

	tx, err := b.blockchain.GetTransaction(hash)
	if err != nil {
		switch err.(type) {
		case *emulator.ErrTransactionNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	b.logger.
		WithField("txHash", hash.Hex()).
		Debugf("💵  GetTransaction called")

	txMsg := convert.TransactionToMessage(*tx)

	eventMessages := make([]*entities.Event, len(tx.Events))
	for i, event := range tx.Events {
		eventMessages[i] = convert.EventToMessage(event)
	}

	return &observation.GetTransactionResponse{
		Transaction: txMsg,
		Events:      eventMessages,
	}, nil
}

// GetAccount returns the info associated with an address.
func (b *Backend) GetAccount(ctx context.Context, req *observation.GetAccountRequest) (*observation.GetAccountResponse, error) {
	address := flow.BytesToAddress(req.GetAddress())
	account, err := b.blockchain.GetAccount(address)
	if err != nil {
		switch err.(type) {
		case *emulator.ErrAccountNotFound:
			return nil, status.Error(codes.NotFound, err.Error())
		default:
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	b.logger.
		WithField("address", address).
		Debugf("👤  GetAccount called")

	accMsg, err := convert.AccountToMessage(*account)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &observation.GetAccountResponse{
		Account: accMsg,
	}, nil
}

// ExecuteScript performs a call.
func (b *Backend) ExecuteScript(ctx context.Context, req *observation.ExecuteScriptRequest) (*observation.ExecuteScriptResponse, error) {
	script := req.GetScript()
	value, events, err := b.blockchain.ExecuteScript(script)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if value == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid script")
	}

	b.logger.Debugf("📞  Contract script called")

	for _, event := range events {
		b.logger.Debugf("🔔  Event emitted: %s", event.String())
	}

	valueBytes, err := values.Encode(value)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	response := &observation.ExecuteScriptResponse{
		Value: valueBytes,
	}

	return response, nil
}

// GetEvents returns events matching a query.
func (b *Backend) GetEvents(ctx context.Context, req *observation.GetEventsRequest) (*observation.GetEventsResponse, error) {
	// Check for invalid queries
	if req.StartBlock > req.EndBlock {
		return nil, status.Error(codes.InvalidArgument, "invalid query: start block must be <= end block")
	}

	events, err := b.blockchain.GetEvents(req.GetType(), req.GetStartBlock(), req.GetEndBlock())
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	b.logger.WithFields(log.Fields{
		"eventType":  req.Type,
		"startBlock": req.StartBlock,
		"endBlock":   req.EndBlock,
		"results":    len(events),
	}).Debugf("🎁  GetEvents called")

	eventMessages := make([]*entities.Event, len(events))
	for i, event := range events {
		eventMessages[i] = convert.EventToMessage(event)
	}

	res := observation.GetEventsResponse{
		Events: eventMessages,
	}

	return &res, nil
}
