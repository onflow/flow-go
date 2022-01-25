package rest

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/model/flow"
)

// Converter provides functionality to convert from request models generated using
// open api spec and flow models.

// Flow section - converting request data to flow models with validation.

const maxAllowedScriptArgumentsCnt = 100
const signatureLength = 64
const maxAuthorizersCnt = 100
const MaxAllowedIDs = 50
const MaxAllowedHeights = 50

// Converters section from request data to typed models
// all methods names in this section are prefixed with 'to'
//

func toBase64(byteValue []byte) string {
	return base64.StdEncoding.EncodeToString(byteValue)
}

func fromBase64(bytesStr string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(bytesStr)
}

func toUint64(uint64Str string) (uint64, error) {
	return strconv.ParseUint(uint64Str, 10, 64)
}

func fromUint64(number uint64) string {
	return fmt.Sprintf("%d", number)
}

func toID(id string) (flow.Identifier, error) {
	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{64}$`, id)
	if !valid {
		return flow.Identifier{}, errors.New("invalid ID format")
	}

	flowID, err := flow.HexStringToIdentifier(id)
	if err != nil {
		return flow.Identifier{}, fmt.Errorf("invalid ID: %w", err)
	}
	return flowID, nil
}

func toIDs(ids []string) ([]flow.Identifier, error) {
	if len(ids) > MaxAllowedIDs {
		return nil, fmt.Errorf("at most %d IDs can be requested at a time", MaxAllowedIDs)
	}

	resIDs := make([]flow.Identifier, len(ids))
	for i, id := range ids {
		resID, err := toID(id)
		if err != nil {
			return nil, fmt.Errorf("invalid ID %s", id)
		}

		resIDs[i] = resID
	}

	return resIDs, nil
}

func toHeight(height string) (uint64, error) {
	h, err := strconv.ParseUint(height, 0, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid height format")
	}
	return h, nil
}

func toHeights(rawHeights []string) ([]uint64, error) {
	if len(rawHeights) > MaxAllowedHeights {
		return nil, fmt.Errorf("at most %d heights can be requested at a time", MaxAllowedHeights)
	}

	seenHeight := make(map[uint64]bool)
	heights := make([]uint64, len(rawHeights))
	for i, h := range rawHeights {
		conv, err := toHeight(h)
		if err != nil {
			return nil, fmt.Errorf("invalid height %s", h)
		}
		// dedup heights
		if !seenHeight[conv] {
			heights[i] = conv
			seenHeight[conv] = true
		}
	}

	return heights, nil
}

func toStringArray(in string) ([]string, error) {
	in = strings.TrimSuffix(in, "]")
	in = strings.TrimPrefix(in, "[")
	var out []string

	if len(in) == 0 {
		return []string{}, nil
	}

	if strings.Contains(in, ",") {
		out = strings.Split(in, ",")
	} else {
		out = strings.Fields(in)
	}

	// check for empty values
	for i, v := range out {
		if v == "" {
			return nil, fmt.Errorf("value at index %d is empty", i)
		}
	}

	return out, nil
}

func toAddress(address string) (flow.Address, error) {
	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{16}$`, address)
	if !valid {
		return flow.Address{}, fmt.Errorf("invalid address")
	}

	return flow.HexToAddress(address), nil
}

func toProposalKey(key *generated.ProposalKey) (flow.ProposalKey, error) {
	if key == nil {
		return flow.ProposalKey{}, fmt.Errorf("proposal key not provided")
	}

	address, err := toAddress(key.Address)
	if err != nil {
		return flow.ProposalKey{}, fmt.Errorf("invalid proposal address: %v", err)
	}

	keyIndex, err := toUint64(key.KeyIndex)
	if err != nil {
		return flow.ProposalKey{}, fmt.Errorf("invalid key index: %v", err)
	}

	keySeqNumber, err := toUint64(key.SequenceNumber)
	if err != nil {
		return flow.ProposalKey{}, fmt.Errorf("invalid key sequence number: %v", err)
	}

	return flow.ProposalKey{
		Address:        address,
		KeyIndex:       keyIndex,
		SequenceNumber: keySeqNumber,
	}, nil
}

