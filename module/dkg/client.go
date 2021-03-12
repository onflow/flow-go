package dkg

import (
	"context"
	"fmt"
	"time"

	"github.com/onflow/cadence"
	sdk "github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/client"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"google.golang.org/grpc"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/messages"
)

// Client is a client to the Flow DKG contract. Allows functionality to Broadcast,
// read a Broadcast and submit the final result of the DKG protocol
type Client struct {
	accessAddress      string
	dkgContractAddress string
	accountAddress     sdk.Address
	accountKeyIndex    uint
	flowClient         *client.Client
	signer             sdkcrypto.Signer

	env templates.Environment // the required contract addresses to be used by the Flow SDK
}

// NewClient initializes a new client to the Flow DKG contract
func NewClient(accessAddress, dkgContractAddress, accountAddress string, accountKeyIndex uint, flowClient *client.Client, signer sdkcrypto.Signer) *Client {
	return &Client{
		accessAddress:      accessAddress,
		flowClient:         flowClient,
		dkgContractAddress: dkgContractAddress,
		signer:             signer,
		accountKeyIndex:    accountKeyIndex,
		accountAddress:     sdk.HexToAddress(accountAddress),
		env:                templates.Environment{DkgAddress: dkgContractAddress},
	}
}

// Broadcast broadcasts a message to all other nodes participating in the
// DKG. The message is broadcast by submitting a transaction to the DKG
// smart contract. An error is returned if the transaction has failed has
// failed
func (c *Client) Broadcast(msg messages.DKGMessage) error {

	ctx := context.Background()

	// get account for given address
	account, err := c.flowClient.GetAccount(ctx, c.accountAddress, grpc.EmptyCallOption{})
	if err != nil {
		return fmt.Errorf("could not get account details: %v", err)
	}

	// get latest sealed block to execute transaction
	latestBlock, err := c.flowClient.GetLatestBlock(ctx, true)
	if err != nil {
		return fmt.Errorf("could not get latest block from node: %w", err)
	}

	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSendDKGWhiteboardMessageScript(c.env)).
		SetGasLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(c.accountAddress, int(c.accountKeyIndex), account.Keys[int(c.accountKeyIndex)].SequenceNumber).
		SetPayer(c.accountAddress).
		AddAuthorizer(c.accountAddress)

	err = tx.AddArgument(cadence.NewString(string(msg.Data)))
	if err != nil {
		return fmt.Errorf("could not add whiteboard dkg message to transaction: %v", err)
	}

	// sign payload using account signer
	err = tx.SignPayload(c.accountAddress, int(c.accountKeyIndex), c.signer)
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
		result, err = c.flowClient.GetTransactionResult(ctx, txID)
		if err != nil {
			return fmt.Errorf("could not get transaction result: %w", err)
		}

		// if the transaction has expired we skip waiting for seal
		if result.Status == sdk.TransactionStatusExpired {
			return fmt.Errorf("broadcast dkg message transaction has expired")
		}

		// wait 1 second before trying again.
		time.Sleep(time.Second)
	}

	return nil
}

// ReadBroadcast reads the broadcast messages from the smart contract.
// Messages are returned in the order in which they were broadcast (received
// and stored in the smart contract)
func (c *Client) ReadBroadcast(fromIndex uint, referenceBlock flow.Identifier) ([]messages.DKGMessage, error) {

	ctx := context.Background()

	template := templates.GenerateGetDKGLatestWhiteBoardMessagesScript(c.env)
	dkgMessages, err := c.flowClient.ExecuteScriptAtBlockID(ctx, sdk.Identifier(referenceBlock), template, []cadence.Value{cadence.NewInt(int(fromIndex))})
	if err != nil {
		return nil, fmt.Errorf("could not execute read broadcast script")
	}

	// TODO: convert from `cadence.Array` to `[]messages.DKGMessage`
	return dkgMessages.ToGoValue().([]messages.DKGMessage), nil
}

// SubmitResult submits the final public result of the DKG protocol. This
// represents the group public key and the node's local computation of the
// public keys for each DKG participant
func (c *Client) SubmitResult(groupPublicKey crypto.PublicKey, publicKeys []crypto.PublicKey) error {

	ctx := context.Background()

	// get account for given address
	account, err := c.flowClient.GetAccount(ctx, c.accountAddress, grpc.EmptyCallOption{})
	if err != nil {
		return fmt.Errorf("could not get account details: %v", err)
	}

	// get latest sealed block to execute transaction
	latestBlock, err := c.flowClient.GetLatestBlock(ctx, true)
	if err != nil {
		return fmt.Errorf("could not get latest block from node: %w", err)
	}

	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSendDKGFinalSubmissionScript(c.env)).
		SetGasLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(c.accountAddress, int(c.accountKeyIndex), account.Keys[int(c.accountKeyIndex)].SequenceNumber).
		SetPayer(c.accountAddress).
		AddAuthorizer(c.accountAddress)

	// create cadence array from public keys
	// Need to make sure that the order in which these are sent to the
	// contract are the same as when we pull them out
	finalSubmission := make([]cadence.Value, len(publicKeys))
	for _, publicKey := range publicKeys {
		finalSubmission = append(finalSubmission, cadence.NewString(publicKey.String()))
	}
	finalSubmission = append(finalSubmission, cadence.NewString(groupPublicKey.String()))

	err = tx.AddArgument(cadence.NewArray(finalSubmission))
	if err != nil {
		return fmt.Errorf("could not add argument to transaction: %v", err)
	}

	// sign payload using account signer
	err = tx.SignPayload(c.accountAddress, int(c.accountKeyIndex), c.signer)
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
		result, err = c.flowClient.GetTransactionResult(ctx, txID)
		if err != nil {
			return fmt.Errorf("could not get transaction result: %w", err)
		}

		// if the transaction has expired we skip waiting for seal
		if result.Status == sdk.TransactionStatusExpired {
			return fmt.Errorf("submit final dkg result transaction has expired")
		}

		// wait 1 second before trying again.
		time.Sleep(time.Second)
	}

	return nil
}

// submitTx submits a transaction to the flow network and checks that the
// transaction is signed
func (c *Client) submitTx(tx *sdk.Transaction) (sdk.Identifier, error) {
	ctx := context.Background()

	// check if the transaction has a signature
	if len(tx.EnvelopeSignatures) == 0 {
		return sdk.EmptyID, fmt.Errorf("can not submit an unsigned transaction")
	}

	// submit trnsaction to client
	err := c.flowClient.SendTransaction(ctx, *tx)
	if err != nil {
		return sdk.EmptyID, fmt.Errorf("failed to send transaction: %w", err)
	}

	return tx.ID(), nil
}
