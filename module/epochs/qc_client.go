package epochs

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/rs/zerolog"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	hotstuffver "github.com/onflow/flow-go/consensus/hotstuff/verification"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

const (

	// TransactionSubmissionTimeout is the time after which we return an error.
	TransactionSubmissionTimeout = 5 * time.Minute

	// TransactionStatusRetryTimeout is the time after which the status of a
	// transaction is checked again
	TransactionStatusRetryTimeout = 1 * time.Second
)

// QCContractClient is a client to the Quorum Certificate contract. Allows the client to
// functionality to submit a vote and check if collection node has voted already.
type QCContractClient struct {
	BaseClient

	nodeID flow.Identifier // flow identifier of the collection node
	env    templates.Environment
}

// NewQCContractClient returns a new client to the Quorum Certificate contract
func NewQCContractClient(
	log zerolog.Logger,
	flowClient module.SDKClientWrapper,
	nodeID flow.Identifier,
	accountAddress string,
	accountKeyIndex uint,
	qcContractAddress string,
	signer sdkcrypto.Signer,
) *QCContractClient {

	log = log.With().Str("component", "qc_contract_client").Logger()
	base := NewBaseClient(log, flowClient, accountAddress, accountKeyIndex, signer, qcContractAddress)

	// set QCContractAddress to the contract address given
	env := templates.Environment{QuorumCertificateAddress: qcContractAddress}

	return &QCContractClient{
		BaseClient: *base,
		nodeID:     nodeID,
		env:        env,
	}
}

// SubmitVote submits the given vote to the cluster QC aggregator smart
// contract. This function returns only once the transaction has been
// processed by the network. An error is returned if the transaction has
// failed and should be re-submitted.
func (c *QCContractClient) SubmitVote(ctx context.Context, vote *model.Vote) error {

	// time method was invoked
	started := time.Now()

	// add a timeout to the context
	ctx, cancel := context.WithTimeout(ctx, TransactionSubmissionTimeout)
	defer cancel()

	// get account for given address and also validates AccountKeyIndex is valid
	account, err := c.GetAccount(ctx)
	if err != nil {
		return fmt.Errorf("could not get account: %w", err)
	}

	// get latest finalized block to execute transaction
	latestBlock, err := c.FlowClient.GetLatestBlock(ctx, false)
	if err != nil {
		return fmt.Errorf("could not get latest block from node: %w", err)
	}

	// attach submit vote transaction template and build transaction
	seqNumber := account.Keys[int(c.AccountKeyIndex)].SequenceNumber
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSubmitVoteScript(c.env)).
		SetGasLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(account.Address, int(c.AccountKeyIndex), seqNumber).
		SetPayer(account.Address).
		AddAuthorizer(account.Address)

	// add signature to the transaction
	sigDataHex, err := cadence.NewString(hex.EncodeToString(vote.SigData))
	if err != nil {
		return fmt.Errorf("could not convert vote sig data: %w", err)
	}
	err = tx.AddArgument(sigDataHex)
	if err != nil {
		return fmt.Errorf("could not add raw vote data to transaction: %w", err)
	}

	// add message to the transaction
	voteMessage := hotstuffver.MakeVoteMessage(vote.View, vote.BlockID)
	voteMessageHex, err := cadence.NewString(hex.EncodeToString(voteMessage))
	if err != nil {
		return fmt.Errorf("could not convert vote message: %w", err)
	}
	err = tx.AddArgument(voteMessageHex)
	if err != nil {
		return fmt.Errorf("could not add raw vote data to transaction: %w", err)
	}

	// sign envelope using account signer
	err = tx.SignEnvelope(account.Address, int(c.AccountKeyIndex), c.Signer)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	// submit signed transaction to node
	c.Log.Info().Str("tx_id", tx.ID().Hex()).Msg("sending SubmitResult transaction")
	txID, err := c.SendTransaction(ctx, tx)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	err = c.WaitForSealed(ctx, txID, started)
	if err != nil {
		return fmt.Errorf("failed to wait for transaction seal: %w", err)
	}

	return nil
}

// Voted returns true if we have successfully submitted a vote to the
// cluster QC aggregator smart contract for the current epoch.
func (c *QCContractClient) Voted(ctx context.Context) (bool, error) {

	// execute script to read if voted
	template := templates.GenerateGetNodeHasVotedScript(c.env)
	hasVoted, err := c.FlowClient.ExecuteScriptAtLatestBlock(ctx, template, []cadence.Value{cadence.String(c.nodeID.String())})
	if err != nil {
		return false, fmt.Errorf("could not execute voted script: %w", err)
	}

	// check if node has voted
	if !hasVoted.(cadence.Bool) {
		return false, nil
	}

	return true, nil
}
