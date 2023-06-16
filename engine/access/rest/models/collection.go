package models

import (
	"encoding/hex"
	"fmt"

	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"

	"github.com/onflow/flow/protobuf/go/flow/entities"
)

const ExpandsTransactions = "transactions"

func (c *Collection) Build(
	collection *flow.LightCollection,
	txs []*flow.TransactionBody,
	link LinkGenerator,
	expand map[string]bool) error {

	self, err := SelfLink(collection.ID(), link.CollectionLink)
	if err != nil {
		return err
	}

	var expandable CollectionExpandable
	var transactions Transactions
	if expand[ExpandsTransactions] {
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

func (c *Collection) BuildFromGrpc(
	collection *entities.Collection,
	txs []*entities.Transaction,
	link LinkGenerator,
	expand map[string]bool,
	chain flow.Chain) error {

	self, err := SelfLink(convert.MessageToIdentifier(collection.Id), link.CollectionLink)
	if err != nil {
		return err
	}

	transactionsBody := make([]*flow.TransactionBody, 0)
	for _, tx := range txs {
		flowTransaction, err := convert.MessageToTransaction(tx, chain)
		if err != nil {
			return err
		}
		transactionsBody = append(transactionsBody, &flowTransaction)
	}

	var expandable CollectionExpandable
	var transactions Transactions
	if expand[ExpandsTransactions] {
		transactions.Build(transactionsBody, link)
	} else {
		expandable.Transactions = make([]string, len(collection.TransactionIds))
		for i, id := range collection.TransactionIds {
			expandable.Transactions[i], err = link.TransactionLink(convert.MessageToIdentifier(id))
			if err != nil {
				return err
			}
		}
	}

	c.Id = hex.EncodeToString(collection.Id)
	c.Transactions = transactions
	c.Links = self
	c.Expandable = &expandable

	return nil
}

func (c *CollectionGuarantee) Build(guarantee *flow.CollectionGuarantee) {
	c.CollectionId = guarantee.CollectionID.String()
	c.SignerIndices = fmt.Sprintf("%x", guarantee.SignerIndices)
	c.Signature = util.ToBase64(guarantee.Signature.Bytes())
}

type CollectionGuarantees []CollectionGuarantee

func (c *CollectionGuarantees) Build(guarantees []*flow.CollectionGuarantee) {
	collGuarantees := make([]CollectionGuarantee, len(guarantees))
	for i, g := range guarantees {
		var col CollectionGuarantee
		col.Build(g)
		collGuarantees[i] = col
	}
	*c = collGuarantees
}
