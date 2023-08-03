package convert

import (
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/module/executiondatasync/execution_data"
)

// BlockExecutionDataToMessage converts a BlockExecutionData to a protobuf message
func BlockExecutionDataToMessage(data *execution_data.BlockExecutionData) (
	*entities.BlockExecutionData,
	error,
) {
	chunkExecutionDatas := make([]*entities.ChunkExecutionData, len(data.ChunkExecutionDatas))
	for i, chunk := range data.ChunkExecutionDatas {
		chunkMessage, err := ChunkExecutionDataToMessage(chunk)
		if err != nil {
			return nil, err
		}
		chunkExecutionDatas[i] = chunkMessage
	}
	return &entities.BlockExecutionData{
		BlockId:            IdentifierToMessage(data.BlockID),
		ChunkExecutionData: chunkExecutionDatas,
	}, nil
}

// MessageToBlockExecutionData converts a protobuf message to a BlockExecutionData
func MessageToBlockExecutionData(
	m *entities.BlockExecutionData,
	chain flow.Chain,
) (*execution_data.BlockExecutionData, error) {
	if m == nil {
		return nil, ErrEmptyMessage
	}
	chunks := make([]*execution_data.ChunkExecutionData, len(m.ChunkExecutionData))
	for i, chunk := range m.GetChunkExecutionData() {
		convertedChunk, err := MessageToChunkExecutionData(chunk, chain)
		if err != nil {
			return nil, err
		}
		chunks[i] = convertedChunk
	}

	return &execution_data.BlockExecutionData{
		BlockID:             MessageToIdentifier(m.GetBlockId()),
		ChunkExecutionDatas: chunks,
	}, nil
}

// ChunkExecutionDataToMessage converts a ChunkExecutionData to a protobuf message
func ChunkExecutionDataToMessage(data *execution_data.ChunkExecutionData) (
	*entities.ChunkExecutionData,
	error,
) {
	collection := &entities.ExecutionDataCollection{}
	if data.Collection != nil {
		collection = &entities.ExecutionDataCollection{
			Transactions: TransactionsToMessages(data.Collection.Transactions),
		}
	}

	events := EventsToMessages(data.Events)
	if len(events) == 0 {
		events = nil
	}

	trieUpdate, err := TrieUpdateToMessage(data.TrieUpdate)
	if err != nil {
		return nil, err
	}

	return &entities.ChunkExecutionData{
		Collection: collection,
		Events:     events,
		TrieUpdate: trieUpdate,
	}, nil
}

// MessageToChunkExecutionData converts a protobuf message to a ChunkExecutionData
func MessageToChunkExecutionData(
	m *entities.ChunkExecutionData,
	chain flow.Chain,
) (*execution_data.ChunkExecutionData, error) {
	collection, err := messageToTrustedCollection(m.GetCollection(), chain)
	if err != nil {
		return nil, err
	}

	var trieUpdate *ledger.TrieUpdate
	if m.GetTrieUpdate() != nil {
		trieUpdate, err = MessageToTrieUpdate(m.GetTrieUpdate())
		if err != nil {
			return nil, err
		}
	}

	events := MessagesToEvents(m.GetEvents())
	if len(events) == 0 {
		events = nil
	}

	return &execution_data.ChunkExecutionData{
		Collection: collection,
		Events:     events,
		TrieUpdate: trieUpdate,
	}, nil
}

// MessageToTrieUpdate converts a protobuf message to a TrieUpdate
func MessageToTrieUpdate(m *entities.TrieUpdate) (*ledger.TrieUpdate, error) {
	rootHash, err := ledger.ToRootHash(m.GetRootHash())
	if err != nil {
		return nil, fmt.Errorf("could not convert root hash: %w", err)
	}

	paths := make([]ledger.Path, len(m.GetPaths()))
	for i, path := range m.GetPaths() {
		convertedPath, err := ledger.ToPath(path)
		if err != nil {
			return nil, fmt.Errorf("could not convert path %d: %w", i, err)
		}
		paths[i] = convertedPath
	}

	payloads := make([]*ledger.Payload, len(m.Payloads))
	for i, payload := range m.GetPayloads() {
		keyParts := make([]ledger.KeyPart, len(payload.GetKeyPart()))
		for j, keypart := range payload.GetKeyPart() {
			keyParts[j] = ledger.NewKeyPart(uint16(keypart.GetType()), keypart.GetValue())
		}
		payloads[i] = ledger.NewPayload(ledger.NewKey(keyParts), payload.GetValue())
	}

	return &ledger.TrieUpdate{
		RootHash: rootHash,
		Paths:    paths,
		Payloads: payloads,
	}, nil
}

