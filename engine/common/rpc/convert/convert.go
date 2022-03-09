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
var ValidChainIds = map[string]bool{
	flow.Mainnet.String():           true,
	flow.Testnet.String():           true,
	flow.Canary.String():            true,
	flow.Benchnet.String():          true,
	flow.Localnet.String():          true,
	flow.Emulator.String():          true,
	flow.BftTestnet.String():        true,
	flow.MonotonicEmulator.String(): true,
}

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

	parentVoterIds := IdentifiersToMessages(h.ParentVoterIDs)

	return &entities.BlockHeader{
		Id:                 id[:],
		ParentId:           h.ParentID[:],
		Height:             h.Height,
		PayloadHash:        h.PayloadHash[:],
		Timestamp:          t,
		View:               h.View,
		ParentVoterIds:     parentVoterIds,
		ParentVoterSigData: h.ParentVoterSigData,
		ProposerId:         h.ProposerID[:],
		ProposerSigData:    h.ProposerSigData,
		ChainId:            h.ChainID.String(),
	}, nil
}

func MessageToBlockHeader(m *entities.BlockHeader) (*flow.Header, error) {
	parentVoterIds := MessagesToIdentifiers(m.ParentVoterIds)
	chainId, err := MessageToChainId(m.ChainId)
	if err != nil {
		return nil, fmt.Errorf("failed to convert ChainId: %w", err)
	}
	return &flow.Header{
		ParentID:           MessageToIdentifier(m.ParentId),
		Height:             m.Height,
		PayloadHash:        MessageToIdentifier(m.PayloadHash),
		Timestamp:          m.Timestamp.AsTime(),
		View:               m.View,
		ParentVoterIDs:     parentVoterIds,
		ParentVoterSigData: m.ParentVoterSigData,
		ProposerID:         MessageToIdentifier(m.ProposerId),
		ProposerSigData:    m.ProposerSigData,
		ChainID:            *chainId,
	}, nil
}

// MessageToChainId checks chainId from enumeration to prevent a panic on Chain() being called
func MessageToChainId(m string) (*flow.ChainID, error) {
	if !ValidChainIds[m] {
		return nil, fmt.Errorf("invalid chainId %s: ", m)
	}
	chainId := flow.ChainID(m)
	return &chainId, nil
}

func CollectionGuaranteesToMessages(c []*flow.CollectionGuarantee) []*entities.CollectionGuarantee {
	cg := make([]*entities.CollectionGuarantee, len(c))
	for i, g := range c {
		cg[i] = CollectionGuaranteeToMessage(g)
	}
	return cg
}

func MessagesToCollectionGuarantees(m []*entities.CollectionGuarantee) []*flow.CollectionGuarantee {
	cg := make([]*flow.CollectionGuarantee, len(m))
	for i, g := range m {
		cg[i] = MessageToCollectionGuarantee(g)
	}
	return cg
}

func BlockSealsToMessages(b []*flow.Seal) []*entities.BlockSeal {
	seals := make([]*entities.BlockSeal, len(b))
	for i, s := range b {
		seals[i] = BlockSealToMessage(s)
	}
	return seals
}

func MessagesToBlockSeals(m []*entities.BlockSeal) ([]*flow.Seal, error) {
	seals := make([]*flow.Seal, len(m))
	for i, s := range m {
		msg, err := MessageToBlockSeal(s)
		if err != nil {
			return nil, err
		}
		seals[i] = msg
	}
	return seals, nil
}

func ExecutionResultsToMessages(e []*flow.ExecutionResult) ([]*entities.ExecutionResult, error) {
	execResults := make([]*entities.ExecutionResult, len(e))
	for i, execRes := range e {
		parsedExecResult, err := ExecutionResultToMessage(execRes)
		if err != nil {
			return nil, err
		}
		execResults[i] = parsedExecResult
	}
	return execResults, nil
}

func MessagesToExecutionResults(m []*entities.ExecutionResult) ([]*flow.ExecutionResult, error) {
	execResults := make([]*flow.ExecutionResult, len(m))
	for i, e := range m {
		parsedExecResult, err := MessageToExecutionResult(e)
		if err != nil {
			return nil, fmt.Errorf("failed to convert message at index %d to execution result: %w", i, err)
		}
		execResults[i] = parsedExecResult
	}
	return execResults, nil
}

