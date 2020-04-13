package testnet

import (
	"context"
	"fmt"

	"github.com/dapperlabs/flow/protobuf/go/flow/access"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/integration/client"
	"github.com/dapperlabs/flow-go/integration/dsl"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// AccessClient is a GRPC client of the Access API exposed by the Flow network.
// NOTE: we use integration/client rather than sdk/client as a stopgap until
// the SDK client is updated with the latest protobuf definitions.
type Client struct {
	client *client.AccessClient
	key    *flow.AccountPrivateKey
}

// NewClientWithKey returns a new client to an Access API listening at the given
// address, using the given account key for signing transactions.
func NewClientWithKey(addr string, key *flow.AccountPrivateKey) (*Client, error) {

	client, err := client.NewAccessClient(addr)
	if err != nil {
		return nil, err
	}

	tc := &Client{
		client: client,
		key:    key,
	}
	return tc, nil
}

// NewClient returns a new client to an Access API listening at the given
// address, with a generated account key for signing transactions.
func NewClient(addr string) (*Client, error) {
	key, err := unittest.AccountKeyFixture()
	if err != nil {
		return nil, fmt.Errorf("could not generate key for client")
	}

	return NewClientWithKey(addr, key)
}

// DeployContract submits a transaction to deploy a contract with the given
// code to the root account.
func (c *Client) DeployContract(ctx context.Context, contract dsl.CadenceCode) error {

	code := dsl.Transaction{
		Import: dsl.Import{},
		Content: dsl.Prepare{
			Content: dsl.UpdateAccountCode{Code: contract.ToCadence()},
		},
	}

	tx := unittest.TransactionBodyFixture(unittest.WithTransactionDSL(code))

	return c.SendTransaction(ctx, tx)
}

// SignTransaction signs the transaction using the configured key and the root
// account, returning the signed transaction.
func (c *Client) SignTransaction(tx flow.TransactionBody) (flow.TransactionBody, error) {
	sig, err := signTransaction(tx, c.key.PrivateKey)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	accountSig := flow.AccountSignature{
		Account:   flow.RootAddress,
		Signature: sig,
	}
	tx.Signatures = append(tx.Signatures, accountSig)

	return tx, nil
}

// SendTransaction submits the transaction to the Access API. The caller must
// set up the transaction, including signing it.
func (c *Client) SendTransaction(ctx context.Context, tx flow.TransactionBody) error {
	return c.client.SendTransaction(ctx, tx)
}

// SignAndSendTransaction signs, then sends, a transaction. I bet you didn't
// see that one coming.
func (c *Client) SignAndSendTransaction(ctx context.Context, tx flow.TransactionBody) error {
	tx, err := c.SignTransaction(tx)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	return c.SendTransaction(ctx, tx)
}

func (c *Client) ExecuteScript(ctx context.Context, script dsl.Main) ([]byte, error) {

	code := script.ToCadence()

	res, err := c.client.ExecuteScript(ctx, []byte(code))
	if err != nil {
		return nil, fmt.Errorf("could not execute script: %w", err)
	}

	return res, nil
}

// signTransaction signs a transaction with a private key.
func signTransaction(tx flow.TransactionBody, privateKey crypto.PrivateKey) (crypto.Signature, error) {
	hasher := hash.NewSHA3_256()

	transaction := flow.Transaction{
		TransactionBody: tx,
	}
	b := transaction.Singularity()
	return privateKey.Sign(b, hasher)
}

func (c *Client) GetClient() *access.AccessAPIClient {
	return c.client.GetRPCClient()
}
