package network

import (
	"context"
	"time"

	sdk "github.com/dapperlabs/flow-go-sdk"
	"github.com/dapperlabs/flow-go-sdk/client"
	"github.com/dapperlabs/flow-go-sdk/keys"
)

func NewFlowTestClient(context context.Context, client *client.Client, key *sdk.AccountPrivateKey) *FlowTestClient {
	return &FlowTestClient{
		context: context,
		client:  client,
		key:     key,
	}
}

type FlowTestClient struct {
	context context.Context
	client  *client.Client
	key     *sdk.AccountPrivateKey
}

func (f *FlowTestClient) DeployContract(contract CadenceCode) error {

	code := Transaction{
		Import{},
		Prepare{
			UpdateAccountCode{contract.ToCadence()},
		},
	}

	return f.SendTransaction(code)
}

func (f *FlowTestClient) SendTransaction(code CadenceCode) error {

	codeStr := code.ToCadence()

	tx := sdk.Transaction{
		Script:             []byte(codeStr),
		ReferenceBlockHash: nil,
		Nonce:              uint64(2137 + time.Now().UnixNano()),
		ComputeLimit:       10,
		PayerAccount:       sdk.RootAddress,
		ScriptAccounts:     []sdk.Address{sdk.RootAddress},
	}

	sig, err := keys.SignTransaction(tx, *f.key)
	if err != nil {
		return err
	}

	tx.AddSignature(sdk.RootAddress, sig)

	return f.client.SendTransaction(f.context, tx)
}

func (f *FlowTestClient) ExecuteScript(script Main) ([]byte, error) {

	code := script.ToCadence()

	executeScript, err := f.client.ExecuteScript(f.context, []byte(code))
	if err != nil {
		return nil, err
	}

	return executeScript, nil
}