func BlockToMessage(h *flow.Block) (*entities.Block, error) {

	id := h.ID()

	parentID := h.Header.ParentID
	t := timestamppb.New(h.Header.Timestamp)
	cg := CollectionGuaranteesToMessages(h.Payload.Guarantees)

	seals := BlockSealsToMessages(h.Payload.Seals)

	execResults, err := ExecutionResultsToMessages(h.Payload.Results)
	if err != nil {
		return nil, err
	}

	blockHeader, err := BlockHeaderToMessage(h.Header)
	if err != nil {
		return nil, err
	}

	bh := entities.Block{
		Id:                       id[:],
		Height:                   h.Header.Height,
		ParentId:                 parentID[:],
		Timestamp:                t,
		CollectionGuarantees:     cg,
		BlockSeals:               seals,
		Signatures:               [][]byte{h.Header.ParentVoterSigData},
		ExecutionReceiptMetaList: ExecutionResultMetaListToMessages(h.Payload.Receipts),
		ExecutionResultList:      execResults,
		BlockHeader:              blockHeader,
	}

	return &bh, nil
}

func MessageToBlock(m *entities.Block) (*flow.Block, error) {
	payload, err := PayloadFromMessage(m)
	if err != nil {
		return nil, fmt.Errorf("failed to extract payload data from message: %w", err)
	}
	header, err := MessageToBlockHeader(m.BlockHeader)
	if err != nil {
		return nil, fmt.Errorf("failed to convert block header: %w", err)
	}
	return &flow.Block{
		Header:  header,
		Payload: payload,
	}, nil
}

func MessagesToExecutionResultMetaList(m []*entities.ExecutionReceiptMeta) flow.ExecutionReceiptMetaList {
	execMetaList := make([]*flow.ExecutionReceiptMeta, len(m))
	for i, message := range m {
		execMetaList[i] = &flow.ExecutionReceiptMeta{
			ExecutorID:        MessageToIdentifier(message.ExecutorId),
			ResultID:          MessageToIdentifier(message.ResultId),
			Spocks:            MessagesToSignatures(message.Spocks),
			ExecutorSignature: MessageToSignature(message.ExecutorSignature),
		}
	}
	return execMetaList[:]
}

func ExecutionResultMetaListToMessages(e flow.ExecutionReceiptMetaList) []*entities.ExecutionReceiptMeta {
	messageList := make([]*entities.ExecutionReceiptMeta, len(e))
	for i, execMeta := range e {
		messageList[i] = &entities.ExecutionReceiptMeta{
			ExecutorId:        IdentifierToMessage(execMeta.ExecutorID),
			ResultId:          IdentifierToMessage(execMeta.ResultID),
			Spocks:            SignaturesToMessages(execMeta.Spocks),
			ExecutorSignature: MessageToSignature(execMeta.ExecutorSignature),
		}
	}
	return messageList
}

func PayloadFromMessage(m *entities.Block) (*flow.Payload, error) {
	cgs := MessagesToCollectionGuarantees(m.CollectionGuarantees)
	seals, err := MessagesToBlockSeals(m.BlockSeals)
	if err != nil {
		return nil, err
	}
	receipts := MessagesToExecutionResultMetaList(m.ExecutionReceiptMetaList)
	results, err := MessagesToExecutionResults(m.ExecutionResultList)
	if err != nil {
		return nil, err
	}
	return &flow.Payload{
		Guarantees: cgs,
		Seals:      seals,
		Receipts:   receipts,
		Results:    results,
	}, nil
}

func CollectionGuaranteeToMessage(g *flow.CollectionGuarantee) *entities.CollectionGuarantee {
	id := g.ID()

	return &entities.CollectionGuarantee{
		CollectionId:     id[:],
		Signatures:       [][]byte{g.Signature},
		ReferenceBlockId: IdentifierToMessage(g.ReferenceBlockID),
		Signature:        g.Signature,
		SignerIds:        IdentifiersToMessages(g.SignerIDs),
	}
}

