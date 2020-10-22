package testnet

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/dsl"
	"github.com/onflow/flow-go/utils/unittest"
)

// AccessClient is a GRPC client of the Access API exposed by the Flow network.
// NOTE: we use integration/client rather than sdk/client as a stopgap until
// the SDK client is updated with the latest protobuf definitions.
type Client struct {
	client *client.Client
	key    *sdk.AccountKey
	signer sdkcrypto.InMemorySigner
	seqNo  uint64
	Chain  flow.Chain
}

// NewClientWithKey returns a new client to an Access API listening at the given
// address, using the given account key for signing transactions.
func NewClientWithKey(accessAddr string, accountAddr sdk.Address, key sdkcrypto.PrivateKey, chain flow.Chain) (*Client, error) {

	flowClient, err := client.New(accessAddr, grpc.WithInsecure())
	if err != nil {
		return nil, err
	}

	acc, err := flowClient.GetAccount(context.Background(), accountAddr)
	if err != nil {
		return nil, err
	}
	accountKey := acc.Keys[0]

	mySigner := crypto.NewInMemorySigner(key, accountKey.HashAlgo)

	tc := &Client{
		client: flowClient,
		key:    accountKey,
		signer: mySigner,
		Chain:  chain,
		seqNo:  accountKey.SequenceNumber,
	}
	return tc, nil
}

// NewClient returns a new client to an Access API listening at the given
// address, with a test service account key for signing transactions.
func NewClient(addr string, chain flow.Chain) (*Client, error) {
	key := unittest.ServiceAccountPrivateKey
	privateKey, err := sdkcrypto.DecodePrivateKey(sdkcrypto.SignatureAlgorithm(key.SignAlgo), key.PrivateKey.Encode())
	if err != nil {
		return nil, err
	}
	// Uncomment for debugging keys

	//json, err := key.MarshalJSON()
	//if err != nil {
	//	return nil, fmt.Errorf("cannot marshal key json: %w", err)
	//}
	//public := key.PublicKey(1000)
	//publicJson, err := public.MarshalJSON()
	//if err != nil {
	//	return nil, fmt.Errorf("cannot marshal key json: %w", err)
	//}

	//fmt.Printf("New client with private key: \n%s\n", json)
	//fmt.Printf("and public key: \n%s\n", publicJson)

	return NewClientWithKey(addr, sdk.Address(chain.ServiceAddress()), privateKey, chain)
}

func (c *Client) GetSeqNumber() uint64 {
	n := c.seqNo
	c.seqNo++
	return n
}

func (c *Client) Events(ctx context.Context, typ string) ([]client.BlockEvents, error) {
	return c.client.GetEventsForHeightRange(ctx, client.EventRangeQuery{
		Type:        typ,
		StartHeight: 0,
		EndHeight:   1000,
	})
}

// DeployContract submits a transaction to deploy a contract with the given
// code to the root account.
func (c *Client) DeployContract(ctx context.Context, refID sdk.Identifier, contract dsl.CadenceCode) error {

	code := dsl.Transaction{
		Import: dsl.Import{},
		Content: dsl.Prepare{
			Content: dsl.UpdateAccountCode{Code: contract.ToCadence()},
		},
	}

	tx := sdk.NewTransaction().
		SetScript([]byte(code.ToCadence())).
		SetReferenceBlockID(refID).
		SetProposalKey(c.SDKServiceAddress(), 0, c.GetSeqNumber()).
		SetPayer(c.SDKServiceAddress()).
		AddAuthorizer(c.SDKServiceAddress())

	return c.SignAndSendTransaction(ctx, tx)
}

// SignTransaction signs the transaction using the proposer's key
func (c *Client) SignTransaction(tx *sdk.Transaction) (*sdk.Transaction, error) {

	err := tx.SignEnvelope(tx.Payer, tx.ProposalKey.KeyID, c.signer)
	if err != nil {
		return nil, err
	}

	return tx, err
}

// SendTransaction submits the transaction to the Access API. The caller must
// set up the transaction, including signing it.
func (c *Client) SendTransaction(ctx context.Context, tx *sdk.Transaction) error {
	return c.client.SendTransaction(ctx, *tx)
}

// SignAndSendTransaction signs, then sends, a transaction. I bet you didn't
// see that one coming.
func (c *Client) SignAndSendTransaction(ctx context.Context, tx *sdk.Transaction) error {
	tx, err := c.SignTransaction(tx)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	return c.SendTransaction(ctx, tx)
}

func (c *Client) ExecuteScript(ctx context.Context, script dsl.Main) (cadence.Value, error) {

	code := script.ToCadence()

	res, err := c.client.ExecuteScriptAtLatestBlock(ctx, []byte(code), nil)
	if err != nil {
		return nil, fmt.Errorf("could not execute script: %w", err)
	}

	return res, nil
}

func (c *Client) SDKServiceAddress() sdk.Address {
	return sdk.Address(c.Chain.ServiceAddress())
}

func (c *Client) WaitForSealed(ctx context.Context, id sdk.Identifier) (*sdk.TransactionResult, error) {
	result, err := c.client.GetTransactionResult(ctx, id)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Waiting for transaction %s to be sealed...\n", id)
	errCount := 0
	for result == nil || (result.Status != sdk.TransactionStatusSealed) {
		time.Sleep(time.Second)
		result, err = c.client.GetTransactionResult(ctx, id)
		if err != nil {
			fmt.Print("x")
			errCount++
			if errCount >= 10 {
				return &sdk.TransactionResult{
					Error: err,
				}, err
			}
		} else {
			fmt.Print(".")
		}
	}

	fmt.Println()
	fmt.Printf("Transaction %s sealed\n", id)

	return result, err
}
