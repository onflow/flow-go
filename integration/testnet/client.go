package testnet

import (
	"context"
	"math/big"

	"github.com/dapperlabs/flow-go/crypto"
	"github.com/dapperlabs/flow-go/integration/client"
	"github.com/dapperlabs/flow-go/integration/dsl"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// Client is a GRPC client of the Observation API exposed by the Flow network.
// NOTE: we use integration/client rather than sdk/client as a stopgap until
// the SDK client is updated with the latest protobuf definitions.
type Client struct {
	client *client.Client
	key    *flow.AccountPrivateKey
}

func NewClient(addr string, key *flow.AccountPrivateKey) (*Client, error) {

	client, err := client.New(addr)
	if err != nil {
		return nil, err
	}

	tc := &Client{
		client: client,
		key:    key,
	}
	return tc, nil
}

func (c *Client) DeployContract(ctx context.Context, contract dsl.CadenceCode) error {

	code := dsl.Transaction{
		Import: dsl.Import{},
		Content: dsl.Prepare{
			Content: dsl.UpdateAccountCode{Code: contract.ToCadence()},
		},
	}

	return c.SendTransaction(ctx, code)
}

func (c *Client) SendTransaction(ctx context.Context, code dsl.Transaction) error {

	codeStr := code.ToCadence()

	rootAddress := flow.BytesToAddress(big.NewInt(1).Bytes())
	tx := flow.TransactionBody{
		Script:           []byte(codeStr),
		ReferenceBlockID: unittest.IdentifierFixture(),
		PayerAccount:     rootAddress,
	}

	sig, err := signTransaction(tx, c.key.PrivateKey)
	if err != nil {
		return err
	}

	accountSig := flow.AccountSignature{
		Account:   rootAddress,
		Signature: sig.Bytes(),
	}

	tx.Signatures = append(tx.Signatures, accountSig)

	return c.client.SendTransaction(ctx, tx)
}

func (c *Client) ExecuteScript(ctx context.Context, script dsl.Main) ([]byte, error) {

	code := script.ToCadence()

	res, err := c.client.ExecuteScript(ctx, []byte(code))
	if err != nil {
		return nil, err
	}

	return res, nil
}

// signTransaction signs a transaction with a private key.
func signTransaction(tx flow.TransactionBody, privateKey crypto.PrivateKey) (crypto.Signature, error) {
	hasher, err := crypto.NewHasherSHA3_256()
	if err != nil {
		return nil, err
	}

	transaction := flow.Transaction{
		TransactionBody: tx,
	}
	b := transaction.Singularity()
	return privateKey.Sign(b, hasher)
}