func toSignature(signature string) ([]byte, error) {
	signatureBytes, err := fromBase64(signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}
	if len(signatureBytes) != signatureLength {
		return nil, errors.New("invalid signature length")
	}
	return signatureBytes, nil
}

func toTransactionSignature(transactionSignature *generated.TransactionSignature) (flow.TransactionSignature, error) {
	address, err := toAddress(transactionSignature.Address)
	if err != nil {
		return flow.TransactionSignature{}, err
	}

	signature, err := toSignature(transactionSignature.Signature)
	if err != nil {
		return flow.TransactionSignature{}, err
	}

	signerIndex, err := toUint64(transactionSignature.SignerIndex)
	if err != nil {
		return flow.TransactionSignature{}, err
	}

	keyIndex, err := toUint64(transactionSignature.KeyIndex)
	if err != nil {
		return flow.TransactionSignature{}, err
	}

	return flow.TransactionSignature{
		Address:     address,
		SignerIndex: int(signerIndex),
		KeyIndex:    keyIndex,
		Signature:   signature,
	}, nil
}

func toTransactionSignatures(sigs []generated.TransactionSignature) ([]flow.TransactionSignature, error) {
	// transaction signatures can be empty in that case we shouldn't convert them
	if len(sigs) == 0 {
		return nil, nil
	}
	signatures := make([]flow.TransactionSignature, len(sigs))
	for i, sig := range sigs {
		signature, err := toTransactionSignature(&sig)
		if err != nil {
			return nil, err
		}

		signatures[i] = signature
	}

	return signatures, nil
}

func toTransaction(tx generated.TransactionsBody) (flow.TransactionBody, error) {
	argLen := len(tx.Arguments)
	if argLen > maxAllowedScriptArgumentsCnt {
		return flow.TransactionBody{}, fmt.Errorf("too many arguments. Maximum arguments allowed: %d", maxAllowedScriptArgumentsCnt)
	}

	if tx.ProposalKey == nil {
		return flow.TransactionBody{}, fmt.Errorf("proposal key not provided")
	}
	if tx.Script == "" {
		return flow.TransactionBody{}, fmt.Errorf("script not provided")
	}
	if tx.Payer == "" {
		return flow.TransactionBody{}, fmt.Errorf("payer not provided")
	}
	if len(tx.Authorizers) == 0 {
		return flow.TransactionBody{}, fmt.Errorf("authorizers not provided")
	}
	if len(tx.Authorizers) > maxAuthorizersCnt {
		return flow.TransactionBody{}, fmt.Errorf("too many authorizers. Maximum authorizers allowed: %d", maxAuthorizersCnt)
	}
	if len(tx.EnvelopeSignatures) == 0 {
		return flow.TransactionBody{}, fmt.Errorf("envelope signatures not provided")
	}
	if tx.ReferenceBlockId == "" {
		return flow.TransactionBody{}, fmt.Errorf("reference block not provided")
	}

	// script arguments come in as a base64 encoded strings, decode base64 back to a string here
	var args [][]byte
	for i, arg := range tx.Arguments {
		decodedArg, err := fromBase64(arg)
		if err != nil {
			return flow.TransactionBody{}, fmt.Errorf("invalid argument encoding with index: %d", i)
		}
		args = append(args, decodedArg)
	}

	proposal, err := toProposalKey(tx.ProposalKey)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	payer, err := toAddress(tx.Payer)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	auths := make([]flow.Address, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		a, err := toAddress(auth)
		if err != nil {
			return flow.TransactionBody{}, err
		}

		auths[i] = a
	}

	payloadSigs, err := toTransactionSignatures(tx.PayloadSignatures)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	envelopeSigs, err := toTransactionSignatures(tx.EnvelopeSignatures)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	// script comes in as a base64 encoded string, decode base64 back to a string here
	script, err := fromBase64(tx.Script)
	if err != nil {
		return flow.TransactionBody{}, fmt.Errorf("invalid transaction script encoding")
	}

	blockID, err := toID(tx.ReferenceBlockId)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	gasLimit, err := toUint64(tx.GasLimit)
	if err != nil {
		return flow.TransactionBody{}, fmt.Errorf("invalid value for gas limit")
	}

	return flow.TransactionBody{
		ReferenceBlockID:   blockID,
		Script:             script,
		Arguments:          args,
		GasLimit:           gasLimit,
		ProposalKey:        proposal,
		Payer:              payer,
		Authorizers:        auths,
		PayloadSignatures:  payloadSigs,
		EnvelopeSignatures: envelopeSigs,
	}, nil
}

