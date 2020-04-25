package testnet

import (
	"context"
	"fmt"

	"github.com/onflow/cadence"
	flowgosdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"

	"github.com/dapperlabs/flow-go/crypto/hash"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/dsl"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

// AccessClient is a GRPC client of the Access API exposed by the Flow network.
// NOTE: we use integration/client rather than sdk/client as a stopgap until
// the SDK client is updated with the latest protobuf definitions.
type Client struct {
	client *client.Client
	key    *flow.AccountPrivateKey
}

// NewClientWithKey returns a new client to an Access API listening at the given
// address, using the given account key for signing transactions.
func NewClientWithKey(addr string, key *flow.AccountPrivateKey) (*Client, error) {

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

// SignTransaction signs the transaction using the proposer's key
func (c *Client) SignTransaction(tx flow.TransactionBody) (flow.TransactionBody, error) {

	hasher := hash.NewSHA3_256()
	err := tx.SignPayload(tx.Payer, tx.ProposalKey.KeyID, c.key.PrivateKey, hasher)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	err = tx.SignEnvelope(tx.Payer, tx.ProposalKey.KeyID, c.key.PrivateKey, hasher)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	return tx, err
}

// SendTransaction submits the transaction to the Access API. The caller must
// set up the transaction, including signing it.
func (c *Client) SendTransaction(ctx context.Context, tx flow.TransactionBody) error {
	return c.client.SendTransaction(ctx, flowGoSDKTransaction(tx))
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

func (c *Client) ExecuteScript(ctx context.Context, script dsl.Main) (cadence.Value, error) {

	code := script.ToCadence()

	res, err := c.client.ExecuteScriptAtLatestBlock(ctx, []byte(code))
	if err != nil {
		return nil, fmt.Errorf("could not execute script: %w", err)
	}

	return res, nil
}

func flowGoSDKTransaction(tx flow.TransactionBody) flowgosdk.Transaction {
	flowgosdkProposalKey := flowgosdk.ProposalKey{
		Address:        flowgosdk.Address(tx.ProposalKey.Address),
		KeyID:          tx.ProposalKey.KeyID,
		SequenceNumber: tx.ProposalKey.SequenceNumber,
	}

	flowGoSDKTx := flowgosdk.Transaction{
		Script:             tx.Script,
		ReferenceBlockID:   flowgosdk.Identifier(tx.ReferenceBlockID),
		GasLimit:           tx.GasLimit,
		ProposalKey:        flowgosdkProposalKey,
		Payer:              flowgosdk.Address(tx.Payer),
		Authorizers:        flowGoSDKAddresses(tx.Authorizers),
		PayloadSignatures:  flowGoSDKSignatures(tx.PayloadSignatures),
		EnvelopeSignatures: flowGoSDKSignatures(tx.EnvelopeSignatures),
	}

	return flowGoSDKTx
}

func flowGoSDKAddress(address flow.Address) flowgosdk.Address {
	return flowgosdk.Address(address)
}

func flowGoSDKAddresses(addresses []flow.Address) []flowgosdk.Address {
	fgsAddrs := make([]flowgosdk.Address, len(addresses))
	for i, a := range addresses {
		fgsAddrs[i] = flowGoSDKAddress(a)
	}
	return fgsAddrs
}

func flowGoSDKSignature(sig flow.TransactionSignature) flowgosdk.TransactionSignature {
	return flowgosdk.TransactionSignature{
		Address:     flowGoSDKAddress(sig.Address),
		SignerIndex: sig.SignerIndex,
		KeyID:       sig.KeyID,
		Signature:   sig.Signature,
	}
}

func flowGoSDKSignatures(sigs []flow.TransactionSignature) []flowgosdk.TransactionSignature {
	fgsSigs := make([]flowgosdk.TransactionSignature, len(sigs))
	for i, s := range sigs {
		fgsSigs[i] = flowGoSDKSignature(s)
	}
	return fgsSigs
}
