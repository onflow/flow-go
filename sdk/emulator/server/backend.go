package server

import (
	"context"
	"fmt"

	"github.com/logrusorgru/aurora"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"
	"github.com/dapperlabs/flow-go/protobuf/services/observation"
	"github.com/dapperlabs/flow-go/sdk/abi/encoding"
	"github.com/dapperlabs/flow-go/sdk/convert"
	"github.com/dapperlabs/flow-go/sdk/emulator"
)

// Backend wraps an emulated blockchain and implements the RPC handlers
// required by the Observation API.
type Backend struct {
	blockchain emulator.EmulatedBlockchainAPI
	logger     *logrus.Logger
}

// NewBackend returns a new backend.
func NewBackend(blockchain emulator.EmulatedBlockchainAPI, logger *logrus.Logger) *Backend {
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

	err = b.blockchain.AddTransaction(tx)
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
			Debug("️✉️   Transaction submitted")
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

	b.logger.WithFields(logrus.Fields{
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
	result, err := b.blockchain.ExecuteScript(script)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if result.Value == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid script")
	}

	printScriptResult(b.logger, result)

	valueBytes, err := encoding.Encode(result.Value)
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

	b.logger.WithFields(logrus.Fields{
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

// commitBlock executes the current pending transactions and commits the results in a new block.
func (b *Backend) commitBlock() {
	block, results, err := b.blockchain.ExecuteAndCommitBlock()
	if err != nil {
		b.logger.WithError(err).Error("Failed to commit block")
	} else {
		for _, result := range results {
			printTransactionResult(b.logger, result)
		}

		b.logger.WithFields(logrus.Fields{
			"blockNum":  block.Number,
			"blockHash": block.Hash().Hex(),
			"blockSize": len(block.TransactionHashes),
		}).Debugf("📦  Block #%d committed", block.Number)
	}
}

func printTransactionResult(logger *logrus.Logger, result emulator.TransactionResult) {
	if result.Succeeded() {
		logger.
			WithField("txHash", result.TransactionHash.Hex()).
			Info("⭐  Transaction executed")
	} else {
		logger.
			WithField("txHash", result.TransactionHash.Hex()).
			Warn("❗  Transaction reverted")
	}

	for _, log := range result.Logs {
		logger.Debugf(
			"%s %s",
			logPrefix("LOG", result.TransactionHash, aurora.BlueFg),
			log,
		)
	}

	for _, event := range result.Events {
		logger.Debugf(
			"%s %s",
			logPrefix("EVT", result.TransactionHash, aurora.GreenFg),
			event.String(),
		)
	}

	if result.Reverted() {
		logger.Warnf(
			"%s %s",
			logPrefix("ERR", result.TransactionHash, aurora.RedFg),
			result.Error.Error(),
		)
	}
}

func printScriptResult(logger *logrus.Logger, result emulator.ScriptResult) {
	if result.Succeeded() {
		logger.
			WithField("scriptHash", result.ScriptHash.Hex()).
			Info("⭐  Script executed")
	} else {
		logger.
			WithField("scriptHash", result.ScriptHash.Hex()).
			Warn("❗  Script reverted")
	}

	for _, log := range result.Logs {
		logger.Debugf(
			"%s %s",
			logPrefix("LOG", result.ScriptHash, aurora.BlueFg),
			log,
		)
	}

	for _, event := range result.Events {
		logger.Debugf(
			"%s %s",
			logPrefix("EVT", result.ScriptHash, aurora.GreenFg),
			event.String(),
		)
	}

	if result.Reverted() {
		logger.Warnf(
			"%s %s",
			logPrefix("ERR", result.ScriptHash, aurora.RedFg),
			result.Error.Error(),
		)
	}
}

func logPrefix(prefix string, hash crypto.Hash, color aurora.Color) string {
	prefix = aurora.Colorize(prefix, color|aurora.BoldFm).String()
	shortHash := fmt.Sprintf("[%s]", hash.Hex()[:6])
	shortHash = aurora.Colorize(shortHash, aurora.FaintFm).String()
	return fmt.Sprintf("%s %s", prefix, shortHash)
}
