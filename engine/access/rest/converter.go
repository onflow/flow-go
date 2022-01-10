package rest

import (
	"encoding/base64"
	"fmt"
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/models"
	"github.com/onflow/flow-go/model/flow"
)

// Converter provides functionality to convert from request models generated using
// open api spec and flow models.

// Flow section - converting request data to flow models with validation.

const MaxAllowedIDs = 50

// Converters section from request data to typed models
// all methods names in this section are prefixed with 'to'
//

func toBase64(byteValue []byte) string {
	return base64.StdEncoding.EncodeToString(byteValue)
}

func fromUint64(number uint64) string {
	return fmt.Sprintf("%d", number)
}

// Response section converting flow models to response models
// all methods names in this section are appended with 'response'
//

func proposalKeyResponse(key *flow.ProposalKey) *models.ProposalKey {

	return &models.ProposalKey{
		Address:        key.Address.String(),
		KeyIndex:       fromUint64(key.KeyIndex),
		SequenceNumber: fromUint64(key.SequenceNumber),
	}
}

func transactionSignatureResponse(signatures []flow.TransactionSignature) []models.TransactionSignature {
	sigs := make([]models.TransactionSignature, len(signatures))
	for i, sig := range signatures {
		sigs[i] = models.TransactionSignature{
			Address:     sig.Address.String(),
			SignerIndex: fromUint64(uint64(sig.SignerIndex)),
			KeyIndex:    fromUint64(sig.KeyIndex),
			Signature:   toBase64(sig.Signature),
		}
	}

	return sigs
}

func transactionResponse(tx *flow.TransactionBody, txr *access.TransactionResult, link LinkGenerator) *models.Transaction {
	args := make([]string, len(tx.Arguments))
	for i, arg := range tx.Arguments {
		args[i] = toBase64(arg)
	}

	auths := make([]string, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		auths[i] = auth.String()
	}

	var result *models.TransactionResult
	expandable := new(models.TransactionExpandable)
	// if transaction result is provided then add that to the response, else add the result link to the expandable
	if txr != nil {
		result = transactionResultResponse(txr, tx.ID(), link)
	} else {
		resultLink, _ := link.TransactionResultLink(tx.ID())
		expandable.Result = resultLink
	}

	self, _ := selfLink(tx.ID(), link.TransactionLink)

	return &models.Transaction{
		Id:                 tx.ID().String(),
		Script:             toBase64(tx.Script),
		Arguments:          args,
		ReferenceBlockId:   tx.ReferenceBlockID.String(),
		GasLimit:           fromUint64(tx.GasLimit),
		Payer:              tx.Payer.String(),
		ProposalKey:        proposalKeyResponse(&tx.ProposalKey),
		Authorizers:        auths,
		PayloadSignatures:  transactionSignatureResponse(tx.PayloadSignatures),
		EnvelopeSignatures: transactionSignatureResponse(tx.EnvelopeSignatures),
		Result:             result,
		Links:              self,
		Expandable:         expandable,
	}
}

func eventResponse(event flow.Event) models.Event {
	return models.Event{
		Type_:            string(event.Type),
		TransactionId:    event.TransactionID.String(),
		TransactionIndex: fromUint64(uint64(event.TransactionIndex)),
		EventIndex:       fromUint64(uint64(event.EventIndex)),
		Payload:          toBase64(event.Payload),
	}
}

func eventsResponse(events []flow.Event) []models.Event {
	eventsRes := make([]models.Event, len(events))
	for i, e := range events {
		eventsRes[i] = eventResponse(e)
	}

	return eventsRes
}

func statusResponse(status flow.TransactionStatus) models.TransactionStatus {
	switch status {
	case flow.TransactionStatusExpired:
		return models.EXPIRED
	case flow.TransactionStatusExecuted:
		return models.EXECUTED
	case flow.TransactionStatusFinalized:
		return models.FINALIZED
	case flow.TransactionStatusSealed:
		return models.SEALED
	case flow.TransactionStatusPending:
		return models.PENDING
	default:
		return ""
	}
}

func transactionResultResponse(txr *access.TransactionResult, txID flow.Identifier, link LinkGenerator) *models.TransactionResult {
	status := statusResponse(txr.Status)

	self, _ := selfLink(txID, link.TransactionResultLink)

	return &models.TransactionResult{
		BlockId:         txr.BlockID.String(),
		Status:          &status,
		ErrorMessage:    txr.ErrorMessage,
		ComputationUsed: fromUint64(0),
		Events:          eventsResponse(txr.Events),
		Links:           self,
	}
}