// TrieUpdateToMessage converts a TrieUpdate to a protobuf message
func TrieUpdateToMessage(t *ledger.TrieUpdate) (*entities.TrieUpdate, error) {
	if t == nil {
		return nil, nil
	}

	paths := make([][]byte, len(t.Paths))
	for i := range t.Paths {
		paths[i] = t.Paths[i][:]
	}

	payloads := make([]*entities.Payload, len(t.Payloads))
	for i, payload := range t.Payloads {
		key, err := payload.Key()
		if err != nil {
			return nil, fmt.Errorf("could not convert payload %d: %w", i, err)
		}
		keyParts := make([]*entities.KeyPart, len(key.KeyParts))
		for j, keyPart := range key.KeyParts {
			keyParts[j] = &entities.KeyPart{
				Type:  uint32(keyPart.Type),
				Value: keyPart.Value,
			}
		}
		payloads[i] = &entities.Payload{
			KeyPart: keyParts,
			Value:   payload.Value(),
		}
	}

	return &entities.TrieUpdate{
		RootHash: t.RootHash[:],
		Paths:    paths,
		Payloads: payloads,
	}, nil
}

// messageToTrustedCollection converts a protobuf message to a collection using the
// messageToTrustedTransaction converter to support service transactions.
func messageToTrustedCollection(
	m *entities.ExecutionDataCollection,
	chain flow.Chain,
) (*flow.Collection, error) {
	messages := m.GetTransactions()
	transactions := make([]*flow.TransactionBody, len(messages))
	for i, message := range messages {
		transaction, err := messageToTrustedTransaction(message, chain)
		if err != nil {
			return nil, fmt.Errorf("could not convert transaction %d: %w", i, err)
		}
		transactions[i] = &transaction
	}

	if len(transactions) == 0 {
		return nil, nil
	}

	return &flow.Collection{Transactions: transactions}, nil
}

// messageToTrustedTransaction converts a transaction message to a transaction body.
// This is useful when converting transactions from trusted state like BlockExecutionData which
// contain service transactions that do not conform to external transaction format.
func messageToTrustedTransaction(
	m *entities.Transaction,
	chain flow.Chain,
) (flow.TransactionBody, error) {
	if m == nil {
		return flow.TransactionBody{}, ErrEmptyMessage
	}

	t := flow.NewTransactionBody()

	proposalKey := m.GetProposalKey()
	if proposalKey != nil {
		proposalAddress, err := insecureAddress(proposalKey.GetAddress(), chain)
		if err != nil {
			return *t, fmt.Errorf("could not convert proposer address: %w", err)
		}
		t.SetProposalKey(proposalAddress, uint64(proposalKey.GetKeyId()), proposalKey.GetSequenceNumber())
	}

	payer := m.GetPayer()
	if payer != nil {
		payerAddress, err := insecureAddress(payer, chain)
		if err != nil {
			return *t, fmt.Errorf("could not convert payer address: %w", err)
		}
		t.SetPayer(payerAddress)
	}

	for _, authorizer := range m.GetAuthorizers() {
		authorizerAddress, err := Address(authorizer, chain)
		if err != nil {
			return *t, fmt.Errorf("could not convert authorizer address: %w", err)
		}
		t.AddAuthorizer(authorizerAddress)
	}

	for _, sig := range m.GetPayloadSignatures() {
		addr, err := Address(sig.GetAddress(), chain)
		if err != nil {
			return *t, fmt.Errorf("could not convert payload signature address: %w", err)
		}
		t.AddPayloadSignature(addr, uint64(sig.GetKeyId()), sig.GetSignature())
	}

	for _, sig := range m.GetEnvelopeSignatures() {
		addr, err := Address(sig.GetAddress(), chain)
		if err != nil {
			return *t, fmt.Errorf("could not convert envelope signature address: %w", err)
		}
		t.AddEnvelopeSignature(addr, uint64(sig.GetKeyId()), sig.GetSignature())
	}

	t.SetScript(m.GetScript())
	t.SetArguments(m.GetArguments())
	t.SetReferenceBlockID(flow.HashToID(m.GetReferenceBlockId()))
	t.SetGasLimit(m.GetGasLimit())

	return *t, nil
}

// insecureAddress converts a raw address to a flow.Address, skipping validation
// This is useful when converting transactions from trusted state like BlockExecutionData.
// This should only be used for trusted inputs
func insecureAddress(rawAddress []byte, chain flow.Chain) (flow.Address, error) {
	if len(rawAddress) == 0 {
		return flow.EmptyAddress, status.Error(codes.InvalidArgument, "address cannot be empty")
	}

	return flow.BytesToAddress(rawAddress), nil
}
