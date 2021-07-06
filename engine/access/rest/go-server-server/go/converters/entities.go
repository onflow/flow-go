package swagger

import (
	"encoding/base64"
	"encoding/hex"
	"strconv"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/model/flow"
)

func convertSignature(signature crypto.Signature) string {
	return base64.RawStdEncoding.EncodeToString(signature)
}

func convertAddress(address flow.Address) string {
	return hex.EncodeToString(address[:])
}

func convertIdentifier(identifier flow.Identifier) string {
	return hex.EncodeToString(identifier[:])
}

func convertProposalKey(proposalKey *flow.ProposalKey) *TransactionProposalKey {
	return &TransactionProposalKey{
		Address:        convertAddress(proposalKey.Address),
		KeyIndex:       strconv.FormatUint(proposalKey.KeyIndex, 10),
		SequenceNumber: strconv.FormatUint(proposalKey.SequenceNumber, 10),
	}
}

func convertTransactionSignature(transactionSignature *flow.TransactionSignature) *TransactionSignature {
	return &TransactionSignature{
		Address:   convertAddress(transactionSignature.Address),
		KeyIndex:  strconv.FormatUint(transactionSignature.KeyIndex, 10),
		Signature: convertSignature(transactionSignature.Signature),
	}
}

func convertTransactionBody(transaction *flow.TransactionBody) *EntitiesTransaction {
	arguments := make([]string, len(transaction.Arguments))
	for i, argument := range transaction.Arguments {
		arguments[i] = base64.RawStdEncoding.EncodeToString(argument)
	}

	authorizers := make([]string, len(transaction.Authorizers))
	for i, address := range transaction.Authorizers {
		authorizers[i] = convertAddress(address)
	}

	payloadSignatures := make([]TransactionSignature, len(transaction.PayloadSignatures))
	for i, signature := range transaction.PayloadSignatures {
		payloadSignatures[i] = convertTransactionSignature(&signature)
	}

	envelopeSignatures := make([]TransactionSignature, len(transaction.EnvelopeSignatures))
	for i, signature := range transaction.EnvelopeSignatures {
		envelopeSignatures[i] = convertTransactionSignature(&signature)
	}

	return &EntitiesTransaction{
		Script:             hex.EncodeToString(transaction.Script),
		Arguments:          arguments,
		ReferenceBlockId:   convertIdentifier(transaction.ReferenceBlockID),
		GasLimit:           strconv.FormatUint(transaction.GasLimit, 10),
		ProposalKey:        convertProposalKey(&transaction.ProposalKey),
		Payer:              convertAddress(transaction.Payer),
		Authorizers:        authorizers,
		PayloadSignatures:  payloadSignatures,
		envelopeSignatures: envelopeSignatures,
	}
}

func convertCollectionGuarantee(collectionGuarantee *flow.CollectionGuarantee) *EntitiesCollectionGuarantee {
	return &EntitiesCollectionGuarantee{
		CollectionId: convertIdentifier(collectionGuarantee.CollectionID),
		Signature:    convertSignature(collectionGuarantee.Signature),
	}
}

func convertSeal(seal *flow.Seal) *EntitiesBlockSeal {
	return &EntitiesBlockSeal{
		BlockId:            convertIdentifier(seal.BlockID),
		ExecutionReceiptId: convertIdentifier(seal.ResultID),
		// ExecutionReceiptSignatures:
		// ResultApprovalSignatures:
	}
}

func convertBlock(block *flow.Block) *EntitiesBlock {
	collectionGuarantees := make([]EntitiesCollectionGuarantee, len(block.Payload.Guarantees))
	for i, guarantee := range block.Payload.Guarantees {
		collectionGuarantees[i] = convertCollectionGuarantee(guarantee)
	}

	blockSeals := make([]EntitiesBlockSeal, len(block.Payload.Seals))
	for i, seal := range block.Payload.Seals {
		blockSeals[i] = convertSeal(seal)
	}

	return &EntitiesBlock{
		Id:                   convertIdentifier(block.ID()),
		ParentId:             convertIdentifier(block.Header.ParentID),
		Height:               strconv.FormatUint(block.Header.Height, 10),
		Timestamp:            block.Header.Timestamp, // TODO: how should JSON format this?
		CollectionGuarantees: collectionGuarantees,
		BlockSeals:           blockSeals,
		ParentVoterSignature: convertSignature(block.Header.ParentVoterSig),
	}
}

