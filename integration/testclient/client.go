package testclient

import (
	"context"
	"math/rand"

	sdk "github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dapperlabs/flow-go-sdk/keys"

	"github.com/dapperlabs/flow-go/integration/dsl"
)

type TestClient struct {
	client *client.Client
	key    *sdk.AccountPrivateKey
}

func New(addr string, key *sdk.AccountPrivateKey) (*TestClient, error) {

	client, err := client.New(addr)
	if err != nil {
		return nil, err
	}

	tc := &TestClient{
		client: client,
		key:    key,
	}
	return tc, nil
}

func (c *TestClient) DeployContract(ctx context.Context, contract dsl.CadenceCode) error {

	code := dsl.Transaction{
		Import: dsl.Import{},
		Content: dsl.Prepare{
			Content: dsl.UpdateAccountCode{Code: contract.ToCadence()},
		},
	}

	return c.SendTransaction(ctx, code)
}

func (c *TestClient) SendTransaction(ctx context.Context, code dsl.CadenceCode) error {

	codeStr := code.ToCadence()

	tx := sdk.Transaction{
		Script:             []byte(codeStr),
		ReferenceBlockHash: nil,
		Nonce:              rand.Uint64(),
		ComputeLimit:       10,
		PayerAccount:       sdk.RootAddress,
		ScriptAccounts:     []sdk.Address{sdk.RootAddress},
	}

	sig, err := keys.SignTransaction(tx, *c.key)
	if err != nil {
		return err
	}

	tx.AddSignature(sdk.RootAddress, sig)

	return c.client.SendTransaction(ctx, tx)
}

func (c *TestClient) ExecuteScript(ctx context.Context, script dsl.Main) ([]byte, error) {

	code := script.ToCadence()

	res, err := c.client.ExecuteScript(ctx, []byte(code))
	if err != nil {
		return nil, err
	}

	return res, nil
}
