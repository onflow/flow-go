package dkg

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/onflow/cadence"
	"github.com/onflow/crypto"
	"github.com/onflow/flow-core-contracts/lib/go/templates"

	sdk "github.com/onflow/flow-go-sdk"
	sdkcrypto "github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-go/model/flow"
	model "github.com/onflow/flow-go/model/messages"
	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/epochs"
)

// submitResultV1Script is a version of the DKG result submission script compatible with
// Protocol Version 1 (omitting DKG index map field).
// Deprecated:
// TODO(mainnet27, #6792): remove
//
//go:embed dkg_submit_result_v1.cdc
var submitResultV1Script string

//go:embed dkg_check_v2.cdc
var checkV2Script string

// Client is a client to the Flow DKG contract. Allows functionality to Broadcast,
// read a Broadcast and submit the final result of the DKG protocol
type Client struct {
	epochs.BaseClient

	env templates.Environment
}

var _ module.DKGContractClient = (*Client)(nil)

// NewClient initializes a new client to the Flow DKG contract
func NewClient(
	log zerolog.Logger,
	flowClient module.SDKClientWrapper,
	flowClientANID flow.Identifier,
	signer sdkcrypto.Signer,
	dkgContractAddress,
	accountAddress string,
	accountKeyIndex uint32,
) *Client {

	log = log.With().
		Str("component", "dkg_contract_client").
		Str("flow_client_an_id", flowClientANID.String()).
		Logger()
	base := epochs.NewBaseClient(log, flowClient, accountAddress, accountKeyIndex, signer)

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
	value, err := c.FlowClient.ExecuteScriptAtBlockID(
		ctx,
		sdk.Identifier(referenceBlock),
		template,
		[]cadence.Value{
			cadence.NewInt(int(fromIndex)),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("could not execute read broadcast script: %w", err)
	}
	values := value.(cadence.Array).Values

	// unpack return from contract to `model.DKGMessage`
	messages := make([]model.BroadcastDKGMessage, 0, len(values))
	for _, val := range values {
		fields := cadence.FieldsMappedByName(val.(cadence.Struct))

		id, err := strconv.Unquote(fields["nodeID"].String())
		if err != nil {
			return nil, fmt.Errorf("could not unquote nodeID cadence string (%s): %w", id, err)
		}

		nodeID, err := flow.HexStringToIdentifier(id)
		if err != nil {
			return nil, fmt.Errorf("could not parse nodeID (%v): %w", val, err)
		}

		content := fields["content"]
		jsonString, err := strconv.Unquote(content.String())
		if err != nil {
			return nil, fmt.Errorf("could not unquote json string: %w", err)
		}

		var flowMsg model.BroadcastDKGMessage
		err = json.Unmarshal([]byte(jsonString), &flowMsg)
		if err != nil {
			return nil, fmt.Errorf("could not unmarshal dkg message: %w", err)
		}
		flowMsg.NodeID = nodeID
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
		SetComputeLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(account.Address, c.AccountKeyIndex, account.Keys[int(c.AccountKeyIndex)].SequenceNumber).
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
	err = tx.SignEnvelope(account.Address, c.AccountKeyIndex, c.Signer)
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

// submitResult_ProtocolV1 submits this node's locally computed DKG result (public key vector)
// to the FlowDKG smart contract, using the submission API associated with Protocol Version 1.
// Input public keys are expected to be either all nil or all non-nil.
// Deprecated: This is a temporary function to provide backward compatibility between Protocol Versions 1 and 2.
// Protocol Version 2 introduces the DKG index map, which must be included in result submissions (see SubmitParametersAndResult).
// TODO(mainnet27, #6792): remove this function
func (c *Client) submitResult_ProtocolV1(groupPublicKey crypto.PublicKey, publicKeys []crypto.PublicKey) error {
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
		SetScript([]byte(templates.ReplaceAddresses(submitResultV1Script, c.env))).
		SetComputeLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(account.Address, c.AccountKeyIndex, account.Keys[int(c.AccountKeyIndex)].SequenceNumber).
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
	err = tx.SignEnvelope(account.Address, c.AccountKeyIndex, c.Signer)
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

// checkDKGContractV2Compatible executes a script which imports a type only defined in the
// FlowDKG smart contract version associated with Protocol Version 2.
// By observing the result of this script execution, we can infer the current FlowDKG version.
// Deprecated:
// TODO(mainnet27, #6792): remove this function
// Returns:
//   - (true, nil) if the smart contract is certainly v2-compatible (script succeeds)
//   - (false, nil) if the smart contract is certainly not v2-compatible (script fails with expected error)
//   - (false, error) if we don't know whether the smart contract is v2-compatible (script fails with unexpected error)
func (c *Client) checkDKGContractV2Compatible(ctx context.Context) (bool, error) {
	script := []byte(templates.ReplaceAddresses(checkV2Script, c.env))
	_, err := c.FlowClient.ExecuteScriptAtLatestBlock(ctx, script, nil)
	if err == nil {
		return true, nil
	}
	v1ErrorSubstring := "cannot find type in this scope: `FlowDKG.ResultSubmission`"
	if strings.Contains(err.Error(), v1ErrorSubstring) {
		return false, nil
	}
	return false, fmt.Errorf("unable to determine FlowDKG compatibility: %w", err)
}

// SubmitParametersAndResult posts the DKG setup parameters (`flow.DKGIndexMap`) and the node's locally-computed DKG result to
// the DKG white-board smart contract. The DKG results are the node's local computation of the group public key and the public
// key shares. Serialized public keys are encoded as lower-case hex strings.
// Conceptually the flow.DKGIndexMap is not an output of the DKG protocol. Rather, it is part of the configuration/initialization
// information of the DKG. Before an epoch transition on the happy path (using the data in the EpochSetup event), each consensus
// participant locally fixes the DKG committee 𝒟 including the respective nodes' order to be identical to the consensus
// committee 𝒞. However, in case of a failed epoch transition, we desire the ability to manually provide the result of a successful
// DKG for the immediately next epoch (so-called recovery epoch). The DKG committee 𝒟 must have a sufficiently large overlap with
// the recovery epoch's consensus committee 𝒞 -- though for flexibility, we do *not* want to require that both committees are identical.
// Therefore, we need to explicitly specify the DKG committee 𝒟 on the fallback path. For uniformity of implementation, we do the
// same also on the happy path.
func (c *Client) SubmitParametersAndResult(indexMap flow.DKGIndexMap, groupPublicKey crypto.PublicKey, publicKeys []crypto.PublicKey) error {
	started := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), epochs.TransactionSubmissionTimeout)
	defer cancel()

	// TODO(mainnet27, #6792): remove this code block
	{
		v2Compatible, err := c.checkDKGContractV2Compatible(ctx)
		// CASE 1: we know FlowDKG is not v2 compatible - fallback to v1 submission
		if !v2Compatible && err == nil {
			c.Log.Debug().Msg("detected FlowDKG version incompatible with protocol v2 - falling back to v1 result submission")
			return c.submitResult_ProtocolV1(groupPublicKey, publicKeys)
		}
		// CASE 2: we aren't sure whether FlowDKG is v2 compatible - fallback to v1 submission
		if err != nil {
			c.Log.Warn().Err(err).Msg("unable to determine FlowDKG compatibility - falling back to v1 result submission")
			return c.submitResult_ProtocolV1(groupPublicKey, publicKeys)
		}
		// CASE 3: FlowDKG is compatible with v2 - proceed with happy path logic
	}

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
		SetComputeLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(account.Address, c.AccountKeyIndex, account.Keys[int(c.AccountKeyIndex)].SequenceNumber).
		SetPayer(account.Address).
		AddAuthorizer(account.Address)

	trimmedGroupHexString := trim0x(groupPublicKey.String())
	cdcGroupString, err := cadence.NewString(trimmedGroupHexString)
	if err != nil {
		return fmt.Errorf("could not convert group key to cadence: %w", err)
	}

	// setup first arg - group key
	err = tx.AddArgument(cdcGroupString)
	if err != nil {
		return fmt.Errorf("could not add argument to transaction: %w", err)
	}

	cdcPublicKeys := make([]cadence.Value, 0, len(publicKeys))
	for _, publicKey := range publicKeys {
		// append individual public keys
		trimmedHexString := trim0x(publicKey.String())
		cdcPubKey, err := cadence.NewString(trimmedHexString)
		if err != nil {
			return fmt.Errorf("could not convert pub keyshare to cadence: %w", err)
		}
		cdcPublicKeys = append(cdcPublicKeys, cdcPubKey)
	}

	// setup second arg - array of public keys
	err = tx.AddArgument(cadence.NewArray(cdcPublicKeys))
	if err != nil {
		return fmt.Errorf("could not add argument to transaction: %w", err)
	}

	cdcIndexMap := make([]cadence.KeyValuePair, 0, len(indexMap))
	for nodeID, dkgIndex := range indexMap {
		cdcIndexMap = append(cdcIndexMap, cadence.KeyValuePair{
			Key:   cadence.String(nodeID.String()),
			Value: cadence.NewInt(dkgIndex),
		})
	}

	// setup third arg - IndexMap
	err = tx.AddArgument(cadence.NewDictionary(cdcIndexMap))
	if err != nil {
		return fmt.Errorf("could not add argument to transaction: %w", err)
	}

	// sign envelope using account signer
	err = tx.SignEnvelope(account.Address, c.AccountKeyIndex, c.Signer)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	c.Log.Info().Str("tx_id", tx.ID().Hex()).Msg("sending SubmitParametersAndResult transaction")
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

// SubmitEmptyResult submits an empty result of the DKG protocol. The empty result is obtained by a node when
// it realizes locally that its DKG participation was unsuccessful (either because the DKG failed as a whole,
// or because the node received too many byzantine inputs). However, a node obtaining an empty result can
// happen in both cases of the DKG succeeding or failing. For further details, please see:
// https://flowfoundation.notion.site/Random-Beacon-2d61f3b3ad6e40ee9f29a1a38a93c99c
// Honest nodes would call `SubmitEmptyResult` strictly after the final phase has ended if DKG has ended.
// Though, `SubmitEmptyResult` also supports implementing byzantine participants for testing that submit an
// empty result too early (intentional protocol violation), *before* the final DKG phase concluded.
// SubmitEmptyResult must be called strictly after the final phase has ended if DKG has failed.
func (c *Client) SubmitEmptyResult() error {
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
		SetScript(templates.GenerateSendEmptyDKGFinalSubmissionScript(c.env)).
		SetComputeLimit(9999).
		SetReferenceBlockID(latestBlock.ID).
		SetProposalKey(account.Address, c.AccountKeyIndex, account.Keys[int(c.AccountKeyIndex)].SequenceNumber).
		SetPayer(account.Address).
		AddAuthorizer(account.Address)

	// sign envelope using account signer
	err = tx.SignEnvelope(account.Address, c.AccountKeyIndex, c.Signer)
	if err != nil {
		return fmt.Errorf("could not sign transaction: %w", err)
	}

	c.Log.Info().Str("tx_id", tx.ID().Hex()).Msg("sending SubmitEmptyResult transaction")
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
