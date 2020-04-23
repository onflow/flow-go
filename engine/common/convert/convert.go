package convert

import (
	"fmt"

	"github.com/golang/protobuf/ptypes"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/dapperlabs/flow-go/model/flow"
)

func MessageToTransaction(m *entities.Transaction) (flow.TransactionBody, error) {
	if m == nil {
		return flow.TransactionBody{}, fmt.Errorf("message is empty")
	}

	t := flow.NewTransactionBody()

	t.SetScript(m.GetScript())
	t.SetReferenceBlockID(flow.HashToID(m.GetReferenceBlockId()))
	t.SetGasLimit(m.GetGasLimit())

	proposalKey := m.GetProposalKey()
	if proposalKey != nil {
		proposalAddress := flow.BytesToAddress(proposalKey.GetAddress())
		t.SetProposalKey(proposalAddress, int(proposalKey.GetKeyId()), proposalKey.GetSequenceNumber())
	}

	payer := m.GetPayer()
	if payer != nil {
		t.SetPayer(
			flow.BytesToAddress(payer),
		)
	}

	for _, authorizer := range m.GetAuthorizers() {
		t.AddAuthorizer(
			flow.BytesToAddress(authorizer),
		)
	}

	for _, sig := range m.GetPayloadSignatures() {
		addr := flow.BytesToAddress(sig.GetAddress())
		t.AddPayloadSignature(addr, int(sig.GetKeyId()), sig.GetSignature())
	}

	for _, sig := range m.GetEnvelopeSignatures() {
		addr := flow.BytesToAddress(sig.GetAddress())
		t.AddEnvelopeSignature(addr, int(sig.GetKeyId()), sig.GetSignature())
	}

	return *t, nil
}

func TransactionToMessage(tb flow.TransactionBody) *entities.Transaction {
	proposalKeyMessage := &entities.Transaction_ProposalKey{
		Address:        tb.ProposalKey.Address.Bytes(),
		KeyId:          uint32(tb.ProposalKey.KeyID),
		SequenceNumber: tb.ProposalKey.SequenceNumber,
	}

	authMessages := make([][]byte, len(tb.Authorizers))
	for i, auth := range tb.Authorizers {
		authMessages[i] = auth.Bytes()
	}

	payloadSigMessages := make([]*entities.Transaction_Signature, len(tb.PayloadSignatures))

	for i, sig := range tb.PayloadSignatures {
		payloadSigMessages[i] = &entities.Transaction_Signature{
			Address:   sig.Address.Bytes(),
			KeyId:     uint32(sig.KeyID),
			Signature: sig.Signature,
		}
	}

	envelopeSigMessages := make([]*entities.Transaction_Signature, len(tb.EnvelopeSignatures))

	for i, sig := range tb.EnvelopeSignatures {
		envelopeSigMessages[i] = &entities.Transaction_Signature{
			Address:   sig.Address.Bytes(),
			KeyId:     uint32(sig.KeyID),
			Signature: sig.Signature,
		}
	}

	return &entities.Transaction{
		Script:             tb.Script,
		ReferenceBlockId:   tb.ReferenceBlockID[:],
		GasLimit:           tb.GasLimit,
		ProposalKey:        proposalKeyMessage,
		Payer:              tb.Payer.Bytes(),
		Authorizers:        authMessages,
		PayloadSignatures:  payloadSigMessages,
		EnvelopeSignatures: envelopeSigMessages,
	}
}

func BlockHeaderToMessage(h *flow.Header) (entities.BlockHeader, error) {
	id := h.ID()
	bh := entities.BlockHeader{
		Id:       id[:],
		ParentId: h.ParentID[:],
		Height:   h.Height,
	}
	return bh, nil
}

func BlockToMessage(h *flow.Block) (*entities.Block, error) {

	id := h.ID()

	parentID := h.ParentID
	t, err := ptypes.TimestampProto(h.Timestamp)
	if err != nil {
		return nil, err
	}

	cg := make([]*entities.CollectionGuarantee, len(h.Guarantees))
	for i, g := range h.Guarantees {
		cg[i] = collectionGuaranteeToMessage(g)
	}

	seals := make([]*entities.BlockSeal, len(h.Seals))
	for i, s := range h.Seals {
		seals[i] = blockSealToMessage(s)
	}

	bh := entities.Block{
		Id:                   id[:],
		Height:               h.Height,
		ParentId:             parentID[:],
		Timestamp:            t,
		CollectionGuarantees: cg,
		BlockSeals:           seals,
		Signatures:           [][]byte{h.ParentVoterSig},
	}
	return &bh, nil
}

func collectionGuaranteeToMessage(g *flow.CollectionGuarantee) *entities.CollectionGuarantee {
	id := g.ID()

	return &entities.CollectionGuarantee{
		CollectionId: id[:],
		Signatures:   [][]byte{g.Signature},
	}
}

func blockSealToMessage(s *flow.Seal) *entities.BlockSeal {
	id := s.BlockID
	result := s.ResultID
	return &entities.BlockSeal{
		BlockId:                    id[:],
		ExecutionReceiptId:         result[:],
		ExecutionReceiptSignatures: [][]byte{}, // filling seals signature with zero
	}
}

func CollectionToMessage(c *flow.Collection) (*entities.Collection, error) {
	if c == nil || c.Transactions == nil {
		return nil, fmt.Errorf("invalid collection")
	}

	transactionsIDs := make([][]byte, len(c.Transactions))
	for i, t := range c.Transactions {
		id := t.ID()
		transactionsIDs[i] = id[:]
	}

	collectionID := c.ID()
	ce := &entities.Collection{
		Id:             collectionID[:],
		TransactionIds: transactionsIDs,
	}
	return ce, nil
}

func EventToMessage(e flow.Event) *entities.Event {
	id := e.TransactionID
	return &entities.Event{
		Type:             string(e.Type),
		TransactionId:    id[:],
		TransactionIndex: e.TransactionIndex,
		EventIndex:       e.EventIndex,
		Payload:          e.Payload,
	}
}

func AccountToMessage(a *flow.Account) (*entities.Account, error) {

	keys := make([]*entities.AccountKey, len(a.Keys))
	for i, k := range a.Keys {
		messageKey, err := AccountKeyToMessage(k)
		if err != nil {
			return nil, err
		}
		keys[i] = messageKey
	}

	return &entities.Account{
		Address: a.Address.Bytes(),
		Balance: a.Balance,
		Code:    a.Code,
		Keys:    keys,
	}, nil
}

func AccountKeyToMessage(a flow.AccountPublicKey) (*entities.AccountKey, error) {
	publicKey := a.PublicKey.Encode()
	return &entities.AccountKey{
		PublicKey: publicKey,
		SignAlgo:  uint32(a.SignAlgo),
		HashAlgo:  uint32(a.HashAlgo),
		Weight:    uint32(a.Weight),
	}, nil
}
