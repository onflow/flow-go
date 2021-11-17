package rest

import (
	"encoding/base64"
	"fmt"
	"regexp"

	"github.com/onflow/flow-go/engine/access/rest/generated"
	"github.com/onflow/flow-go/model/flow"
)

// Converter provides functionality to convert from request models generated using
// open api spec and flow models.

// Flow section - converting request data to flow models with validation.

func toID(id string) (flow.Identifier, error) {
	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{64}$`, id)
	if !valid {
		return flow.Identifier{}, fmt.Errorf("invalid ID")
	}

	return flow.HexStringToIdentifier(id)
}

func toAddress(address string) (flow.Address, error) {
	valid, _ := regexp.MatchString(`^[0-9a-fA-F]{16}$`, address)
	if !valid {
		return flow.Address{}, fmt.Errorf("invalid address")
	}

	return flow.HexToAddress(address), nil
}

func toProposalKey(key *generated.ProposalKey) (flow.ProposalKey, error) {
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

func toSignature(signature *generated.TransactionSignature) (flow.TransactionSignature, error) {
	address, err := toAddress(signature.Address)
	if err != nil {
		return flow.TransactionSignature{}, err
	}

	return flow.TransactionSignature{
		Address:     address,
		SignerIndex: int(signature.SignerIndex),
		KeyIndex:    uint64(signature.KeyIndex),
		Signature:   []byte(signature.Signature),
	}, nil
}

func toSignatures(sigs []generated.TransactionSignature) ([]flow.TransactionSignature, error) {
	signatures := make([]flow.TransactionSignature, len(sigs))
	for _, sig := range sigs {
		signature, err := toSignature(&sig)
		if err != nil {
			return nil, err
		}

		signatures = append(signatures, signature)
	}

	return signatures, nil
}

func toTransaction(tx *generated.TransactionsBody) (flow.TransactionBody, error) {
	args := make([][]byte, len(tx.Arguments))
	for _, arg := range tx.Arguments {
		// todo validate
		args = append(args, []byte(arg))
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
	for _, auth := range tx.Authorizers {
		a, err := toAddress(auth)
		if err != nil {
			return flow.TransactionBody{}, err
		}

		auths = append(auths, a)
	}

	payloadSigs, err := toSignatures(tx.PayloadSignatures)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	envelopeSigs, err := toSignatures(tx.EnvelopeSignatures)
	if err != nil {
		return flow.TransactionBody{}, err
	}

	return flow.TransactionBody{
		ReferenceBlockID:   flow.Identifier{},
		Script:             []byte(tx.Script),
		Arguments:          args,
		GasLimit:           uint64(tx.GasLimit),
		ProposalKey:        proposal,
		Payer:              payer,
		Authorizers:        auths,
		PayloadSignatures:  payloadSigs,
		EnvelopeSignatures: envelopeSigs,
	}, nil
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
	for _, sig := range signatures {
		sigs = append(sigs,
			generated.TransactionSignature{
				Address:     sig.Address.String(),
				SignerIndex: int32(sig.SignerIndex),
				KeyIndex:    int32(sig.KeyIndex),
				Signature:   string(sig.Signature),
			},
		)
	}

	return sigs
}

func transactionResponse(tx *flow.TransactionBody) *generated.Transaction {
	var args []string
	for _, arg := range tx.Arguments {
		args = append(args, string(arg))
	}

	var auths []string
	for _, auth := range tx.Authorizers {
		auths = append(auths, auth.String())
	}

	return &generated.Transaction{
		Id:                 tx.ID().String(),
		Script:             string(tx.Script),
		Arguments:          args,
		ReferenceBlockId:   tx.ReferenceBlockID.String(),
		GasLimit:           int32(tx.GasLimit), // todo(sideninja) make sure this is ok
		Payer:              tx.Payer.String(),
		ProposalKey:        proposalKeyResponse(&tx.ProposalKey),
		Authorizers:        auths,
		PayloadSignatures:  transactionSignatureResponse(tx.PayloadSignatures),
		EnvelopeSignatures: transactionSignatureResponse(tx.EnvelopeSignatures),
		Result:             nil, // todo(sideninja) should we provide result, maybe have a wait for result http long pulling system would be super helpful (with reasonable timeout) but careful about resources and dos
	}
}

func blockResponse(flowBlock *flow.Block) *generated.Block {
	return &generated.Block{
		Header:  blockHeaderResponse(flowBlock.Header),
		Payload: blockPayloadResponse(flowBlock.Payload),
	}
}

func blockHeaderResponse(flowHeader *flow.Header) *generated.BlockHeader {
	return &generated.BlockHeader{
		Id:                   flowHeader.ID().String(),
		ParentId:             flowHeader.ParentID.String(),
		Height:               int32(flowHeader.Height),
		Timestamp:            flowHeader.Timestamp,
		ParentVoterSignature: fmt.Sprint(flowHeader.ParentVoterSigData),
	}
}

func blockPayloadResponse(flowPayload *flow.Payload) *generated.BlockPayload {
	return &generated.BlockPayload{
		CollectionGuarantees: collectionGuaranteesResponse(flowPayload.Guarantees),
		BlockSeals:           blockSealsResponse(flowPayload.Seals),
	}
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
		Signature:    base64.StdEncoding.EncodeToString(flowCollGuarantee.Signature.Bytes()),
	}
}

func blockSealsResponse(flowSeals []*flow.Seal) []generated.BlockSeal {
	seals := make([]generated.BlockSeal, len(flowSeals))
	for i, seal := range flowSeals {
		seals[i] = blockSealResponse(seal)
	}
	return seals
}

func blockSealResponse(flowSeal *flow.Seal) generated.BlockSeal {
	return generated.BlockSeal{
		BlockId:  flowSeal.BlockID.String(),
		ResultId: flowSeal.ResultID.String(),
	}
}
