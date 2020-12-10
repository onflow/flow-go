package epochs

import (
	"context"
	"fmt"

	"google.golang.org/grpc"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk/client"

	jsonCadence "github.com/onflow/cadence/encoding/json"
	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
)

// QCContractClient is a client to the QC Contract
type QCContractClient struct {
	accessAddress   string         // address of the access node
	contractAddress string         // QuorumCertificate contract address
	client          *client.Client // flow-go-sdk client to access node
}

// NewQCContractClient returns a new client to the QC contract
func NewQCContractClient(accessAddress string, qcContractAddress string) (*QCContractClient, error) {

	// create a new instance of flow-go-sdk client
	flowClient, err := client.New(accessAddress, grpc.WithInsecure())
	if err != nil {
		return nil, fmt.Errorf("could not create flow client: %w", err)
	}

	return &QCContractClient{
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

	// attach submit vote transaction template and attempt to submit to reference blocks
	// would also need to get latest block to exectue on
	_ = sdk.NewTransaction().SetScript([]byte("")).SetReferenceBlockID(sdk.Identifier{})

	return nil
}

// Voted returns true if we have successfully submitted a vote to the
// cluster QC aggregator smart contract for the current epoch.
func (c *QCContractClient) Voted(ctx context.Context) (bool, error) {

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