func convertHeader(header *flow.Header) *EntitiesBlockHeader {
	return &EntitiesBlockHeader{
		Id:        convertIdentifier(header.ID()),
		ParentId:  convertIdentifier(header.ParentID),
		Height:    strconv.FormatUint(header.Height, 10),
		Timestamp: header.Timestamp, // TODO
	}
}

func convertLightCollection(collection *flow.LightCollection) *EntitiesCollection {
	transactionIds := make([]string, len(collection.Transactions))
	for i, identifier := range collection.Transactions {
		transactionIds[i] = convertIdentifier(identifier)
	}

	return &EntitiesCollection{
		Id:             convertIdentifier(collection.ID()),
		TransactionIds: transactionIds,
	}
}

func convertEvent(event *flow.Event) *EntitiesEvent {
	return &EntitiesEvent{
		Type_:            event.Type, // TODO: convertEventType?
		Transactionid:    convertIdentifier(event.TransactionID),
		TransactionIndex: strconv.FormatUint(uint64(event.TransactionIndex), 10),
		EventIndex:       strconv.FormatUint(uint64(event.EventIndex), 10),
		Payload:          base64.RawStdEncoding.EncodeToString(event.Payload),
	}
}

func convertTransactionStatus(transactionStatus flow.TransactionStatus) EntitiesTransactionStatus {
	// TODO????
	return EntitiesTransactionStatus(flow.TransactionStatus)
}

func convertTransactionResult(transactionResult *TransactionResult) *AccessTransactionResultResponse {

	events := make([]EntitiesEvent, len(transactionResult.Events))
	for i, event := range transactionResult.Events {
		events[i] = convertEvent(event)
	}

	return &AccessTransactionResultResponse{
		Status:       convertTransactionStatus(transactionResult.Status),
		StatusCode:   strconv.FormatUint(transactionResult.StatusCode, 10),
		ErrorMessage: transactionResult.ErrorMessage,
		Events:       events,
		BlockId:      convertIdentifier(transactionResult.BlockID),
	}
}

func convertAccountPublicKey(accountPublicKey *flow.AccountPublicKey) *EntitiesAccountKey {
	// TODO

	// type AccountPublicKey struct {
	// 	Index     int
	// 	PublicKey crypto.PublicKey
	// 	SignAlgo  crypto.SigningAlgorithm
	// 	HashAlgo  hash.HashingAlgorithm
	// 	SeqNumber uint64
	// 	Weight    int
	// 	Revoked   bool
	// }

	// type EntitiesAccountKey struct {
	// 	Index int64 `json:"index,omitempty"`
	// 	PublicKey string `json:"publicKey,omitempty"`
	// 	SignAlgo int64 `json:"signAlgo,omitempty"`
	// 	HashAlgo int64 `json:"hashAlgo,omitempty"`
	// 	Weight int64 `json:"weight,omitempty"`
	// 	SequenceNumber int64 `json:"sequenceNumber,omitempty"`
	// 	Revoked bool `json:"revoked,omitempty"`
	// }
}

func convertAccount(account *flow.Account) *EntitiesAccount {
	keys := make([]EntitiesAccountKey, len(account.Keys))
	for i, key := range account.Keys {
		keys[i] = convertAccountPublicKey(key)
	}

	contracts := make(map[string]string)
	for key, value := range account.Contracts {
		contracts[key] = base64.RawStdEncoding.EncodeToString(value)
	}

	return &EntitiesAccount{
		Address:   convertAddress(account.Address),
		Balance:   strconv.FormatUint(account.Balance, 10),
		Keys:      keys,
		Contracts: contracts,
	}
}
