package server

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/model/types"
	"github.com/dapperlabs/flow-go/proto/services/observation"
	"github.com/dapperlabs/flow-go/sdk/convert"
	"github.com/dapperlabs/flow-go/sdk/emulator"
	"github.com/dapperlabs/flow-go/sdk/emulator/events"
)

// Backend wraps an emulated blockchain and implements the RPC handlers
// required by the Observation GRPC API.
type Backend struct {
	blockchain *emulator.EmulatedBlockchain
	eventStore events.Store
	logger     *log.Logger
}

// NewBackend returns a new backend.
func NewBackend(blockchain *emulator.EmulatedBlockchain, eventStore events.Store, logger *log.Logger) *Backend {
	return &Backend{
		blockchain: blockchain,
		eventStore: eventStore,
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

	err = b.blockchain.SubmitTransaction(&tx)
	if err != nil {
		switch err.(type) {
		case *emulator.ErrTransactionReverted:
			b.logger.
				WithField("txHash", tx.Hash().Hex()).
				Infof("💸  Transaction #%d mined", tx.Nonce)
			b.logger.WithError(err).Warnf("⚠️  Transaction #%d reverted", tx.Nonce)
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
	}

	block := b.blockchain.CommitBlock()

	b.logger.WithFields(log.Fields{
		"blockNum":  block.Number,
		"blockHash": block.Hash().Hex(),
		"blockSize": len(block.TransactionHashes),
	}).Infof("⛏  Block #%d mined", block.Number)

	response := &observation.SendTransactionResponse{
		Hash: tx.Hash(),
	}

	return response, nil
}

// GetLatestBlock gets the latest sealed block.
func (b *Backend) GetLatestBlock(ctx context.Context, req *observation.GetLatestBlockRequest) (*observation.GetLatestBlockResponse, error) {
	block := b.blockchain.GetLatestBlock()

	// create block header for block
	blockHeader := types.BlockHeader{
		Hash:              block.Hash(),
		PreviousBlockHash: block.PreviousBlockHash,
		Number:            block.Number,
		TransactionCount:  uint32(len(block.TransactionHashes)),
	}

	b.logger.WithFields(log.Fields{
		"blockNum":  blockHeader.Number,
		"blockHash": blockHeader.Hash.Hex(),
		"blockSize": blockHeader.TransactionCount,
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

	return &observation.GetTransactionResponse{
		Transaction: txMsg,
	}, nil
}

// GetAccount returns the info associated with an address.
func (b *Backend) GetAccount(ctx context.Context, req *observation.GetAccountRequest) (*observation.GetAccountResponse, error) {
	address := types.BytesToAddress(req.GetAddress())
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

// CallScript performs a call.
func (b *Backend) CallScript(ctx context.Context, req *observation.CallScriptRequest) (*observation.CallScriptResponse, error) {
	script := req.GetScript()
	value, err := b.blockchain.CallScript(script)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if value == nil {
		return nil, status.Error(codes.InvalidArgument, "invalid script")
	}

	b.logger.Debugf("📞  Contract script called")

	// TODO: change this to whatever interface -> byte encoding decided on
	valueBytes, _ := json.Marshal(value)

	response := &observation.CallScriptResponse{
		// TODO: standardize types to be language-agnostic
		Type:  reflect.TypeOf(value).String(),
		Value: valueBytes,
	}

	return response, nil
}

// GetEvents returns events matching a query.
func (b *Backend) GetEvents(ctx context.Context, req *observation.GetEventsRequest) (*observation.GetEventsResponse, error) {
	query := convert.MessageToEventQuery(req)

	// Check for invalid queries
	if query.StartBlock > query.EndBlock {
		return nil, status.Error(codes.InvalidArgument, "invalid query: start block must be <= end block")
	}

	events, err := b.eventStore.Query(ctx, query)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	b.logger.WithFields(log.Fields{
		"eventID":    query.ID,
		"startBlock": query.StartBlock,
		"endBlock":   query.EndBlock,
		"results":    len(events),
	}).Debugf("🎁  GetEvents called")

	var buf bytes.Buffer
	err = json.NewEncoder(&buf).Encode(events)
	if err != nil {
		return nil, err
	}
	res := observation.GetEventsResponse{
		EventsJson: buf.Bytes(),
	}

	return &res, nil
}
