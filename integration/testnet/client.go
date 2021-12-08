package testnet

import (
	"context"
	"fmt"
	"github.com/onflow/flow-go-sdk/templates"
	"time"

	"google.golang.org/grpc"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	"github.com/onflow/flow-go-sdk/crypto"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol/inmem"
	"github.com/onflow/flow-go/utils/dsl"
	"github.com/onflow/flow-go/utils/unittest"
)

// AccessClient is a GRPC client of the Access API exposed by the Flow network.
// NOTE: we use integration/client rather than sdk/client as a stopgap until
// the SDK client is updated with the latest protobuf definitions.
type Client struct {
	client         *client.Client
	accountKey     *sdk.AccountKey
	accountKeyPriv sdkcrypto.PrivateKey
	signer         sdkcrypto.InMemorySigner
	seqNo          uint64
	Chain          flow.Chain
	account        *sdk.Account
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
		client:         flowClient,
		accountKey:     accountKey,
		accountKeyPriv: key,
		signer:         mySigner,
		Chain:          chain,
		seqNo:          accountKey.SequenceNumber,
		account:        acc,
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

// AccountKeyPriv returns the private used to create the in memory signer for the account
func (c *Client) AccountKeyPriv() sdkcrypto.PrivateKey {
	return c.accountKeyPriv
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
func (c *Client) DeployContract(ctx context.Context, refID sdk.Identifier, contract dsl.Contract) error {

	code := dsl.Transaction{
		Import: dsl.Import{},
		Content: dsl.Prepare{
			Content: dsl.UpdateAccountCode{Code: contract.ToCadence(), Name: contract.Name},
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

	err := tx.SignEnvelope(tx.Payer, tx.ProposalKey.KeyIndex, c.signer)
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

func (c *Client) ExecuteScriptBytes(ctx context.Context, script []byte, args []cadence.Value) (cadence.Value, error) {
	res, err := c.client.ExecuteScriptAtLatestBlock(ctx, script, args)
	if err != nil {
		return nil, fmt.Errorf("could not execute script: %w", err)
	}

	return res, nil
}

func (c *Client) SDKServiceAddress() sdk.Address {
	return sdk.Address(c.Chain.ServiceAddress())
}

// AccountKey returns the flow account key for the client
func (c *Client) AccountKey() *sdk.AccountKey {
	return c.accountKey
}

func (c *Client) Account() *sdk.Account {
	return c.account
}

func (c *Client) WaitForSealed(ctx context.Context, id sdk.Identifier) (*sdk.TransactionResult, error) {

	fmt.Printf("Waiting for transaction %s to be sealed...\n", id)
	errCount := 0
	var result *sdk.TransactionResult
	var err error
	for result == nil || (result.Status != sdk.TransactionStatusSealed) {
		childCtx, cancel := context.WithTimeout(ctx, time.Second*5)
		result, err = c.client.GetTransactionResult(childCtx, id)
		cancel()
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
		time.Sleep(time.Second)
	}

	fmt.Println()
	fmt.Printf("(Wait for Seal) Transaction %s sealed\n", id)

	return result, err
}

// GetLatestProtocolSnapshot ...
func (c *Client) GetLatestProtocolSnapshot(ctx context.Context) (*inmem.Snapshot, error) {
	b, err := c.client.GetLatestProtocolStateSnapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("could not get latest snapshot from access node: %v", err)
	}

	snapshot, err := convert.BytesToInmemSnapshot(b)
	if err != nil {
		return nil, fmt.Errorf("could not convert bytes to snapshot: %v", err)
	}

	return snapshot, nil
}

func (c *Client) GetLatestBlockID(ctx context.Context) (flow.Identifier, error) {
	header, err := c.client.GetLatestBlockHeader(ctx, true)
	if err != nil {
		return flow.ZeroID, fmt.Errorf("could not get latest block header: %w", err)
	}

	var id flow.Identifier
	copy(id[:], header.ID[:])
	return id, nil
}

func (c *Client) UserAddress(txResp *sdk.TransactionResult) (sdk.Address, bool) {
	var (
		address sdk.Address
		found   bool
	)

	// For account creation transactions that create multiple accounts, assume the last
	// created account is the user account. This is specifically the case for the
	// locked token shared account creation transactions.
	for _, event := range txResp.Events {
		if event.Type == sdk.EventAccountCreated {
			accountCreatedEvent := sdk.AccountCreatedEvent(event)
			address = accountCreatedEvent.Address()
			found = true
		}
	}

	return address, found
}

func (c *Client) TokenAmountByRole(role flow.Role) (string, float64, error) {
	if role == flow.RoleCollection {
		return "250000.0", 250000.0, nil
	}
	if role == flow.RoleConsensus {
		return "500000.0", 500000.0, nil
	}
	if role == flow.RoleExecution {
		return "1250000.0", 1250000.0, nil
	}
	if role == flow.RoleVerification {
		return "135000.0", 135000.0, nil
	}
	if role == flow.RoleAccess {
		return "0.0", 0.0, nil
	}

	return "", 0, fmt.Errorf("could not get token amount by role: %v", role)
}

func (c *Client) GetAccount(accountAddress sdk.Address) (*sdk.Account, error) {
	ctx := context.Background()
	account, err := c.client.GetAccount(ctx, accountAddress)
	if err != nil {
		return nil, fmt.Errorf("could not get account: %w", err)
	}

	return account, nil
}

func (c *Client) CreateAccount(
	ctx context.Context,
	accountKey *sdk.AccountKey,
	payerAccount *sdk.Account,
	payer sdk.Address,
	latestBlockID sdk.Identifier,
) (sdk.Address, error) {

	payerKey := payerAccount.Keys[0]
	tx := templates.CreateAccount([]*sdk.AccountKey{accountKey}, nil, payer)
	tx.SetGasLimit(1000).
		SetReferenceBlockID(latestBlockID).
		SetProposalKey(payer, 0, payerKey.SequenceNumber).
		SetPayer(payer)

	err := c.SignAndSendTransaction(ctx, tx)
	if err != nil {
		return sdk.Address{}, fmt.Errorf("failed to sign and send create account transaction %w", err)
	}

	result, err := c.WaitForSealed(ctx, tx.ID())
	if err != nil {
		return sdk.Address{}, fmt.Errorf("failed to wait for create account transaction to seal %w", err)
	}

	if address, ok := c.UserAddress(result); ok {
		return address, nil
	}

	return sdk.Address{}, fmt.Errorf("failed to get account address of the created flow account")
}