func toScriptArgs(script generated.ScriptsBody) ([][]byte, error) {
	args := make([][]byte, len(script.Arguments))
	for i, a := range script.Arguments {
		arg, err := fromBase64(a)
		if err != nil {
			return nil, fmt.Errorf("invalid script encoding")
		}
		args[i] = arg
	}
	return args, nil
}

func toScriptSource(script generated.ScriptsBody) ([]byte, error) {
	source, err := fromBase64(script.Script)
	if err != nil {
		return nil, fmt.Errorf("invalid script source encoding")
	}
	return source, nil
}

// Response section converting flow models to response models
// all methods names in this section are appended with 'response'
//

func proposalKeyResponse(key *flow.ProposalKey) *generated.ProposalKey {

	return &generated.ProposalKey{
		Address:        key.Address.String(),
		KeyIndex:       fromUint64(key.KeyIndex),
		SequenceNumber: fromUint64(key.SequenceNumber),
	}
}

func transactionSignatureResponse(signatures []flow.TransactionSignature) []generated.TransactionSignature {
	sigs := make([]generated.TransactionSignature, len(signatures))
	for i, sig := range signatures {
		sigs[i] = generated.TransactionSignature{
			Address:     sig.Address.String(),
			SignerIndex: fromUint64(uint64(sig.SignerIndex)),
			KeyIndex:    fromUint64(sig.KeyIndex),
			Signature:   toBase64(sig.Signature),
		}
	}

	return sigs
}

