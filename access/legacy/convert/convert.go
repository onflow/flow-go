package convert

import (
	"errors"
	"fmt"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	accessproto "github.com/onflow/flow/protobuf/go/flow/legacy/access"
	entitiesproto "github.com/onflow/flow/protobuf/go/flow/legacy/entities"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

var ErrEmptyMessage = errors.New("protobuf message is empty")

func MessageToTransaction(m *entitiesproto.Transaction, chain flow.Chain) (flow.TransactionBody, error) {
	var t flow.TransactionBody
	if m == nil {
		return t, ErrEmptyMessage
	}

	tb := flow.NewTransactionBodyBuilder()
	proposalKey := m.GetProposalKey()
	if proposalKey != nil {
		proposalAddress, err := convert.Address(proposalKey.GetAddress(), chain)
		if err != nil {
			return t, err
		}
		tb.SetProposalKey(proposalAddress, proposalKey.GetKeyId(), proposalKey.GetSequenceNumber())
	}

	payer := m.GetPayer()
	if payer != nil {
		payerAddress, err := convert.Address(payer, chain)
		if err != nil {
			return t, err
		}
		tb.SetPayer(payerAddress)
	}

	for _, authorizer := range m.GetAuthorizers() {
		authorizerAddress, err := convert.Address(authorizer, chain)
		if err != nil {
			return t, err
		}
		tb.AddAuthorizer(authorizerAddress)
	}

	for _, sig := range m.GetPayloadSignatures() {
		addr, err := convert.Address(sig.GetAddress(), chain)
		if err != nil {
			return t, err
		}
		tb.AddPayloadSignature(addr, sig.GetKeyId(), sig.GetSignature())
	}

	for _, sig := range m.GetEnvelopeSignatures() {
		addr, err := convert.Address(sig.GetAddress(), chain)
		if err != nil {
			return t, err
		}
		tb.AddEnvelopeSignature(addr, sig.GetKeyId(), sig.GetSignature())
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

func TransactionToMessage(tb flow.TransactionBody) *entitiesproto.Transaction {
	proposalKeyMessage := &entitiesproto.Transaction_ProposalKey{
		Address:        tb.ProposalKey.Address.Bytes(),
		KeyId:          uint32(tb.ProposalKey.KeyIndex),
		SequenceNumber: tb.ProposalKey.SequenceNumber,
	}

	authMessages := make([][]byte, len(tb.Authorizers))
	for i, auth := range tb.Authorizers {
		authMessages[i] = auth.Bytes()
	}

	payloadSigMessages := make([]*entitiesproto.Transaction_Signature, len(tb.PayloadSignatures))

	for i, sig := range tb.PayloadSignatures {
		payloadSigMessages[i] = &entitiesproto.Transaction_Signature{
			Address:   sig.Address.Bytes(),
			KeyId:     uint32(sig.KeyIndex),
			Signature: sig.Signature,
		}
	}

	envelopeSigMessages := make([]*entitiesproto.Transaction_Signature, len(tb.EnvelopeSignatures))

	for i, sig := range tb.EnvelopeSignatures {
		envelopeSigMessages[i] = &entitiesproto.Transaction_Signature{
			Address:   sig.Address.Bytes(),
			KeyId:     uint32(sig.KeyIndex),
			Signature: sig.Signature,
		}
	}

	return &entitiesproto.Transaction{
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

func TransactionResultToMessage(result accessmodel.TransactionResult) *accessproto.TransactionResultResponse {
	return &accessproto.TransactionResultResponse{
		Status:       entitiesproto.TransactionStatus(result.Status),
		StatusCode:   uint32(result.StatusCode),
		ErrorMessage: result.ErrorMessage,
		Events:       EventsToMessages(result.Events),
	}
}

func BlockHeaderToMessage(h *flow.Header) (*entitiesproto.BlockHeader, error) {
	id := h.ID()

	return &entitiesproto.BlockHeader{
		Id:        id[:],
		ParentId:  h.ParentID[:],
		Height:    h.Height,
		Timestamp: convert.BlockTimestamp2ProtobufTime(h.Timestamp),
	}, nil
}

func BlockToMessage(h *flow.Block) (*entitiesproto.Block, error) {
	id := h.ID()

	cg := make([]*entitiesproto.CollectionGuarantee, len(h.Payload.Guarantees))
	for i, g := range h.Payload.Guarantees {
		cg[i] = collectionGuaranteeToMessage(g)
	}

	seals := make([]*entitiesproto.BlockSeal, len(h.Payload.Seals))
	for i, s := range h.Payload.Seals {
		seals[i] = blockSealToMessage(s)
	}

	bh := entitiesproto.Block{
		Id:                   id[:],
		Height:               h.Height,
		ParentId:             h.ParentID[:],
		Timestamp:            convert.BlockTimestamp2ProtobufTime(h.Timestamp),
		CollectionGuarantees: cg,
		BlockSeals:           seals,
		Signatures:           [][]byte{h.ParentVoterSigData},
	}

	return &bh, nil
}

func collectionGuaranteeToMessage(g *flow.CollectionGuarantee) *entitiesproto.CollectionGuarantee {
	return &entitiesproto.CollectionGuarantee{
		CollectionId: IdentifierToMessage(g.CollectionID),
		Signatures:   [][]byte{g.Signature},
	}
}

func blockSealToMessage(s *flow.Seal) *entitiesproto.BlockSeal {
	id := s.BlockID
	result := s.ResultID
	return &entitiesproto.BlockSeal{
		BlockId:                    id[:],
		ExecutionReceiptId:         result[:],
		ExecutionReceiptSignatures: [][]byte{}, // filling seals signature with zero
	}
}

func LightCollectionToMessage(c *flow.LightCollection) (*entitiesproto.Collection, error) {
	if c == nil || c.Transactions == nil {
		return nil, fmt.Errorf("invalid collection")
	}

	collectionID := c.ID()

	return &entitiesproto.Collection{
		Id:             collectionID[:],
		TransactionIds: IdentifiersToMessages(c.Transactions),
	}, nil
}

func EventToMessage(e flow.Event) *entitiesproto.Event {
	return &entitiesproto.Event{
		Type:             string(e.Type),
		TransactionId:    e.TransactionID[:],
		TransactionIndex: e.TransactionIndex,
		EventIndex:       e.EventIndex,
		Payload:          e.Payload,
	}
}

func AccountToMessage(a *flow.Account) (*entitiesproto.Account, error) {
	keys := make([]*entitiesproto.AccountKey, len(a.Keys))
	for i, k := range a.Keys {
		messageKey, err := AccountKeyToMessage(k)
		if err != nil {
			return nil, err
		}
		keys[i] = messageKey
	}

	return &entitiesproto.Account{
		Address: a.Address.Bytes(),
		Balance: a.Balance,
		Code:    nil,
		Keys:    keys,
	}, nil
}

func MessageToAccountKey(m *entitiesproto.AccountKey) (*flow.AccountPublicKey, error) {
	if m == nil {
		return nil, ErrEmptyMessage
	}

	sigAlgo := crypto.SigningAlgorithm(m.GetSignAlgo())
	hashAlgo := hash.HashingAlgorithm(m.GetHashAlgo())

	publicKey, err := crypto.DecodePublicKey(sigAlgo, m.GetPublicKey())
	if err != nil {
		return nil, err
	}

	return &flow.AccountPublicKey{
		PublicKey: publicKey,
		SignAlgo:  sigAlgo,
		HashAlgo:  hashAlgo,
		Weight:    int(m.GetWeight()),
		SeqNumber: uint64(m.GetSequenceNumber()),
	}, nil
}

func AccountKeyToMessage(a flow.AccountPublicKey) (*entitiesproto.AccountKey, error) {
	return &entitiesproto.AccountKey{
		Index:          uint32(a.Index),
		PublicKey:      a.PublicKey.Encode(),
		SignAlgo:       uint32(a.SignAlgo),
		HashAlgo:       uint32(a.HashAlgo),
		Weight:         uint32(a.Weight),
		SequenceNumber: uint32(a.SeqNumber),
	}, nil
}

func EventsToMessages(flowEvents []flow.Event) []*entitiesproto.Event {
	events := make([]*entitiesproto.Event, len(flowEvents))
	for i, e := range flowEvents {
		event := EventToMessage(e)
		events[i] = event
	}
	return events
}

func IdentifierToMessage(i flow.Identifier) []byte {
	return i[:]
}

func MessageToIdentifier(b []byte) flow.Identifier {
	return flow.HashToID(b)
}

func IdentifiersToMessages(l []flow.Identifier) [][]byte {
	results := make([][]byte, len(l))
	for i, item := range l {
		results[i] = IdentifierToMessage(item)
	}
	return results
}

func MessagesToIdentifiers(l [][]byte) []flow.Identifier {
	results := make([]flow.Identifier, len(l))
	for i, item := range l {
		results[i] = MessageToIdentifier(item)
	}
	return results
}
