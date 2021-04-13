package dkg

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"google.golang.org/grpc"

	"github.com/onflow/cadence"

	"github.com/onflow/flow-core-contracts/lib/go/templates"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
	model "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/epochs"
)

// Client is a client to the Flow DKG contract. Allows functionality to Broadcast,
// read a Broadcast and submit the final result of the DKG protocol
type Client struct {
	dkgContractAddress string
	accountAddress     sdk.Address
	accountKeyIndex    uint
	flowClient         module.SDKClientWrapper
	signer             sdkcrypto.Signer

	env templates.Environment // the required contract addresses to be used by the Flow SDK
}

// NewClient initializes a new client to the Flow DKG contract
func NewClient(flowClient module.SDKClientWrapper, signer sdkcrypto.Signer, dkgContractAddress, accountAddress string, accountKeyIndex uint) *Client {
	return &Client{
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
// smart contract. An error is returned if the transaction has failed.
func (c *Client) Broadcast(msg model.BroadcastDKGMessage) error {

	ctx, cancel := context.WithTimeout(context.Background(), epochs.TransactionSubmissionTimeout)
	defer cancel()

	// get account for given address
	account, err := c.flowClient.GetAccount(ctx, c.accountAddress, grpc.EmptyCallOption{})
	if err != nil {
		return fmt.Errorf("could not get account details: %w", err)
	}

	// get latest sealed block to execute transaction
	latestBlock, err := c.flowClient.GetLatestBlock(ctx, true)
	if err != nil {
		return fmt.Errorf("could not get latest block from node: %w", err)
	}

	// construct transaction to send dkg whiteboard message to contract
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSendDKGWhiteboardMessageScript(c.env)).
		SetGasLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(c.accountAddress, int(c.accountKeyIndex), account.Keys[int(c.accountKeyIndex)].SequenceNumber).
		SetPayer(c.accountAddress).
		AddAuthorizer(c.accountAddress)

	// json encode the DKG message
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("could not marshal DKG messages struct: %v", err)
	}

	// add dkg message json encoded string to tx args
	err = tx.AddArgument(cadence.NewString(string(data)))
	if err != nil {
		return fmt.Errorf("could not add whiteboard dkg message to transaction: %w", err)
	}

	// sign envelope using account signer
	err = tx.SignEnvelope(c.accountAddress, int(c.accountKeyIndex), c.signer)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	// submit signed transaction to node
	txID, err := c.submitTx(ctx, tx)
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

		// wait before trying again.
		time.Sleep(epochs.TransactionStatusRetryTimeout)
	}

	if result.Error != nil {
		return fmt.Errorf("error executing transaction: %w", result.Error)
	}

	return nil
}

// ReadBroadcast reads the broadcast messages from the smart contract.
// Messages are returned in the order in which they were broadcast (received
// and stored in the smart contract)
func (c *Client) ReadBroadcast(fromIndex uint, referenceBlock flow.Identifier) ([]model.BroadcastDKGMessage, error) {

	ctx := context.Background()

	// construct read latest broadcast messages transaction
	template := templates.GenerateGetDKGLatestWhiteBoardMessagesScript(c.env)
	value, err := c.flowClient.ExecuteScriptAtBlockID(ctx, sdk.Identifier(referenceBlock), template, []cadence.Value{cadence.NewInt(int(fromIndex))})
	if err != nil {
		return nil, fmt.Errorf("could not execute read broadcast script: %w", err)
	}
	values := value.(cadence.Array).Values

	// unpack return from contract to `model.DKGMessage`
	messages := make([]model.BroadcastDKGMessage, 0, len(values))
	for _, val := range values {

		content := val.(cadence.Struct).Fields[1]
		jsonString, err := strconv.Unquote(content.String())
		if err != nil {
			return nil, fmt.Errorf("could not unquote json string: %w", err)
		}

		var flowMsg model.BroadcastDKGMessage
		err = json.Unmarshal([]byte(jsonString), &flowMsg)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal dkg message: %w", err)
		}
		messages = append(messages, flowMsg)
	}

	return messages, nil
}

// SubmitResult submits the final public result of the DKG protocol. This
// represents the group public key and the node's local computation of the
// public keys for each DKG participant. Serialized pub keys are encoded as hex.
func (c *Client) SubmitResult(groupPublicKey crypto.PublicKey, publicKeys []crypto.PublicKey) error {

	ctx := context.Background()

	// get account for given address
	account, err := c.flowClient.GetAccount(ctx, c.accountAddress, grpc.EmptyCallOption{})
	if err != nil {
		return fmt.Errorf("could not get account details: %w", err)
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

	// Note: We need to make sure that we pull the keys out in the same order that
	// we have done here. Group Public key first followed by the individual public keys
	finalSubmission := make([]cadence.Value, 0, len(publicKeys))

	// initially append group public key
	trimmedGroupHexString := trim0x(groupPublicKey.String())
	finalSubmission = append(finalSubmission, cadence.NewString(trimmedGroupHexString))

	for _, publicKey := range publicKeys {
		trimmedHexString := trim0x(publicKey.String())
		finalSubmission = append(finalSubmission, cadence.NewString(trimmedHexString))
	}

	err = tx.AddArgument(cadence.NewArray(finalSubmission))
	if err != nil {
		return fmt.Errorf("could not add argument to transaction: %w", err)
	}

	// sign envelope using account signer
	err = tx.SignEnvelope(c.accountAddress, int(c.accountKeyIndex), c.signer)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	// submit signed transaction to node
	txID, err := c.submitTx(ctx, tx)
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
func (c *Client) submitTx(ctx context.Context, tx *sdk.Transaction) (sdk.Identifier, error) {

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

// trim0x trims the `0x` if it exists from a hexadecimal string
// This method is required as the DKG contract expects key legths of 192 bytes
// the `PublicKey.String()` method returns the hexadecimal string representation of the
// public key prefixed with `0x` resulting in length of 194 bytes.
func trim0x(hexString string) string {

	prefix := hexString[:2]
	if prefix == "0x" {
		return hexString[2:]
	}

	return hexString
}
