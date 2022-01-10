package models

import (
	"github.com/onflow/flow-go/engine/access/rest"
	"github.com/onflow/flow-go/engine/access/rest/request"
	"github.com/onflow/flow-go/model/flow"
)

func (c *Collection) Build(
	collection *flow.LightCollection,
	txs []*flow.TransactionBody,
	link rest.LinkGenerator,
	expand map[string]bool) error {

	self, err := rest.SelfLink(collection.ID(), link.CollectionLink)
	if err != nil {
		return err
	}

	var expandable CollectionExpandable
	var transactions Transactions
	if expand[request.ExpandsTransactions] {
		transactions.Build(txs, link)
	} else {
		expandable.Transactions = make([]string, len(collection.Transactions))
		for i, tx := range collection.Transactions {
			expandable.Transactions[i], err = link.TransactionLink(tx)
			if err != nil {
				return err
			}
		}
	}

	c.Id = collection.ID().String()
	c.Transactions = transactions
	c.Links = self
	c.Expandable = &expandable

	return nil
}