func transactionResponse(tx *flow.TransactionBody, txr *access.TransactionResult, link LinkGenerator) *generated.Transaction {
	args := make([]string, len(tx.Arguments))
	for i, arg := range tx.Arguments {
		args[i] = toBase64(arg)
	}

	auths := make([]string, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		auths[i] = auth.String()
	}

	var result *generated.TransactionResult
	expandable := new(generated.TransactionExpandable)
	// if transaction result is provided then add that to the response, else add the result link to the expandable
	if txr != nil {
		result = transactionResultResponse(txr, tx.ID(), link)
	} else {
		resultLink, _ := link.TransactionResultLink(tx.ID())
		expandable.Result = resultLink
	}

	self, _ := selfLink(tx.ID(), link.TransactionLink)

	return &generated.Transaction{
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

func eventResponse(event flow.Event) generated.Event {
	return generated.Event{
		Type_:            string(event.Type),
		TransactionId:    event.TransactionID.String(),
		TransactionIndex: fromUint64(uint64(event.TransactionIndex)),
		EventIndex:       fromUint64(uint64(event.EventIndex)),
		Payload:          toBase64(event.Payload),
	}
}

func eventsResponse(events []flow.Event) []generated.Event {
	eventsRes := make([]generated.Event, len(events))
	for i, e := range events {
		eventsRes[i] = eventResponse(e)
	}

	return eventsRes
}

func blockEventsResponse(block flow.BlockEvents) generated.BlockEvents {
	return generated.BlockEvents{
		BlockId:        block.BlockID.String(),
		BlockHeight:    fromUint64(block.BlockHeight),
		BlockTimestamp: block.BlockTimestamp,
		Events:         eventsResponse(block.Events),
		Links:          nil, // todo link
	}
}

func blocksEventsResponse(blocks []flow.BlockEvents) []generated.BlockEvents {
	events := make([]generated.BlockEvents, len(blocks))
	for i, b := range blocks {
		events[i] = blockEventsResponse(b)
	}

	return events
}

func statusResponse(status flow.TransactionStatus) generated.TransactionStatus {
	switch status {
	case flow.TransactionStatusExpired:
		return generated.EXPIRED
	case flow.TransactionStatusExecuted:
		return generated.EXECUTED
	case flow.TransactionStatusFinalized:
		return generated.FINALIZED
	case flow.TransactionStatusSealed:
		return generated.SEALED
	case flow.TransactionStatusPending:
		return generated.PENDING
	default:
		return ""
	}
}

func transactionResultResponse(txr *access.TransactionResult, txID flow.Identifier, link LinkGenerator) *generated.TransactionResult {
	status := statusResponse(txr.Status)

	self, _ := selfLink(txID, link.TransactionResultLink)

	return &generated.TransactionResult{
		BlockId:         txr.BlockID.String(),
		Status:          &status,
		ErrorMessage:    txr.ErrorMessage,
		ComputationUsed: fromUint64(0),
		Events:          eventsResponse(txr.Events),
		Links:           self,
	}
}

func blockHeaderResponse(flowHeader *flow.Header) *generated.BlockHeader {
	return &generated.BlockHeader{
		Id:                   flowHeader.ID().String(),
		ParentId:             flowHeader.ParentID.String(),
		Height:               fromUint64(flowHeader.Height),
		Timestamp:            flowHeader.Timestamp,
		ParentVoterSignature: toBase64(flowHeader.ParentVoterSigData),
	}
}

func blockPayloadResponse(flowPayload *flow.Payload) (*generated.BlockPayload, error) {
	blockSealResp, err := blockSealsResponse(flowPayload.Seals)
	if err != nil {
		return nil, err
	}
	return &generated.BlockPayload{
		CollectionGuarantees: collectionGuaranteesResponse(flowPayload.Guarantees),
		BlockSeals:           blockSealResp,
	}, nil
}

func collectionGuaranteesResponse(flowCollGuarantee []*flow.CollectionGuarantee) []generated.CollectionGuarantee {
	collectionGuarantees := make([]generated.CollectionGuarantee, len(flowCollGuarantee))
	for i, flowCollGuarantee := range flowCollGuarantee {
		collectionGuarantees[i] = collectionGuaranteeResponse(flowCollGuarantee)
	}
	return collectionGuarantees
}

func collectionGuaranteeResponse(flowCollGuarantee *flow.CollectionGuarantee) generated.CollectionGuarantee {
	signerIDs := make([]string, len(flowCollGuarantee.SignerIDs))
	for i, signerID := range flowCollGuarantee.SignerIDs {
		signerIDs[i] = signerID.String()
	}
	return generated.CollectionGuarantee{
		CollectionId: flowCollGuarantee.CollectionID.String(),
		SignerIds:    signerIDs,
		Signature:    toBase64(flowCollGuarantee.Signature.Bytes()),
	}
}

func blockSealsResponse(flowSeals []*flow.Seal) ([]generated.BlockSeal, error) {
	seals := make([]generated.BlockSeal, len(flowSeals))
	for i, seal := range flowSeals {
		sealResp, err := blockSealResponse(seal)
		if err != nil {
			return nil, err
		}
		seals[i] = sealResp
	}
	return seals, nil
}

func aggregatedApprovalSignaturesResponse(signatures []flow.AggregatedSignature) []generated.AggregatedSignature {
	response := make([]generated.AggregatedSignature, len(signatures))
	for i, signature := range signatures {

		verifierSignatures := make([]string, len(signature.VerifierSignatures))
		for y, verifierSignature := range signature.VerifierSignatures {
			verifierSignatures[y] = toBase64(verifierSignature.Bytes())
		}

		signerIDs := make([]string, len(signature.SignerIDs))
		for j, signerID := range signature.SignerIDs {
			signerIDs[j] = signerID.String()
		}

		response[i] = generated.AggregatedSignature{
			VerifierSignatures: verifierSignatures,
			SignerIds:          signerIDs,
		}
	}

	return response
}

func blockSealResponse(flowSeal *flow.Seal) (generated.BlockSeal, error) {
	finalState := ""
	if len(flowSeal.FinalState) > 0 { // todo(sideninja) this is always true?
		finalStateBytes, err := flowSeal.FinalState.MarshalJSON()
		if err != nil {
			return generated.BlockSeal{}, err
		}
		finalState = string(finalStateBytes)
	}

	return generated.BlockSeal{
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
) (generated.Collection, error) {

	expandable := &generated.CollectionExpandable{}

	self, err := selfLink(collection.ID(), link.CollectionLink)
	if err != nil {
		return generated.Collection{}, err
	}

	var transactions []generated.Transaction
	if expand[transactionsExpandable] {
		for _, t := range txs {
			transactions = append(transactions, *transactionResponse(t, nil, link))
		}
	} else {
		expandable.Transactions = make([]string, len(collection.Transactions))
		for i, tx := range collection.Transactions {
			expandable.Transactions[i], err = link.TransactionLink(tx)
			if err != nil {
				return generated.Collection{}, err
			}
		}
	}

	return generated.Collection{
		Id:           collection.ID().String(),
		Transactions: transactions,
		Links:        self,
		Expandable:   expandable,
	}, nil
}

func serviceEventListResponse(eventList flow.ServiceEventList) []generated.Event {
	events := make([]generated.Event, len(eventList))
	for i, e := range eventList {
		events[i] = generated.Event{
			Type_: e.Type,
		}
	}
	return events
}

func executionResultResponse(exeResult *flow.ExecutionResult, link LinkGenerator) (*generated.ExecutionResult, error) {
	self, err := selfLink(exeResult.ID(), link.ExecutionResultLink)
	if err != nil {
		return nil, err
	}

	return &generated.ExecutionResult{
		Id:      exeResult.ID().String(),
		BlockId: exeResult.BlockID.String(),
		Events:  serviceEventListResponse(exeResult.ServiceEvents),
		Links:   self,
	}, nil
}

func accountKeysResponse(keys []flow.AccountPublicKey) []generated.AccountPublicKey {
	keysResponse := make([]generated.AccountPublicKey, len(keys))
	for i, k := range keys {
		sigAlgo := generated.SigningAlgorithm(k.SignAlgo.String())
		hashAlgo := generated.HashingAlgorithm(k.HashAlgo.String())

		keysResponse[i] = generated.AccountPublicKey{
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

func accountResponse(flowAccount *flow.Account, link LinkGenerator, expand map[string]bool) (generated.Account, error) {

	account := generated.Account{
		Address: flowAccount.Address.String(),
		Balance: fromUint64(flowAccount.Balance),
	}

	// TODO: change spec to include default values (so that this doesn't need to be done)
	expandable := generated.AccountExpandable{
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
		return generated.Account{}, nil
	}
	account.Links = &generated.Links{
		Self: selfLink,
	}

	return account, nil
}

func blockResponse(blk *flow.Block, execResult *flow.ExecutionResult, link LinkGenerator, expand map[string]bool) (*generated.Block, error) {
	self, err := selfLink(blk.ID(), link.BlockLink)
	if err != nil {
		return nil, err
	}

	response := &generated.Block{
		Header:     blockHeaderResponse(blk.Header),
		Links:      self,
		Expandable: &generated.BlockExpandable{},
	}

	// add the payload to the response if it is specified as an expandable field
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
