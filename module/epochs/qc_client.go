package epochs

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/onflow/cadence"
	jsonCadence "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-core-contracts/lib/go/templates"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/module"
)

// QCContractClient is a client to the QC Contract
type QCContractClient struct {
	accessAddress   string         // address of the access node
	contractAddress string         // QuorumCertificate contract address
	client          *client.Client // flow-go-sdk client to access node
	me              module.Local   // local node
}

// NewQCContractClient returns a new client to the QC contract
func NewQCContractClient(me module.Local, accessAddress string, qcContractAddress string) (*QCContractClient, error) {

	// create a new instance of flow-go-sdk client
	flowClient, err := client.New(accessAddress, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not create flow client: %w", err)
	}

	return &QCContractClient{
		me:              me,
		accessAddress:   accessAddress,
		contractAddress: qcContractAddress,
		client:          flowClient,
	}, nil
}

// SubmitVote submits the given vote to the cluster QC aggregator smart
// contract. This function returns only once the transaction has been
// processed by the network. An error is returned if the transaction has
// failed and should be re-submitted.
func (c *QCContractClient) SubmitVote(ctx context.Context, vote *model.Vote) error {

	// get latest sealed block to execute transaction
	latestBlock, err := c.client.GetLatestBlock(ctx, true)
	if err != nil {
		return fmt.Errorf("could not get latest block from node: %w", err)
	}

	// attach submit vote transaction template and build transaction
	// TODO: requires `ENV`?
	submitVoteScript := templates.GenerateSubmitVoteScript(templates.Environment{})
	tx := sdk.NewTransaction().
		SetScript(submitVoteScript).
		SetGasLimit(1000).
		SetReferenceBlockID(latestBlock.ID)

	// add signature data to the transaction and submit to node
	err = tx.AddArgument(cadence.NewString(string(vote.SigData)))
	if err != nil {
		return fmt.Errorf("could not add raw vote data to transaction: %w", err)
	}

	// TODO: what are these?
	keyIndex := 0
	account := sdk.Account{}

	// sign transaction
	err = ``tx.SignPayload(account.Address, keyIndex, c.me)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	// submit signed transaction to node
	txHash, err := c.submitTx(tx)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	return nil
}

// Voted returns true if we have successfully submitted a vote to the
// cluster QC aggregator smart contract for the current epoch.
func (c *QCContractClient) Voted(ctx context.Context) (bool, error) {

	// TODO: transaction for has voted is missing?

	// args for voted
	args := make([][]byte, 0)

	// convert to cadence values
	arguments := []cadence.Value{}
	for _, arg := range args {
		val, err := jsonCadence.Decode(arg)
		if err != nil {
			return false, fmt.Errorf("could not deocde arguments: %w", err)
		}
		arguments = append(arguments, val)
	}

	// execute script to read if voted
	_, err := c.client.ExecuteScriptAtLatestBlock(ctx, []byte{}, arguments)
	if err != nil {
		return false, fmt.Errorf("could not execute voted script: %w", err)
	}

	return false, nil
}

// submitTx submits a transaction to flow
func (c *QCContractClient) submitTx(tx *sdk.Transaction) (sdk.Identifier, error) {

	// check if the transaction has a signature
	if len(tx.EnvelopeSignatures) == 0 {
		return sdk.EmptyID, fmt.Errorf("can not submit an unsigned transaction")
	}

	// submit trnsaction to client
	err := c.client.SendTransaction(context.Background(), *tx)
	if err != nil {
		return sdk.EmptyID, fmt.Errorf("failed to send transaction: %w", err)
	}

	return tx.ID(), nil
}
