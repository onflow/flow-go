package convert

import (
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

// TransactionToMessage converts a flow.TransactionBody to a protobuf message
func TransactionToMessage(tb flow.TransactionBody) *entities.Transaction {
	proposalKeyMessage := &entities.Transaction_ProposalKey{
		Address:        tb.ProposalKey.Address.Bytes(),
		KeyId:          uint32(tb.ProposalKey.KeyIndex),
		SequenceNumber: tb.ProposalKey.SequenceNumber,
	}

	authMessages := make([][]byte, len(tb.Authorizers))
	for i, auth := range tb.Authorizers {
		authMessages[i] = auth.Bytes()
	}

	payloadSigMessages := make([]*entities.Transaction_Signature, len(tb.PayloadSignatures))

	for i, sig := range tb.PayloadSignatures {
		payloadSigMessages[i] = &entities.Transaction_Signature{
			Address:       sig.Address.Bytes(),
			KeyId:         uint32(sig.KeyIndex),
			Signature:     sig.Signature,
			ExtensionData: sig.ExtensionData,
		}
	}

	envelopeSigMessages := make([]*entities.Transaction_Signature, len(tb.EnvelopeSignatures))

	for i, sig := range tb.EnvelopeSignatures {
		envelopeSigMessages[i] = &entities.Transaction_Signature{
			Address:       sig.Address.Bytes(),
			KeyId:         uint32(sig.KeyIndex),
			Signature:     sig.Signature,
			ExtensionData: sig.ExtensionData,
		}
	}

	return &entities.Transaction{
		Script:             tb.Script,
		Arguments:          tb.Arguments,
		ReferenceBlockId:   tb.ReferenceBlockID[:],
		GasLimit:           tb.GasLimit,
		ProposalKey:        proposalKeyMessage,
		Payer:              tb.Payer.Bytes(),
		Authorizers:        authMessages,
		PayloadSignatures:  payloadSigMessages,
		EnvelopeSignatures: envelopeSigMessages,
	}
}

// MessageToTransaction converts a protobuf message to a flow.TransactionBody
func MessageToTransaction(
	m *entities.Transaction,
	chain flow.Chain,
) (flow.TransactionBody, error) {
	var t flow.TransactionBody
	if m == nil {
		return t, ErrEmptyMessage
	}
	tb := flow.NewTransactionBodyBuilder()

	proposalKey := m.GetProposalKey()
	if proposalKey != nil {
		proposalAddress, err := Address(proposalKey.GetAddress(), chain)
		if err != nil {
			return t, err
		}
		tb.SetProposalKey(proposalAddress, proposalKey.GetKeyId(), proposalKey.GetSequenceNumber())
	}

	payer := m.GetPayer()
	if payer != nil {
		payerAddress, err := Address(payer, chain)
		if err != nil {
			return t, err
		}
		tb.SetPayer(payerAddress)
	}

	for _, authorizer := range m.GetAuthorizers() {
		authorizerAddress, err := Address(authorizer, chain)
		if err != nil {
			return t, err
		}
		tb.AddAuthorizer(authorizerAddress)
	}

	for _, sig := range m.GetPayloadSignatures() {
		addr, err := Address(sig.GetAddress(), chain)
		if err != nil {
			return t, err
		}
		tb.AddPayloadSignatureWithExtensionData(addr, sig.GetKeyId(), sig.GetSignature(), sig.GetExtensionData())
	}

	for _, sig := range m.GetEnvelopeSignatures() {
		addr, err := Address(sig.GetAddress(), chain)
		if err != nil {
			return t, err
		}
		tb.AddEnvelopeSignatureWithExtensionData(addr, sig.GetKeyId(), sig.GetSignature(), sig.GetExtensionData())
	}

	transactionBody, err := tb.SetScript(m.GetScript()).
		SetArguments(m.GetArguments()).
		SetReferenceBlockID(flow.HashToID(m.GetReferenceBlockId())).
		SetComputeLimit(m.GetGasLimit()).
		Build()
	if err != nil {
		return t, fmt.Errorf("could not build transaction body: %w", err)
	}

	return *transactionBody, nil
}

// TransactionsToMessages converts a slice of flow.TransactionBody to a slice of protobuf messages
func TransactionsToMessages(transactions []*flow.TransactionBody) []*entities.Transaction {
	transactionMessages := make([]*entities.Transaction, len(transactions))
	for i, t := range transactions {
		transactionMessages[i] = TransactionToMessage(*t)
	}
	return transactionMessages
}
