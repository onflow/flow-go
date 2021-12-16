package convert

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/onflow/flow-go/crypto"
	"github.com/onflow/flow-go/crypto/hash"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/state/protocol"
	"github.com/onflow/flow-go/state/protocol/inmem"
)

var ErrEmptyMessage = errors.New("protobuf message is empty")

func MessageToTransaction(m *entities.Transaction, chain flow.Chain) (flow.TransactionBody, error) {
	if m == nil {
		return flow.TransactionBody{}, ErrEmptyMessage
	}

	t := flow.NewTransactionBody()

	proposalKey := m.GetProposalKey()
	if proposalKey != nil {
		proposalAddress, err := Address(proposalKey.GetAddress(), chain)
		if err != nil {
			return *t, err
		}
		t.SetProposalKey(proposalAddress, uint64(proposalKey.GetKeyId()), proposalKey.GetSequenceNumber())
	}

	payer := m.GetPayer()
	if payer != nil {
		payerAddress, err := Address(payer, chain)
		if err != nil {
			return *t, err
		}
		t.SetPayer(payerAddress)
	}

	for _, authorizer := range m.GetAuthorizers() {
		authorizerAddress, err := Address(authorizer, chain)
		if err != nil {
			return *t, err
		}
		t.AddAuthorizer(authorizerAddress)
	}

	for _, sig := range m.GetPayloadSignatures() {
		addr, err := Address(sig.GetAddress(), chain)
		if err != nil {
			return *t, err
		}
		t.AddPayloadSignature(addr, uint64(sig.GetKeyId()), sig.GetSignature())
	}

	for _, sig := range m.GetEnvelopeSignatures() {
		addr, err := Address(sig.GetAddress(), chain)
		if err != nil {
			return *t, err
		}
		t.AddEnvelopeSignature(addr, uint64(sig.GetKeyId()), sig.GetSignature())
	}

	t.SetScript(m.GetScript())
	t.SetArguments(m.GetArguments())
	t.SetReferenceBlockID(flow.HashToID(m.GetReferenceBlockId()))
	t.SetGasLimit(m.GetGasLimit())

	return *t, nil
}

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
			Address:   sig.Address.Bytes(),
			KeyId:     uint32(sig.KeyIndex),
			Signature: sig.Signature,
		}
	}

	envelopeSigMessages := make([]*entities.Transaction_Signature, len(tb.EnvelopeSignatures))

	for i, sig := range tb.EnvelopeSignatures {
		envelopeSigMessages[i] = &entities.Transaction_Signature{
			Address:   sig.Address.Bytes(),
			KeyId:     uint32(sig.KeyIndex),
			Signature: sig.Signature,
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

func BlockHeaderToMessage(h *flow.Header) (*entities.BlockHeader, error) {
	id := h.ID()

	t := timestamppb.New(h.Timestamp)

	return &entities.BlockHeader{
		Id:        id[:],
		ParentId:  h.ParentID[:],
		Height:    h.Height,
		Timestamp: t,
	}, nil
}

func BlockToMessage(h *flow.Block) (*entities.Block, error) {

	id := h.ID()

	parentID := h.Header.ParentID
	t := timestamppb.New(h.Header.Timestamp)
	cg := make([]*entities.CollectionGuarantee, len(h.Payload.Guarantees))
	for i, g := range h.Payload.Guarantees {
		cg[i] = collectionGuaranteeToMessage(g)
	}

	seals := make([]*entities.BlockSeal, len(h.Payload.Seals))
	for i, s := range h.Payload.Seals {
		seals[i] = blockSealToMessage(s)
	}

	bh := entities.Block{
		Id:                   id[:],
		Height:               h.Header.Height,
		ParentId:             parentID[:],
		Timestamp:            t,
		CollectionGuarantees: cg,
		BlockSeals:           seals,
		Signatures:           [][]byte{h.Header.ParentVoterSigData},
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

func LightCollectionToMessage(c *flow.LightCollection) (*entities.Collection, error) {
	if c == nil || c.Transactions == nil {
		return nil, fmt.Errorf("invalid collection")
	}

	collectionID := c.ID()

	return &entities.Collection{
		Id:             collectionID[:],
		TransactionIds: IdentifiersToMessages(c.Transactions),
	}, nil
}

func EventToMessage(e flow.Event) *entities.Event {
	return &entities.Event{
		Type:             string(e.Type),
		TransactionId:    e.TransactionID[:],
		TransactionIndex: e.TransactionIndex,
		EventIndex:       e.EventIndex,
		Payload:          e.Payload,
	}
}

func MessageToAccount(m *entities.Account) (*flow.Account, error) {
	if m == nil {
		return nil, ErrEmptyMessage
	}

	accountKeys := make([]flow.AccountPublicKey, len(m.GetKeys()))
	for i, key := range m.GetKeys() {
		accountKey, err := MessageToAccountKey(key)
		if err != nil {
			return nil, err
		}

		accountKeys[i] = *accountKey
	}

	return &flow.Account{
		Address:   flow.BytesToAddress(m.GetAddress()),
		Balance:   m.GetBalance(),
		Keys:      accountKeys,
		Contracts: m.Contracts,
	}, nil
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
		Address:   a.Address.Bytes(),
		Balance:   a.Balance,
		Code:      nil,
		Keys:      keys,
		Contracts: a.Contracts,
	}, nil
}

func MessageToAccountKey(m *entities.AccountKey) (*flow.AccountPublicKey, error) {
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
		Index:     int(m.GetIndex()),
		PublicKey: publicKey,
		SignAlgo:  sigAlgo,
		HashAlgo:  hashAlgo,
		Weight:    int(m.GetWeight()),
		SeqNumber: uint64(m.GetSequenceNumber()),
		Revoked:   m.GetRevoked(),
	}, nil
}

func AccountKeyToMessage(a flow.AccountPublicKey) (*entities.AccountKey, error) {
	publicKey := a.PublicKey.Encode()
	return &entities.AccountKey{
		Index:          uint32(a.Index),
		PublicKey:      publicKey,
		SignAlgo:       uint32(a.SignAlgo),
		HashAlgo:       uint32(a.HashAlgo),
		Weight:         uint32(a.Weight),
		SequenceNumber: uint32(a.SeqNumber),
		Revoked:        a.Revoked,
	}, nil
}

func MessagesToEvents(l []*entities.Event) []flow.Event {
	events := make([]flow.Event, len(l))

	for i, m := range l {
		events[i] = MessageToEvent(m)
	}

	return events
}

func MessageToEvent(m *entities.Event) flow.Event {
	return flow.Event{
		Type:             flow.EventType(m.GetType()),
		TransactionID:    flow.HashToID(m.GetTransactionId()),
		TransactionIndex: m.GetTransactionIndex(),
		EventIndex:       m.GetEventIndex(),
		Payload:          m.GetPayload(),
	}
}

func EventsToMessages(flowEvents []flow.Event) []*entities.Event {
	events := make([]*entities.Event, len(flowEvents))
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

func StateCommitmentToMessage(s flow.StateCommitment) []byte {
	return s[:]
}

func MessageToStateCommitment(bytes []byte) (sc flow.StateCommitment, err error) {
	if len(bytes) != len(sc) {
		return sc, fmt.Errorf("invalid state commitment length. got %d expected %d", len(bytes), len(sc))
	}
	copy(sc[:], bytes)
	return
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

// SnapshotToBytes converts a `protocol.Snapshot` to bytes, encoded as JSON
func SnapshotToBytes(snapshot protocol.Snapshot) ([]byte, error) {
	serializable, err := inmem.FromSnapshot(snapshot)
	if err != nil {
		return nil, err
	}

	data, err := json.Marshal(serializable.Encodable())
	if err != nil {
		return nil, err
	}

	return data, nil
}

// BytesToInmemSnapshot converts an array of bytes to `inmem.Snapshot`
func BytesToInmemSnapshot(bytes []byte) (*inmem.Snapshot, error) {
	var encodable inmem.EncodableSnapshot
	err := json.Unmarshal(bytes, &encodable)
	if err != nil {
		return nil, fmt.Errorf("could not unmarshal decoded snapshot: %w", err)
	}

	return inmem.SnapshotFromEncodable(encodable), nil
}
