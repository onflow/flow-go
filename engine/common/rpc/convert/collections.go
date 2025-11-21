package convert

import (
	"fmt"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/model/flow"
)

// CollectionToMessage converts a collection to a protobuf message
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

// LightCollectionToMessage converts a light collection to a protobuf message
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

// MessageToLightCollection converts a protobuf message to a light collection
func MessageToLightCollection(m *entities.Collection) (*flow.LightCollection, error) {
	transactions := make([]flow.Identifier, 0, len(m.TransactionIds))
	for _, txId := range m.TransactionIds {
		transactions = append(transactions, MessageToIdentifier(txId))
	}

	return flow.NewLightCollection(flow.UntrustedLightCollection{
		Transactions: transactions,
	}), nil
}

// FullCollectionToMessage converts a full collection to a slice of protobuf transaction messages
//
// All errors indicate the input could not be correctly converted.
func FullCollectionToMessage(c *flow.Collection) ([]*entities.Transaction, error) {
	if c == nil {
		return nil, fmt.Errorf("invalid collection")
	}

	transactions := make([]*entities.Transaction, len(c.Transactions))
	for i, tx := range c.Transactions {
		transactions[i] = TransactionToMessage(*tx)
	}

	return transactions, nil
}

// MessageToFullCollection converts a slice of protobuf transaction messages to a full collection
func MessageToFullCollection(m []*entities.Transaction, chain flow.Chain) (*flow.Collection, error) {
	transactions := make([]*flow.TransactionBody, len(m))
	for i, tx := range m {
		t, err := MessageToTransaction(tx, chain)
		if err != nil {
			return nil, err
		}
		transactions[i] = &t
	}

	return flow.NewCollection(flow.UntrustedCollection{Transactions: transactions})
}

// CollectionGuaranteeToMessage converts a collection guarantee to a protobuf message
func CollectionGuaranteeToMessage(g *flow.CollectionGuarantee) *entities.CollectionGuarantee {
	return &entities.CollectionGuarantee{
		CollectionId:     IdentifierToMessage(g.CollectionID),
		Signatures:       [][]byte{g.Signature},
		ReferenceBlockId: IdentifierToMessage(g.ReferenceBlockID),
		Signature:        g.Signature,
		SignerIndices:    g.SignerIndices,
		ClusterChainId:   []byte(g.ClusterChainID),
	}
}

// MessageToCollectionGuarantee converts a protobuf message to a collection guarantee
func MessageToCollectionGuarantee(m *entities.CollectionGuarantee) (*flow.CollectionGuarantee, error) {
	return flow.NewCollectionGuarantee(flow.UntrustedCollectionGuarantee{
		CollectionID:     MessageToIdentifier(m.CollectionId),
		ReferenceBlockID: MessageToIdentifier(m.ReferenceBlockId),
		ClusterChainID:   flow.ChainID(m.ClusterChainId),
		SignerIndices:    m.SignerIndices,
		Signature:        MessageToSignature(m.Signature),
	})
}

// CollectionGuaranteesToMessages converts a slice of collection guarantees to a slice of protobuf messages
func CollectionGuaranteesToMessages(c []*flow.CollectionGuarantee) []*entities.CollectionGuarantee {
	cg := make([]*entities.CollectionGuarantee, len(c))
	for i, g := range c {
		cg[i] = CollectionGuaranteeToMessage(g)
	}
	return cg
}

// MessagesToCollectionGuarantees converts a slice of protobuf messages to a slice of collection guarantees
func MessagesToCollectionGuarantees(m []*entities.CollectionGuarantee) ([]*flow.CollectionGuarantee, error) {
	cg := make([]*flow.CollectionGuarantee, len(m))
	for i, g := range m {
		guarantee, err := MessageToCollectionGuarantee(g)
		if err != nil {
			return nil, fmt.Errorf("could not convert message to collection guarantee: %w", err)
		}
		cg[i] = guarantee
	}
	return cg, nil
}
