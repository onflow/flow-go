package client

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes"

	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-node/pkg/grpc/services/observe"
	"github.com/dapperlabs/bamboo-node/internal/emulator/data"
	"github.com/dapperlabs/bamboo-node/pkg/crypto"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

type Client struct {
	conn       *grpc.ClientConn
	grpcClient observe.ObserveServiceClient
}

func New(host string, port int) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	grpcClient := observe.NewObserveServiceClient(conn)

	return &Client{
		conn:       conn,
		grpcClient: grpcClient,
	}, nil
}

func (c *Client) Close() {
	c.conn.Close()
}

// SendTransaction submits a transaction to an access node.
func (c *Client) SendTransaction(ctx context.Context, tx *types.SignedTransaction) error {
	txMsg := &observe.SendTransactionRequest_Transaction{
		ToAddress:    tx.ToAddress.Bytes(),
		Script:       tx.Script,
		Nonce:        tx.Nonce,
		ComputeLimit: tx.ComputeLimit,
		PayerSignature: &observe.Signature{
			AccountAddress: tx.PayerSignature.Account.Bytes(),
			Signature:      tx.PayerSignature.Sig,
		},
	}

	_, err := c.grpcClient.SendTransaction(
		ctx,
		&observe.SendTransactionRequest{Transaction: txMsg},
	)
	return err
}

// GetBlockByHash fetches a block by hash.
func (c *Client) GetBlockByHash(ctx context.Context, h crypto.Hash) (*data.Block, error) {
	res, err := c.grpcClient.GetBlockByHash(
		ctx,
		&observe.GetBlockByHashRequest{Hash: h.Bytes()},
	)
	if err != nil {
		return nil, err
	}

	block := res.GetBlock()

	timestamp, err := ptypes.Timestamp(block.GetTimestamp())
	if err != nil {
		return nil, err
	}

	return &data.Block{
		Number:            block.GetNumber(),
		PrevBlockHash:     crypto.BytesToHash(block.GetPrevBlockHash()),
		Timestamp:         timestamp,
		Status:            data.BlockStatus(block.GetStatus()),
		TransactionHashes: crypto.BytesToHashes(block.GetTransactionHashes()),
	}, nil
}

// GetBlockByNumber fetches a block by number.
func (c *Client) GetBlockByNumber(ctx context.Context, n uint64) (*data.Block, error) {
	res, err := c.grpcClient.GetBlockByNumber(
		ctx,
		&observe.GetBlockByNumberRequest{Number: n},
	)
	if err != nil {
		return nil, err
	}

	block := res.GetBlock()

	timestamp, err := ptypes.Timestamp(block.GetTimestamp())
	if err != nil {
		return nil, err
	}

	return &data.Block{
		Number:            block.GetNumber(),
		PrevBlockHash:     crypto.BytesToHash(block.GetPrevBlockHash()),
		Timestamp:         timestamp,
		Status:            data.BlockStatus(block.GetStatus()),
		TransactionHashes: crypto.BytesToHashes(block.GetTransactionHashes()),
	}, nil
}

// GetTransaction fetches a transaction by hash.
func (c *Client) GetTransaction(ctx context.Context, h crypto.Hash) (*types.SignedTransaction, error) {
	res, err := c.grpcClient.GetTransaction(
		ctx,
		&observe.GetTransactionRequest{Hash: h.Bytes()},
	)
	if err != nil {
		return nil, err
	}

	tx := res.GetTransaction()
	payerSig := tx.GetPayerSignature()

	return &types.SignedTransaction{
		ToAddress:    crypto.BytesToAddress(tx.GetToAddress()),
		Script:       tx.GetScript(),
		Nonce:        tx.GetNonce(),
		ComputeLimit: tx.GetComputeLimit(),
		ComputeUsed:  tx.GetComputeUsed(),
		PayerSignature: &crypto.Signature{
			Account: crypto.BytesToAddress(payerSig.GetAccountAddress()),
			Sig:     payerSig.GetSignature(),
		},
		Status: data.TxStatus(tx.GetStatus()),
	}, nil
}

// GetAccount fetches an account by address.
func (c *Client) GetAccount(ctx context.Context, a crypto.Address) (*crypto.Account, error) {
	res, err := c.grpcClient.GetAccount(
		ctx,
		&observe.GetAccountRequest{Address: a.Bytes()},
	)
	if err != nil {
		return nil, err
	}

	account := res.GetAccount()

	return &crypto.Account{
		Address:    crypto.BytesToAddress(account.GetAddress()),
		Balance:    account.GetBalance(),
		Code:       account.GetCode(),
		PublicKeys: account.GetPublicKeys(),
	}, nil
}
