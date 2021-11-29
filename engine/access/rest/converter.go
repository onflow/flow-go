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
const maxSignatureLength = 64
const maxAuthorizersCnt = 100
const MaxAllowedIDs = 50 // todo(sideninja) discuss if we should restrict maximum on all IDs collection or is anywhere required more thant this
const MaxAllowedHeights = 50

//const MaxScriptLength =

var MaxAllowedBlockIDs = MaxAllowedIDs

func toBase64(byteValue []byte) string {
	return base64.StdEncoding.EncodeToString(byteValue)
}

func fromBase64(bytesStr string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(bytesStr)
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

func toIDs(ids string) ([]flow.Identifier, error) {
	// currently, the swagger generated Go REST client is incorrectly doing a `fmt.Sprintf("%v", id)` for the id slice
	// resulting in the client sending the ids in the format [id1 id2 id3...]. This is a temporary workaround to
	// accommodate the client for now. Issue to to fix the client: https://github.com/onflow/flow/issues/698
	ids = strings.TrimSuffix(ids, "]")
	ids = strings.TrimPrefix(ids, "[")

	reqIDs := strings.Split(ids, ",")

	if len(reqIDs) > MaxAllowedBlockIDs {
		return nil, fmt.Errorf("at most %d Block IDs can be requested at a time", MaxAllowedBlockIDs)
	}

	resIDs := make([]flow.Identifier, len(reqIDs))
	for i, id := range reqIDs {
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

func toHeights(height string) ([]uint64, error) {
	height = strings.TrimSuffix(height, "]")
	height = strings.TrimPrefix(height, "[")
	rawHeights := strings.Fields(height)

	if len(rawHeights) > MaxAllowedHeights {
		return nil, fmt.Errorf("at most %d heights can be requested at a time", MaxAllowedHeights)
	}

	heights := make([]uint64, len(rawHeights))
	for i, h := range rawHeights {
		conv, err := toHeight(h)
		if err != nil {
			return nil, fmt.Errorf("invalid height %s", h)
		}
		heights[i] = conv
	}

	return heights, nil
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
		return flow.ProposalKey{}, err
	}

	return flow.ProposalKey{
		Address:        address,
		KeyIndex:       uint64(key.KeyIndex),
		SequenceNumber: uint64(key.SequenceNumber),
	}, nil
}

func toSignature(signature string) ([]byte, error) {
	signatureBytes, err := fromBase64(signature)
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}
	if len(signatureBytes) > maxSignatureLength {
		return nil, errors.New("signature length invalid")
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

	return flow.TransactionSignature{
		Address:     address,
		SignerIndex: int(transactionSignature.SignerIndex),
		KeyIndex:    uint64(transactionSignature.KeyIndex),
		Signature:   signature,
	}, nil
}

func toTransactionSignatures(sigs []generated.TransactionSignature) ([]flow.TransactionSignature, error) {
	signatures := make([]flow.TransactionSignature, len(sigs))
	for x, sig := range sigs {
		signature, err := toTransactionSignature(&sig)
		if err != nil {
			return nil, err
		}

		signatures[x] = signature
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
	if len(tx.EnvelopeSignatures) == 0 {
		return flow.TransactionBody{}, fmt.Errorf("envelope sigantures not provided")
	}
	if tx.ReferenceBlockId == "" {
		return flow.TransactionBody{}, fmt.Errorf("reference block not provided")
	}
	if len(tx.Authorizers) > maxAuthorizersCnt {
		return flow.TransactionBody{}, fmt.Errorf("too many authorizers. Maximum authorizers allowed: %d", maxAuthorizersCnt)
	}
	//if len(tx.Script) > MaxScriptLength { todo(sideninja) define limit
	//	return flow.TransactionBody{}, fmt.Errorf("script exceeding the size limit")
	//}

	// script arguments come in as a base64 encoded strings, decode base64 back to a string here
	args := make([][]byte, argLen)
	for _, arg := range tx.Arguments {
		decodedArg, err := fromBase64(arg)
		if err != nil {
			return flow.TransactionBody{}, fmt.Errorf("invalid arguments encoding")
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
	for x, auth := range tx.Authorizers {
		a, err := toAddress(auth)
		if err != nil {
			return flow.TransactionBody{}, err
		}

		auths[x] = a
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

	return flow.TransactionBody{
		ReferenceBlockID:   blockID,
		Script:             script,
		Arguments:          args,
		GasLimit:           uint64(tx.GasLimit),
		ProposalKey:        proposal,
		Payer:              payer,
		Authorizers:        auths,
		PayloadSignatures:  payloadSigs,
		EnvelopeSignatures: envelopeSigs,
	}, nil
}

func toScriptArgs(script generated.ScriptsBody) ([][]byte, error) {
	// todo(sideninja) validate
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

// Response section - converting flow models to response models.

func proposalKeyResponse(key *flow.ProposalKey) *generated.ProposalKey {
	return &generated.ProposalKey{
		Address:        key.Address.String(),
		KeyIndex:       int32(key.KeyIndex),
		SequenceNumber: int32(key.SequenceNumber),
	}
}

func transactionSignatureResponse(signatures []flow.TransactionSignature) []generated.TransactionSignature {
	sigs := make([]generated.TransactionSignature, len(signatures))
	for i, sig := range signatures {
		sigs[i] = generated.TransactionSignature{
			Address:     sig.Address.String(),
			SignerIndex: int32(sig.SignerIndex),
			KeyIndex:    int32(sig.KeyIndex),
			Signature:   toBase64(sig.Signature),
		}
	}

	return sigs
}

func transactionResponse(tx *flow.TransactionBody, link LinkGenerator) *generated.Transaction {
	var args []string
	for _, arg := range tx.Arguments {
		args = append(args, toBase64(arg))
	}

	var auths []string
	for _, auth := range tx.Authorizers {
		auths = append(auths, auth.String())
	}

	return &generated.Transaction{
		Id:                 tx.ID().String(),
		Script:             toBase64(tx.Script),
		Arguments:          args,
		ReferenceBlockId:   tx.ReferenceBlockID.String(),
		GasLimit:           int32(tx.GasLimit), // todo(sideninja) make sure this is ok
		Payer:              tx.Payer.String(),
		ProposalKey:        proposalKeyResponse(&tx.ProposalKey),
		Authorizers:        auths,
		PayloadSignatures:  transactionSignatureResponse(tx.PayloadSignatures),
		EnvelopeSignatures: transactionSignatureResponse(tx.EnvelopeSignatures),
		Result:             nil, // todo(sideninja) should we provide result, maybe have a wait for result http long pulling system would be super helpful (with reasonable timeout) but careful about resources and dos
		Links:              transactionLink(tx.ID(), link),
	}
}

func transactionLink(id flow.Identifier, link LinkGenerator) *generated.Links {
	self, _ := link.TransactionLink(id)

	return &generated.Links{
		Self: self,
	}
}

func eventResponse(event flow.Event) generated.Event {
	return generated.Event{
		Type_:            string(event.Type),
		TransactionId:    event.TransactionID.String(),
		TransactionIndex: int32(event.TransactionIndex),
		EventIndex:       int32(event.EventIndex),
		Payload:          string(event.Payload),
	}
}

func eventsResponse(events []flow.Event) []generated.Event {
	eventsRes := make([]generated.Event, len(events))
	for i, e := range events {
		eventsRes[i] = eventResponse(e)
	}

	return eventsRes
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

	return &generated.TransactionResult{
		BlockId:         txr.BlockID.String(),
		Status:          &status,
		ErrorMessage:    txr.ErrorMessage,
		ComputationUsed: int32(0),
		Events:          eventsResponse(txr.Events),
		Expandable:      nil,
		Links:           transactionResultLink(txID, link),
	}
}

func transactionResultLink(txID flow.Identifier, link LinkGenerator) *generated.Links {
	self, _ := link.TransactionResultLink(txID)
	return &generated.Links{
		Self: self,
	}
}

func blockLink(id flow.Identifier, link LinkGenerator) *generated.Links {
	self, _ := link.BlockLink(id)
	return &generated.Links{
		Self: self,
	}
}

func blockHeaderResponse(flowHeader *flow.Header) *generated.BlockHeader {
	return &generated.BlockHeader{
		Id:                   flowHeader.ID().String(),
		ParentId:             flowHeader.ParentID.String(),
		Height:               int32(flowHeader.Height),
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
			return []generated.BlockSeal{}, err
		}
		seals[i] = sealResp
	}
	return seals, nil
}

func blockSealResponse(flowSeal *flow.Seal) (generated.BlockSeal, error) {

	var aggregatedApprovalSignatures []generated.AggregatedSignature
	for _, signature := range flowSeal.AggregatedApprovalSigs {

		var verifierSignatures []string
		for _, verifierSignature := range signature.VerifierSignatures {
			verifierSignatures = append(verifierSignatures, toBase64(verifierSignature.Bytes()))
		}
		var signerIDs []string
		for _, signerID := range signature.SignerIDs {
			signerIDs = append(signerIDs, signerID.String())
		}

		aggregatedApprovalSignature := generated.AggregatedSignature{
			VerifierSignatures: verifierSignatures,
			SignerIds:          signerIDs,
		}
		aggregatedApprovalSignatures = append(aggregatedApprovalSignatures, aggregatedApprovalSignature)
	}

	finalState := ""
	if len(flowSeal.FinalState) > 0 {
		finalStateBytes, err := flowSeal.FinalState.MarshalJSON()
		if err != nil {
			return generated.BlockSeal{}, nil
		}
		finalState = string(finalStateBytes)
	}

	return generated.BlockSeal{
		BlockId:                      flowSeal.BlockID.String(),
		ResultId:                     flowSeal.ResultID.String(),
		FinalState:                   finalState,
		AggregatedApprovalSignatures: aggregatedApprovalSignatures,
	}, nil
}

func idsResponse(ids []flow.Identifier) []string {
	res := make([]string, len(ids))
	for x, i := range ids {
		res[x] = i.String()
	}

	return res
}

func collectionResponse(flowCollection *flow.LightCollection) generated.Collection {
	return generated.Collection{
		Id:           flowCollection.ID().String(),
		Transactions: idsResponse(flowCollection.Transactions),
		Links:        nil,
	}
}

func serviceEventListResponse(eventList flow.ServiceEventList) []generated.Event {
	events := make([]generated.Event, len(eventList))
	for i, e := range eventList {
		events[i] = generated.Event{
			Type_:            e.Type,
			TransactionId:    "",
			TransactionIndex: 0,
			EventIndex:       0,
			Payload:          "", //e.Event,
		}
	}
	return events
}

func executionResultResponse(exeResult *flow.ExecutionResult) *generated.ExecutionResult {
	return &generated.ExecutionResult{
		Id:      exeResult.ID().String(),
		BlockId: exeResult.BlockID.String(),
		Events:  serviceEventListResponse(exeResult.ServiceEvents),
		Links:   nil,
	}
}

func accountKeysResponse(keys []flow.AccountPublicKey) []generated.AccountPublicKey {
	keysResponse := make([]generated.AccountPublicKey, len(keys))
	for i, k := range keys {
		sigAlgo := generated.SigningAlgorithm(k.SignAlgo.String())
		hashAlgo := generated.HashingAlgorithm(k.HashAlgo.String())

		keysResponse[i] = generated.AccountPublicKey{
			Index:            int32(k.Index),
			PublicKey:        k.PublicKey.String(),
			SigningAlgorithm: &sigAlgo,
			HashingAlgorithm: &hashAlgo,
			SequenceNumber:   int32(k.SeqNumber),
			Weight:           int32(k.Weight),
			Revoked:          k.Revoked,
		}
	}

	return keysResponse
}

func accountResponse(account *flow.Account) generated.Account {
	contracts := make(map[string]string, len(account.Contracts))
	for name, code := range account.Contracts {
		contracts[name] = string(code)
	}

	return generated.Account{
		Address:    account.Address.String(),
		Balance:    int32(account.Balance),
		Keys:       accountKeysResponse(account.Keys),
		Contracts:  contracts,
		Expandable: nil,
		Links:      nil,
	}
}