func blockHeaderResponse(flowHeader *flow.Header) *models.BlockHeader {
	return &models.BlockHeader{
		Id:                   flowHeader.ID().String(),
		ParentId:             flowHeader.ParentID.String(),
		Height:               fromUint64(flowHeader.Height),
		Timestamp:            flowHeader.Timestamp,
		ParentVoterSignature: toBase64(flowHeader.ParentVoterSigData),
	}
}

func blockPayloadResponse(flowPayload *flow.Payload) (*models.BlockPayload, error) {
	blockSealResp, err := blockSealsResponse(flowPayload.Seals)
	if err != nil {
		return nil, err
	}
	return &models.BlockPayload{
		CollectionGuarantees: collectionGuaranteesResponse(flowPayload.Guarantees),
		BlockSeals:           blockSealResp,
	}, nil
}

func collectionGuaranteesResponse(flowCollGuarantee []*flow.CollectionGuarantee) []models.CollectionGuarantee {
	collectionGuarantees := make([]models.CollectionGuarantee, len(flowCollGuarantee))
	for i, flowCollGuarantee := range flowCollGuarantee {
		collectionGuarantees[i] = collectionGuaranteeResponse(flowCollGuarantee)
	}
	return collectionGuarantees
}

func collectionGuaranteeResponse(flowCollGuarantee *flow.CollectionGuarantee) models.CollectionGuarantee {
	signerIDs := make([]string, len(flowCollGuarantee.SignerIDs))
	for i, signerID := range flowCollGuarantee.SignerIDs {
		signerIDs[i] = signerID.String()
	}
	return models.CollectionGuarantee{
		CollectionId: flowCollGuarantee.CollectionID.String(),
		SignerIds:    signerIDs,
		Signature:    toBase64(flowCollGuarantee.Signature.Bytes()),
	}
}

func blockSealsResponse(flowSeals []*flow.Seal) ([]models.BlockSeal, error) {
	seals := make([]models.BlockSeal, len(flowSeals))
	for i, seal := range flowSeals {
		sealResp, err := blockSealResponse(seal)
		if err != nil {
			return nil, err
		}
		seals[i] = sealResp
	}
	return seals, nil
}

func aggregatedApprovalSignaturesResponse(signatures []flow.AggregatedSignature) []models.AggregatedSignature {
	response := make([]models.AggregatedSignature, len(signatures))
	for i, signature := range signatures {

		verifierSignatures := make([]string, len(signature.VerifierSignatures))
		for y, verifierSignature := range signature.VerifierSignatures {
			verifierSignatures[y] = toBase64(verifierSignature.Bytes())
		}

		signerIDs := make([]string, len(signature.SignerIDs))
		for j, signerID := range signature.SignerIDs {
			signerIDs[j] = signerID.String()
		}

		response[i] = response.AggregatedSignature{
			VerifierSignatures: verifierSignatures,
			SignerIds:          signerIDs,
		}
	}

	return response
}

func blockSealResponse(flowSeal *flow.Seal) (models.BlockSeal, error) {
	finalState := ""
	if len(flowSeal.FinalState) > 0 { // todo(sideninja) this is always true?
		finalStateBytes, err := flowSeal.FinalState.MarshalJSON()
		if err != nil {
			return models.BlockSeal{}, err
		}
		finalState = string(finalStateBytes)
	}

	return models.BlockSeal{
		BlockId:                      flowSeal.BlockID.String(),
		ResultId:                     flowSeal.ResultID.String(),
		FinalState:                   finalState,
		AggregatedApprovalSignatures: aggregatedApprovalSignaturesResponse(flowSeal.AggregatedApprovalSigs),
	}, nil
}

func collectionResponse(
	collection *flow.LightCollection,
	txs []*flow.TransactionBody,
	link LinkGenerator,
	expand map[string]bool,
) (models.Collection, error) {

	expandable := &models.CollectionExpandable{}

	self, err := selfLink(collection.ID(), link.CollectionLink)
	if err != nil {
		return models.Collection{}, err
	}

	var transactions []models.Transaction
	if expand[transactionsExpandable] {
		for _, t := range txs {
			transactions = append(transactions, *transactionResponse(t, nil, link))
		}
	} else {
		expandable.Transactions = make([]string, len(collection.Transactions))
		for i, tx := range collection.Transactions {
			expandable.Transactions[i], err = link.TransactionLink(tx)
			if err != nil {
				return models.Collection{}, err
			}
		}
	}

	return models.Collection{
		Id:           collection.ID().String(),
		Transactions: transactions,
		Links:        self,
		Expandable:   expandable,
	}, nil
}

