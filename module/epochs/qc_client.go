package epochs

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/consensus/hotstuff/model"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module"
)

// TransactionSubmissionTimeout is the time after which we return an error.
const TransactionSubmissionTimeout = 5 * time.Minute

// QCContractClient is a client to the Quorum Certificate contract. Allows the client to
// functionality to submit a vote and check if collection node has voted already.
type QCContractClient struct {
	nodeID            flow.Identifier  // flow identifier of the collection node
	qcContractAddress string           // QuorumCertificate contract address
	accountKeyIndex   uint             // account key index
	signer            sdkcrypto.Signer // signer used to sign vote transaction

	account *sdk.Account // account belonging to the collection node
	client  module.SDKClientWrapper
}

// NewQCContractClient returns a new client to the Quorum Certificate contract
func NewQCContractClient(flowClient module.SDKClientWrapper, nodeID flow.Identifier, accountAddress string,
	accountKeyIndex uint, qcContractAddress string, signer sdkcrypto.Signer) (*QCContractClient, error) {

	// get account for given address
	account, err := flowClient.GetAccount(context.Background(), sdk.HexToAddress(accountAddress))
	if err != nil {
		return nil, fmt.Errorf("could not get account: %w", err)
	}

	// check if account key index within range of keys
	if len(account.Keys) <= int(accountKeyIndex) {
		return nil, fmt.Errorf("given account key index is bigger than the number of keys for this account")
	}

	return &QCContractClient{
		qcContractAddress: qcContractAddress,
		client:            flowClient,
		accountKeyIndex:   accountKeyIndex,
		signer:            signer,
		nodeID:            nodeID,
		account:           account,
	}, nil
}

// SubmitVote submits the given vote to the cluster QC aggregator smart
// contract. This function returns only once the transaction has been
// processed by the network. An error is returned if the transaction has
// failed and should be re-submitted.
func (c *QCContractClient) SubmitVote(ctx context.Context, vote *model.Vote) error {

	// add a timeout to the context
	ctx, cancel := context.WithTimeout(ctx, TransactionSubmissionTimeout)
	defer cancel()

	// get account for given address
	account, err := c.client.GetAccount(ctx, c.account.Address)
	if err != nil {
		return fmt.Errorf("could not get account: %w", err)
	}
	c.account = account

	// get latest sealed block to execute transaction
	latestBlock, err := c.client.GetLatestBlock(ctx, true)
	if err != nil {
		return fmt.Errorf("could not get latest block from node: %w", err)
	}

	// attach submit vote transaction template and build transaction
	seqNumber := c.account.Keys[int(c.accountKeyIndex)].SequenceNumber
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSubmitVoteScript(c.getEnvironment())).
		SetGasLimit(1000).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(c.account.Address, int(c.accountKeyIndex), seqNumber).
		SetPayer(c.account.Address).
		AddAuthorizer(c.account.Address)

	// add signature data to the transaction and submit to node
	err = tx.AddArgument(cadence.NewString(hex.EncodeToString(vote.SigData)))
	if err != nil {
		return fmt.Errorf("could not add raw vote data to transaction: %w", err)
	}

	// sign payload using account signer
	err = tx.SignPayload(c.account.Address, int(c.accountKeyIndex), c.signer)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	// submit signed transaction to node
	txID, err := c.submitTx(tx)
	if err != nil {
		return fmt.Errorf("failed to submit transaction: %w", err)
	}

	// wait for transaction to be sealed
	result := &sdk.TransactionResult{Status: sdk.TransactionStatusUnknown}
	for result.Status != sdk.TransactionStatusSealed {
		result, err = c.client.GetTransactionResult(ctx, txID)
		if err != nil {
			return fmt.Errorf("could not get transaction result: %w", err)
		}

		// if the transaction has expired we skip waiting for seal
		if result.Status == sdk.TransactionStatusExpired {
			return fmt.Errorf("submit vote transaction has expired")
		}

		// wait 1 second before trying again.
		time.Sleep(time.Second)
	}

	if result.Error != nil {
		return fmt.Errorf("error executing transaction: %w", result.Error)
	}

	return nil
}

// Voted returns true if we have successfully submitted a vote to the
// cluster QC aggregator smart contract for the current epoch.
func (c *QCContractClient) Voted(ctx context.Context) (bool, error) {

	val, err := jsoncdc.Decode(c.nodeID[:])
	if err != nil {
		return false, fmt.Errorf("could not deocde arguments: %w", err)
	}

	// execute script to read if voted
	template := templates.GenerateGetNodeHasVotedScript(c.getEnvironment())
	hasVoted, err := c.client.ExecuteScriptAtLatestBlock(ctx, template, []cadence.Value{val})
	if err != nil {
		return false, fmt.Errorf("could not execute voted script: %w", err)
	}

	// check if node has voted
	if !hasVoted.(cadence.Bool) {
		return false, nil
	}

	return true, nil
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

func (c *QCContractClient) getEnvironment() templates.Environment {
	// environment to override transaction template contract addresses
	return templates.Environment{
		QuorumCertificateAddress: c.qcContractAddress,
	}
}