func MessageToCollectionGuarantee(m *entities.CollectionGuarantee) *flow.CollectionGuarantee {
	return &flow.CollectionGuarantee{
		CollectionID:     MessageToIdentifier(m.CollectionId),
		ReferenceBlockID: MessageToIdentifier(m.ReferenceBlockId),
		SignerIDs:        MessagesToIdentifiers(m.SignerIds),
		Signature:        MessageToSignature(m.Signature),
	}
}

func MessagesToAggregatedSignatures(m []*entities.AggregatedSignature) []flow.AggregatedSignature {
	parsedSignatures := make([]flow.AggregatedSignature, len(m))
	for i, message := range m {
		parsedSignatures[i] = flow.AggregatedSignature{
			SignerIDs:          MessagesToIdentifiers(message.SignerIds),
			VerifierSignatures: MessagesToSignatures(message.VerifierSignatures),
		}
	}
	return parsedSignatures
}

func AggregatedSignaturesToMessages(a []flow.AggregatedSignature) []*entities.AggregatedSignature {
	parsedMessages := make([]*entities.AggregatedSignature, len(a))
	for i, sig := range a {
		parsedMessages[i] = &entities.AggregatedSignature{
			SignerIds:          IdentifiersToMessages(sig.SignerIDs),
			VerifierSignatures: SignaturesToMessages(sig.VerifierSignatures),
		}
	}
	return parsedMessages
}

func MessagesToSignatures(m [][]byte) []crypto.Signature {
	signatures := make([]crypto.Signature, len(m))
	for i, message := range m {
		signatures[i] = MessageToSignature(message)
	}
	return signatures
}

func MessageToSignature(m []byte) crypto.Signature {
	return m[:]
}

func SignaturesToMessages(s []crypto.Signature) [][]byte {
	messages := make([][]byte, len(s))
	for i, sig := range s {
		messages[i] = SignatureToMessage(sig)
	}
	return messages
}

func SignatureToMessage(s crypto.Signature) []byte {
	return s[:]
}

func BlockSealToMessage(s *flow.Seal) *entities.BlockSeal {
	id := s.BlockID
	result := s.ResultID
	return &entities.BlockSeal{
		BlockId:                    id[:],
		ExecutionReceiptId:         result[:],
		ExecutionReceiptSignatures: [][]byte{}, // filling seals signature with zero
		FinalState:                 StateCommitmentToMessage(s.FinalState),
		AggregatedApprovalSigs:     AggregatedSignaturesToMessages(s.AggregatedApprovalSigs),
		ResultId:                   IdentifierToMessage(s.ResultID),
	}
}