func serviceEventListResponse(eventList flow.ServiceEventList) []models.Event {
	events := make([]models.Event, len(eventList))
	for i, e := range eventList {
		events[i] = models.Event{
			Type_: e.Type,
		}
	}
	return events
}

func executionResultResponse(exeResult *flow.ExecutionResult, link LinkGenerator) (*models.ExecutionResult, error) {
	self, err := selfLink(exeResult.ID(), link.ExecutionResultLink)
	if err != nil {
		return nil, err
	}

	return &models.ExecutionResult{
		Id:      exeResult.ID().String(),
		BlockId: exeResult.BlockID.String(),
		Events:  serviceEventListResponse(exeResult.ServiceEvents),
		Links:   self,
	}, nil
}

func accountKeysResponse(keys []flow.AccountPublicKey) []models.AccountPublicKey {
	keysResponse := make([]models.AccountPublicKey, len(keys))
	for i, k := range keys {
		sigAlgo := models.SigningAlgorithm(k.SignAlgo.String())
		hashAlgo := models.HashingAlgorithm(k.HashAlgo.String())

		keysResponse[i] = models.AccountPublicKey{
			Index:            fromUint64(uint64(k.Index)),
			PublicKey:        k.PublicKey.String(),
			SigningAlgorithm: &sigAlgo,
			HashingAlgorithm: &hashAlgo,
			SequenceNumber:   fromUint64(k.SeqNumber),
			Weight:           fromUint64(uint64(k.Weight)),
			Revoked:          k.Revoked,
		}
	}

	return keysResponse
}

func accountResponse(flowAccount *flow.Account, link LinkGenerator, expand map[string]bool) (models.Account, error) {

	account := models.Account{
		Address: flowAccount.Address.String(),
		Balance: fromUint64(flowAccount.Balance),
	}

	// TODO: change spec to include default values (so that this doesn't need to be done)
	expandable := models.AccountExpandable{
		Keys:      "keys",
		Contracts: "contracts",
	}

	if expand[expandable.Keys] {
		account.Keys = accountKeysResponse(flowAccount.Keys)
		expandable.Keys = ""
	}

	if expand[expandable.Contracts] {
		contracts := make(map[string]string, len(flowAccount.Contracts))
		for name, code := range flowAccount.Contracts {
			contracts[name] = toBase64(code)
		}
		account.Contracts = contracts
		expandable.Contracts = ""
	}

	account.Expandable = &expandable

	selfLink, err := link.AccountLink(account.Address)
	if err != nil {
		return models.Account{}, nil
	}
	account.Links = &models.Links{
		Self: selfLink,
	}

	return account, nil
}

func blockResponse(blk *flow.Block, execResult *flow.ExecutionResult, link LinkGenerator, expand map[string]bool) (*models.Block, error) {
	self, err := selfLink(blk.ID(), link.BlockLink)
	if err != nil {
		return nil, err
	}

	response := &models.Block{
		Header:     blockHeaderResponse(blk.Header),
		Links:      self,
		Expandable: &models.BlockExpandable{},
	}

	// add the payload to the response if it is specified as an expandable field
	const ExpandableFieldPayload = "payload"
	if expand[ExpandableFieldPayload] {
		payloadResp, err := blockPayloadResponse(blk.Payload)
		if err != nil {
			return nil, err
		}
		response.Payload = payloadResp
	} else {
		// else add the payload expandable link
		payloadExpandable, err := link.PayloadLink(blk.ID())
		if err != nil {
			return nil, err
		}
		response.Expandable.Payload = payloadExpandable
	}

	// execution result might not yet exist
	if execResult != nil {
		// add the execution result to the response if it is specified as an expandable field
		const ExpandableExecutionResult = "execution_result"
		if expand[ExpandableExecutionResult] {
			execResultResp, err := executionResultResponse(execResult, link)
			if err != nil {
				return nil, err
			}
			response.ExecutionResult = execResultResp
		} else {
			// else add the execution result expandable link
			executionResultExpandable, err := link.ExecutionResultLink(execResult.ID())
			if err != nil {
				return nil, err
			}
			response.Expandable.ExecutionResult = executionResultExpandable
		}
	}

	// ship it
	return response, nil
}
