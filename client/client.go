package client

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes"

	"google.golang.org/grpc"

	"github.com/dapperlabs/bamboo-emulator/crypto"
	"github.com/dapperlabs/bamboo-emulator/data"
	"github.com/dapperlabs/bamboo-emulator/gen/grpc/services/accessv1"
	"github.com/dapperlabs/bamboo-emulator/types"
)

type Client struct {
	conn       *grpc.ClientConn
	grpcClient accessv1.BambooAccessAPIClient
}

func New(host string, port int) (*Client, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	grpcClient := accessv1.NewBambooAccessAPIClient(conn)

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
	txMsg := &accessv1.SendTransactionRequest_Transaction{
		ToAddress:      tx.ToAddress.Bytes(),
		Script:         tx.Script,
		Nonce:          tx.Nonce,
		ComputeLimit:   tx.ComputeLimit,
		PayerSignature: tx.PayerSignature,
	}

	_, err := c.grpcClient.SendTransaction(
		ctx,
		&accessv1.SendTransactionRequest{Transaction: txMsg},
	)
	return err
}

// GetBlockByHash fetches a block by hash.
func (c *Client) GetBlockByHash(ctx context.Context, h crypto.Hash) (*data.Block, error) {
	res, err := c.grpcClient.GetBlockByHash(
		ctx,
		&accessv1.GetBlockByHashRequest{Hash: h.Bytes()},
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
		&accessv1.GetBlockByNumberRequest{Number: n},
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
		&accessv1.GetTransactionRequest{Hash: h.Bytes()},
	)
	if err != nil {
		return nil, err
	}

	tx := res.GetTransaction()

	return &types.SignedTransaction{
		ToAddress:      crypto.BytesToAddress(tx.GetToAddress()),
		Script:         tx.GetScript(),
		Nonce:          tx.GetNonce(),
		ComputeLimit:   tx.GetComputeLimit(),
		ComputeUsed:    tx.GetComputeUsed(),
		PayerSignature: tx.GetPayerSignature(),
		Status:         data.TxStatus(tx.GetStatus()),
	}, nil
}

// GetAccount fetches an account by address.
func (c *Client) GetAccount(ctx context.Context, a crypto.Address) (*crypto.Account, error) {
	res, err := c.grpcClient.GetAccount(
		ctx,
		&accessv1.GetAccountRequest{Address: a.Bytes()},
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