func MessageToBlockSeal(m *entities.BlockSeal) (*flow.Seal, error) {
	finalState, err := MessageToStateCommitment(m.FinalState)
	if err != nil {
		return nil, fmt.Errorf("failed to convert message to block seal: %w", err)
	}
	return &flow.Seal{
		BlockID:                MessageToIdentifier(m.BlockId),
		ResultID:               MessageToIdentifier(m.ResultId),
		FinalState:             finalState,
		AggregatedApprovalSigs: MessagesToAggregatedSignatures(m.AggregatedApprovalSigs),
	}, nil
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

func MessagesToChunkList(m []*entities.Chunk) (flow.ChunkList, error) {
	parsedChunks := make(flow.ChunkList, len(m))
	for i, chunk := range m {
		parsedChunk, err := MessageToChunk(chunk)
		if err != nil {
			return nil, fmt.Errorf("failed to parse message at index %d to chunk: %w", i, err)
		}
		parsedChunks[i] = parsedChunk
	}
	return parsedChunks, nil
}

func MessagesToServiceEventList(m []*entities.ServiceEvent) (flow.ServiceEventList, error) {
	parsedServiceEvents := make(flow.ServiceEventList, len(m))
	for i, serviceEvent := range m {
		parsedServiceEvent, err := MessageToServiceEvent(serviceEvent)
		if err != nil {
			return nil, fmt.Errorf("failed to parse service event at index %d from message: %w", i, err)
		}
		parsedServiceEvents[i] = *parsedServiceEvent
	}
	return parsedServiceEvents, nil
}

func MessageToExecutionResult(m *entities.ExecutionResult) (*flow.ExecutionResult, error) {
	// convert Chunks
	parsedChunks, err := MessagesToChunkList(m.Chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to parse messages to ChunkList: %w", err)
	}
	// convert ServiceEvents
	parsedServiceEvents, err := MessagesToServiceEventList(m.ServiceEvents)
	if err != nil {
		return nil, err
	}
	return &flow.ExecutionResult{
		PreviousResultID: MessageToIdentifier(m.PreviousResultId),
		BlockID:          MessageToIdentifier(m.BlockId),
		Chunks:           parsedChunks,
		ServiceEvents:    parsedServiceEvents,
		ExecutionDataID:  MessageToIdentifier(m.ExecutionDataId),
	}, nil
}

func ExecutionResultToMessage(er *flow.ExecutionResult) (*entities.ExecutionResult, error) {

	chunks := make([]*entities.Chunk, len(er.Chunks))

	for i, chunk := range er.Chunks {
		chunks[i] = ChunkToMessage(chunk)
	}

	serviceEvents := make([]*entities.ServiceEvent, len(er.ServiceEvents))
	var err error
	for i, serviceEvent := range er.ServiceEvents {
		serviceEvents[i], err = ServiceEventToMessage(serviceEvent)
		if err != nil {
			return nil, fmt.Errorf("error while convering service event %d: %w", i, err)
		}
	}

	return &entities.ExecutionResult{
		PreviousResultId: IdentifierToMessage(er.PreviousResultID),
		BlockId:          IdentifierToMessage(er.BlockID),
		Chunks:           chunks,
		ServiceEvents:    serviceEvents,
		ExecutionDataId:  IdentifierToMessage(er.ExecutionDataID),
	}, nil
}

func ServiceEventToMessage(event flow.ServiceEvent) (*entities.ServiceEvent, error) {

	bytes, err := json.Marshal(event.Event)
	if err != nil {
		return nil, fmt.Errorf("cannot marshal service event: %w", err)
	}

	return &entities.ServiceEvent{
		Type:    event.Type,
		Payload: bytes,
	}, nil
}

func MessageToServiceEvent(m *entities.ServiceEvent) (*flow.ServiceEvent, error) {
	var event interface{}
	rawEvent := m.Payload
	// map keys correctly
	switch m.Type {
	case flow.ServiceEventSetup:
		setup := new(flow.EpochSetup)
		err := json.Unmarshal(rawEvent, setup)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal to EpochSetup event: %w", err)
		}
		event = setup
	case flow.ServiceEventCommit:
		commit := new(flow.EpochCommit)
		err := json.Unmarshal(rawEvent, commit)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal to EpochCommit event: %w", err)
		}
		event = commit
	default:
		return nil, fmt.Errorf("invalid event type: %s", m.Type)
	}
	return &flow.ServiceEvent{
		Type:  m.Type,
		Event: event,
	}, nil
}

func ChunkToMessage(chunk *flow.Chunk) *entities.Chunk {
	return &entities.Chunk{
		CollectionIndex:      uint32(chunk.CollectionIndex),
		StartState:           StateCommitmentToMessage(chunk.StartState),
		EventCollection:      IdentifierToMessage(chunk.EventCollection),
		BlockId:              IdentifierToMessage(chunk.BlockID),
		TotalComputationUsed: chunk.TotalComputationUsed,
		NumberOfTransactions: uint32(chunk.NumberOfTransactions),
		Index:                chunk.Index,
		EndState:             StateCommitmentToMessage(chunk.EndState),
	}
}

func MessageToChunk(m *entities.Chunk) (*flow.Chunk, error) {
	startState, err := flow.ToStateCommitment(m.StartState)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Message start state to Chunk: %w", err)
	}
	endState, err := flow.ToStateCommitment(m.EndState)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Message end state to Chunk: %w", err)
	}
	chunkBody := flow.ChunkBody{
		CollectionIndex:      uint(m.CollectionIndex),
		StartState:           startState,
		EventCollection:      MessageToIdentifier(m.EventCollection),
		BlockID:              MessageToIdentifier(m.BlockId),
		TotalComputationUsed: m.TotalComputationUsed,
		NumberOfTransactions: uint64(m.NumberOfTransactions),
	}
	return &flow.Chunk{
		ChunkBody: chunkBody,
		Index:     m.Index,
		EndState:  endState,
	}, nil
}
