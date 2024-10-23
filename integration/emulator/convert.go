/*
 * Flow Emulator
 *
 * Copyright Flow Foundation
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package emulator

import (
	"fmt"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/ccf"

	sdk "github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/fvm"
	fvmerrors "github.com/onflow/flow-go/fvm/errors"
	flowgo "github.com/onflow/flow-go/model/flow"
)

func SDKIdentifierToFlow(sdkIdentifier sdk.Identifier) flowgo.Identifier {
	return flowgo.Identifier(sdkIdentifier)
}

func SDKIdentifiersToFlow(sdkIdentifiers []sdk.Identifier) []flowgo.Identifier {
	ret := make([]flowgo.Identifier, len(sdkIdentifiers))
	for i, sdkIdentifier := range sdkIdentifiers {
		ret[i] = SDKIdentifierToFlow(sdkIdentifier)
	}
	return ret
}

func FlowIdentifierToSDK(flowIdentifier flowgo.Identifier) sdk.Identifier {
	return sdk.Identifier(flowIdentifier)
}

func FlowIdentifiersToSDK(flowIdentifiers []flowgo.Identifier) []sdk.Identifier {
	ret := make([]sdk.Identifier, len(flowIdentifiers))
	for i, flowIdentifier := range flowIdentifiers {
		ret[i] = FlowIdentifierToSDK(flowIdentifier)
	}
	return ret
}

func SDKProposalKeyToFlow(sdkProposalKey sdk.ProposalKey) flowgo.ProposalKey {
	return flowgo.ProposalKey{
		Address:        SDKAddressToFlow(sdkProposalKey.Address),
		KeyIndex:       sdkProposalKey.KeyIndex,
		SequenceNumber: sdkProposalKey.SequenceNumber,
	}
}

func FlowProposalKeyToSDK(flowProposalKey flowgo.ProposalKey) sdk.ProposalKey {
	return sdk.ProposalKey{
		Address:        FlowAddressToSDK(flowProposalKey.Address),
		KeyIndex:       flowProposalKey.KeyIndex,
		SequenceNumber: flowProposalKey.SequenceNumber,
	}
}

func SDKAddressToFlow(sdkAddress sdk.Address) flowgo.Address {
	return flowgo.Address(sdkAddress)
}

func FlowAddressToSDK(flowAddress flowgo.Address) sdk.Address {
	return sdk.Address(flowAddress)
}

func SDKAddressesToFlow(sdkAddresses []sdk.Address) []flowgo.Address {
	ret := make([]flowgo.Address, len(sdkAddresses))
	for i, sdkAddress := range sdkAddresses {
		ret[i] = SDKAddressToFlow(sdkAddress)
	}
	return ret
}

func FlowAddressesToSDK(flowAddresses []flowgo.Address) []sdk.Address {
	ret := make([]sdk.Address, len(flowAddresses))
	for i, flowAddress := range flowAddresses {
		ret[i] = FlowAddressToSDK(flowAddress)
	}
	return ret
}

func SDKTransactionSignatureToFlow(sdkTransactionSignature sdk.TransactionSignature) flowgo.TransactionSignature {
	return flowgo.TransactionSignature{
		Address:     SDKAddressToFlow(sdkTransactionSignature.Address),
		SignerIndex: sdkTransactionSignature.SignerIndex,
		KeyIndex:    sdkTransactionSignature.KeyIndex,
		Signature:   sdkTransactionSignature.Signature,
	}
}

func FlowTransactionSignatureToSDK(flowTransactionSignature flowgo.TransactionSignature) sdk.TransactionSignature {
	return sdk.TransactionSignature{
		Address:     FlowAddressToSDK(flowTransactionSignature.Address),
		SignerIndex: flowTransactionSignature.SignerIndex,
		KeyIndex:    flowTransactionSignature.KeyIndex,
		Signature:   flowTransactionSignature.Signature,
	}
}

func SDKTransactionSignaturesToFlow(sdkTransactionSignatures []sdk.TransactionSignature) []flowgo.TransactionSignature {
	ret := make([]flowgo.TransactionSignature, len(sdkTransactionSignatures))
	for i, sdkTransactionSignature := range sdkTransactionSignatures {
		ret[i] = SDKTransactionSignatureToFlow(sdkTransactionSignature)
	}
	return ret
}

func FlowTransactionSignaturesToSDK(flowTransactionSignatures []flowgo.TransactionSignature) []sdk.TransactionSignature {
	ret := make([]sdk.TransactionSignature, len(flowTransactionSignatures))
	for i, flowTransactionSignature := range flowTransactionSignatures {
		ret[i] = FlowTransactionSignatureToSDK(flowTransactionSignature)
	}
	return ret
}

func SDKTransactionToFlow(sdkTx sdk.Transaction) *flowgo.TransactionBody {
	return &flowgo.TransactionBody{
		ReferenceBlockID:   SDKIdentifierToFlow(sdkTx.ReferenceBlockID),
		Script:             sdkTx.Script,
		Arguments:          sdkTx.Arguments,
		GasLimit:           sdkTx.GasLimit,
		ProposalKey:        SDKProposalKeyToFlow(sdkTx.ProposalKey),
		Payer:              SDKAddressToFlow(sdkTx.Payer),
		Authorizers:        SDKAddressesToFlow(sdkTx.Authorizers),
		PayloadSignatures:  SDKTransactionSignaturesToFlow(sdkTx.PayloadSignatures),
		EnvelopeSignatures: SDKTransactionSignaturesToFlow(sdkTx.EnvelopeSignatures),
	}
}

func FlowTransactionToSDK(flowTx flowgo.TransactionBody) sdk.Transaction {
	transaction := sdk.Transaction{
		ReferenceBlockID:   FlowIdentifierToSDK(flowTx.ReferenceBlockID),
		Script:             flowTx.Script,
		Arguments:          flowTx.Arguments,
		GasLimit:           flowTx.GasLimit,
		ProposalKey:        FlowProposalKeyToSDK(flowTx.ProposalKey),
		Payer:              FlowAddressToSDK(flowTx.Payer),
		Authorizers:        FlowAddressesToSDK(flowTx.Authorizers),
		PayloadSignatures:  FlowTransactionSignaturesToSDK(flowTx.PayloadSignatures),
		EnvelopeSignatures: FlowTransactionSignaturesToSDK(flowTx.EnvelopeSignatures),
	}
	return transaction
}

func FlowTransactionResultToSDK(result *access.TransactionResult) (*sdk.TransactionResult, error) {

	events, err := FlowEventsToSDK(result.Events)
	if err != nil {
		return nil, err
	}

	if result.ErrorMessage != "" {
		err = &ExecutionError{Code: int(result.StatusCode), Message: result.ErrorMessage}
	}

	sdkResult := &sdk.TransactionResult{
		Status:        sdk.TransactionStatus(result.Status),
		Error:         err,
		Events:        events,
		TransactionID: sdk.Identifier(result.TransactionID),
		BlockHeight:   result.BlockHeight,
		BlockID:       sdk.Identifier(result.BlockID),
	}

	return sdkResult, nil
}

func SDKEventToFlow(event sdk.Event) (flowgo.Event, error) {
	payload, err := ccf.EventsEncMode.Encode(event.Value)
	if err != nil {
		return flowgo.Event{}, err
	}

	return flowgo.Event{
		Type:             flowgo.EventType(event.Type),
		TransactionID:    SDKIdentifierToFlow(event.TransactionID),
		TransactionIndex: uint32(event.TransactionIndex),
		EventIndex:       uint32(event.EventIndex),
		Payload:          payload,
	}, nil
}

func FlowEventToSDK(flowEvent flowgo.Event) (sdk.Event, error) {
	cadenceValue, err := ccf.EventsDecMode.Decode(nil, flowEvent.Payload)
	if err != nil {
		return sdk.Event{}, err
	}

	cadenceEvent, ok := cadenceValue.(cadence.Event)
	if !ok {
		return sdk.Event{}, fmt.Errorf("cadence value not of type event: %s", cadenceValue)
	}

	return sdk.Event{
		Type:             string(flowEvent.Type),
		TransactionID:    FlowIdentifierToSDK(flowEvent.TransactionID),
		TransactionIndex: int(flowEvent.TransactionIndex),
		EventIndex:       int(flowEvent.EventIndex),
		Value:            cadenceEvent,
	}, nil
}

func FlowEventsToSDK(flowEvents []flowgo.Event) ([]sdk.Event, error) {
	ret := make([]sdk.Event, len(flowEvents))
	var err error
	for i, flowEvent := range flowEvents {
		ret[i], err = FlowEventToSDK(flowEvent)
		if err != nil {
			return nil, err
		}
	}
	return ret, nil
}

func FlowAccountPublicKeyToSDK(flowPublicKey flowgo.AccountPublicKey, index uint32) (sdk.AccountKey, error) {

	return sdk.AccountKey{
		Index:          index,
		PublicKey:      flowPublicKey.PublicKey,
		SigAlgo:        flowPublicKey.SignAlgo,
		HashAlgo:       flowPublicKey.HashAlgo,
		Weight:         flowPublicKey.Weight,
		SequenceNumber: flowPublicKey.SeqNumber,
		Revoked:        flowPublicKey.Revoked,
	}, nil
}

func SDKAccountKeyToFlow(key *sdk.AccountKey) (flowgo.AccountPublicKey, error) {

	return flowgo.AccountPublicKey{
		Index:     key.Index,
		PublicKey: key.PublicKey,
		SignAlgo:  key.SigAlgo,
		HashAlgo:  key.HashAlgo,
		Weight:    key.Weight,
		SeqNumber: key.SequenceNumber,
		Revoked:   key.Revoked,
	}, nil
}

func SDKAccountKeysToFlow(keys []*sdk.AccountKey) ([]flowgo.AccountPublicKey, error) {
	accountKeys := make([]flowgo.AccountPublicKey, len(keys))

	for i, key := range keys {
		accountKey, err := SDKAccountKeyToFlow(key)
		if err != nil {
			return nil, err
		}

		accountKeys[i] = accountKey
	}

	return accountKeys, nil
}

func FlowAccountPublicKeysToSDK(flowPublicKeys []flowgo.AccountPublicKey) ([]*sdk.AccountKey, error) {
	ret := make([]*sdk.AccountKey, len(flowPublicKeys))
	for i, flowPublicKey := range flowPublicKeys {
		v, err := FlowAccountPublicKeyToSDK(flowPublicKey, uint32(i))
		if err != nil {
			return nil, err
		}

		ret[i] = &v
	}
	return ret, nil
}

func FlowAccountToSDK(flowAccount flowgo.Account) (*sdk.Account, error) {
	sdkPublicKeys, err := FlowAccountPublicKeysToSDK(flowAccount.Keys)
	if err != nil {
		return &sdk.Account{}, err
	}

	return &sdk.Account{
		Address:   FlowAddressToSDK(flowAccount.Address),
		Balance:   flowAccount.Balance,
		Code:      nil,
		Keys:      sdkPublicKeys,
		Contracts: flowAccount.Contracts,
	}, nil
}

func SDKAccountToFlow(account *sdk.Account) (*flowgo.Account, error) {
	keys, err := SDKAccountKeysToFlow(account.Keys)
	if err != nil {
		return nil, err
	}

	return &flowgo.Account{
		Address:   SDKAddressToFlow(account.Address),
		Balance:   account.Balance,
		Keys:      keys,
		Contracts: account.Contracts,
	}, nil
}

func FlowLightCollectionToSDK(flowCollection flowgo.LightCollection) sdk.Collection {
	return sdk.Collection{
		TransactionIDs: FlowIdentifiersToSDK(flowCollection.Transactions),
	}
}

func VMTransactionResultToEmulator(
	txnId flowgo.Identifier,
	output fvm.ProcedureOutput,
) (
	*TransactionResult,
	error,
) {
	txID := FlowIdentifierToSDK(txnId)

	sdkEvents, err := FlowEventsToSDK(output.Events)
	if err != nil {
		return nil, err
	}

	return &TransactionResult{
		TransactionID:   txID,
		ComputationUsed: output.ComputationUsed,
		MemoryEstimate:  output.MemoryEstimate,
		Error:           VMErrorToEmulator(output.Err),
		Logs:            output.Logs,
		Events:          sdkEvents,
	}, nil
}

func VMErrorToEmulator(vmError fvmerrors.CodedError) error {
	if vmError == nil {
		return nil
	}

	return &FVMError{FlowError: vmError}
}

func ToStorableResult(
	output fvm.ProcedureOutput,
	blockID flowgo.Identifier,
	blockHeight uint64,
) (
	StorableTransactionResult,
	error,
) {
	var errorCode int
	var errorMessage string

	if output.Err != nil {
		errorCode = int(output.Err.Code())
		errorMessage = output.Err.Error()
	}

	return StorableTransactionResult{
		BlockID:      blockID,
		BlockHeight:  blockHeight,
		ErrorCode:    errorCode,
		ErrorMessage: errorMessage,
		Logs:         output.Logs,
		Events:       output.Events,
	}, nil
}
