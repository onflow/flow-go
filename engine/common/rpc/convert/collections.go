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

	return &flow.LightCollection{
		Transactions: transactions,
	}, nil
}

// CollectionGuaranteeToMessage converts a collection guarantee to a protobuf message
func CollectionGuaranteeToMessage(g *flow.CollectionGuarantee) *entities.CollectionGuarantee {
	id := g.ID()

	return &entities.CollectionGuarantee{
		CollectionId:     id[:],
		Signatures:       [][]byte{g.Signature},
		ReferenceBlockId: IdentifierToMessage(g.ReferenceBlockID),
		Signature:        g.Signature,
		SignerIndices:    g.SignerIndices,
	}
}

// MessageToCollectionGuarantee converts a protobuf message to a collection guarantee
func MessageToCollectionGuarantee(m *entities.CollectionGuarantee) *flow.CollectionGuarantee {
	return &flow.CollectionGuarantee{
		CollectionID:     MessageToIdentifier(m.CollectionId),
		ReferenceBlockID: MessageToIdentifier(m.ReferenceBlockId),
		SignerIndices:    m.SignerIndices,
		Signature:        MessageToSignature(m.Signature),
	}
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
func MessagesToCollectionGuarantees(m []*entities.CollectionGuarantee) []*flow.CollectionGuarantee {
	cg := make([]*flow.CollectionGuarantee, len(m))
	for i, g := range m {
		cg[i] = MessageToCollectionGuarantee(g)
	}
	return cg
}
