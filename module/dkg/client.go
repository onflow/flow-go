package dkg

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-core-contracts/lib/go/templates"
	"github.com/rs/zerolog"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow-go/crypto"
	model "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/epochs"
)

// Client is a client to the Flow DKG contract. Allows functionality to Broadcast,
// read a Broadcast and submit the final result of the DKG protocol
type Client struct {
	epochs.BaseClient

	env templates.Environment
}

// NewClient initializes a new client to the Flow DKG contract
func NewClient(
	log zerolog.Logger,
	flowClient module.SDKClientWrapper,
	signer sdkcrypto.Signer,
	dkgContractAddress,
	accountAddress string,
	accountKeyIndex uint,
) *Client {

	log = log.With().Str("component", "dkg_contract_client").Logger()
	base := epochs.NewBaseClient(log, flowClient, accountAddress, accountKeyIndex, signer, dkgContractAddress)

	env := templates.Environment{DkgAddress: dkgContractAddress}

	return &Client{
		BaseClient: *base,
		env:        env,
	}
}

// ReadBroadcast reads the broadcast messages from the smart contract.
// Messages are returned in the order in which they were received
// and stored in the smart contract
func (c *Client) ReadBroadcast(fromIndex uint, referenceBlock flow.Identifier) ([]model.BroadcastDKGMessage, error) {

	ctx := context.Background()

	// construct read latest broadcast messages transaction
	template := templates.GenerateGetDKGLatestWhiteBoardMessagesScript(c.env)
	value, err := c.FlowClient.ExecuteScriptAtBlockID(ctx,
		sdk.Identifier(referenceBlock), template, []cadence.Value{cadence.NewInt(int(fromIndex))})
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

// Broadcast broadcasts a message to all other nodes participating in the
// DKG. The message is broadcast by submitting a transaction to the DKG
// smart contract. An error is returned if the transaction has failed.
func (c *Client) Broadcast(msg model.BroadcastDKGMessage) error {

	started := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), epochs.TransactionSubmissionTimeout)
	defer cancel()

	// get account for given address
	account, err := c.GetAccount(ctx)
	if err != nil {
		return fmt.Errorf("could not get account details: %w", err)
	}

	// get latest finalized block to execute transaction
	latestBlock, err := c.FlowClient.GetLatestBlock(ctx, false)
	if err != nil {
		return fmt.Errorf("could not get latest block from node: %w", err)
	}

	// construct transaction to send dkg whiteboard message to contract
	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSendDKGWhiteboardMessageScript(c.env)).
		SetGasLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(account.Address, int(c.AccountKeyIndex), account.Keys[int(c.AccountKeyIndex)].SequenceNumber).
		SetPayer(account.Address).
		AddAuthorizer(account.Address)

	// json encode the DKG message
	jsonMessage, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("could not marshal DKG messages struct: %v", err)
	}

	// add dkg message json encoded string to tx args
	cdcMessage, err := cadence.NewString(string(jsonMessage))
	if err != nil {
		return fmt.Errorf("could not convert DKG message to cadence: %w", err)
	}
	err = tx.AddArgument(cdcMessage)
	if err != nil {
		return fmt.Errorf("could not add whiteboard dkg message to transaction: %w", err)
	}

	// sign envelope using account signer
	err = tx.SignEnvelope(account.Address, int(c.AccountKeyIndex), c.Signer)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	// submit signed transaction to node
	c.Log.Info().Str("tx_id", tx.ID().Hex()).Msg("sending Broadcast transaction")
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

// SubmitResult submits the final public result of the DKG protocol. This
// represents the group public key and the node's local computation of the
// public keys for each DKG participant. Serialized pub keys are encoded as hex.
func (c *Client) SubmitResult(groupPublicKey crypto.PublicKey, publicKeys []crypto.PublicKey) error {

	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), epochs.TransactionSubmissionTimeout)
	defer cancel()

	// get account for given address
	account, err := c.GetAccount(ctx)
	if err != nil {
		return fmt.Errorf("could not get account details: %w", err)
	}

	// get latest finalized block to execute transaction
	latestBlock, err := c.FlowClient.GetLatestBlock(ctx, false)
	if err != nil {
		return fmt.Errorf("could not get latest block from node: %w", err)
	}

	tx := sdk.NewTransaction().
		SetScript(templates.GenerateSendDKGFinalSubmissionScript(c.env)).
		SetGasLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(account.Address, int(c.AccountKeyIndex), account.Keys[int(c.AccountKeyIndex)].SequenceNumber).
		SetPayer(account.Address).
		AddAuthorizer(account.Address)

	// Note: We need to make sure that we pull the keys out in the same order that
	// we have done here. Group Public key first followed by the individual public keys
	finalSubmission := make([]cadence.Value, 0, len(publicKeys))

	// first append group public key
	if groupPublicKey != nil {
		trimmedGroupHexString := trim0x(groupPublicKey.String())
		cdcGroupString, err := cadence.NewString(trimmedGroupHexString)
		if err != nil {
			return fmt.Errorf("could not convert group key to cadence: %w", err)
		}
		finalSubmission = append(finalSubmission, cadence.NewOptional(cdcGroupString))
	} else {
		finalSubmission = append(finalSubmission, cadence.NewOptional(nil))
	}

	for _, publicKey := range publicKeys {

		// append individual public keys
		if publicKey != nil {
			trimmedHexString := trim0x(publicKey.String())
			cdcPubKey, err := cadence.NewString(trimmedHexString)
			if err != nil {
				return fmt.Errorf("could not convert pub keyshare to cadence: %w", err)
			}
			finalSubmission = append(finalSubmission, cadence.NewOptional(cdcPubKey))
		} else {
			finalSubmission = append(finalSubmission, cadence.NewOptional(nil))
		}
	}

	err = tx.AddArgument(cadence.NewArray(finalSubmission))
	if err != nil {
		return fmt.Errorf("could not add argument to transaction: %w", err)
	}

	// sign envelope using account signer
	err = tx.SignEnvelope(account.Address, int(c.AccountKeyIndex), c.Signer)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

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

// trim0x trims the `0x` if it exists from a hexadecimal string
// This method is required as the DKG contract expects key lengths of 192 bytes
// the `PublicKey.String()` method returns the hexadecimal string representation of the
// public key prefixed with `0x` resulting in length of 194 bytes.
func trim0x(hexString string) string {

	prefix := hexString[:2]
	if prefix == "0x" {
		return hexString[2:]
	}

	return hexString
}
