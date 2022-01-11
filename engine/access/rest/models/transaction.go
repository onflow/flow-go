package models

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/model/flow"
)

func (t *Transaction) Build(tx *flow.TransactionBody, txr *access.TransactionResult, link LinkGenerator) {
	args := make([]string, len(tx.Arguments))
	for i, arg := range tx.Arguments {
		args[i] = ToBase64(arg)
	}

	auths := make([]string, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		auths[i] = auth.String()
	}

	// if transaction result is provided then add that to the response, else add the result link to the expandable
	var txResult TransactionResult
	var expandable TransactionExpandable
	if txr != nil {
		txResult.Build(txr, tx.ID(), link)
	} else {
		resultLink, _ := link.TransactionResultLink(tx.ID())
		expandable.Result = resultLink
	}

	self, _ := SelfLink(tx.ID(), link.TransactionLink)

	var payloadSigs TransactionSignatures
	payloadSigs.Build(tx.PayloadSignatures)

	var envelopeSigs TransactionSignatures
	envelopeSigs.Build(tx.EnvelopeSignatures)

	var proposalKey ProposalKey
	proposalKey.Build(tx.ProposalKey)

	t.Id = tx.ID().String()
	t.Script = ToBase64(tx.Script)
	t.Arguments = args
	t.ReferenceBlockId = tx.ReferenceBlockID.String()
	t.GasLimit = fromUint64(tx.GasLimit)
	t.Payer = tx.Payer.String()
	t.ProposalKey = &proposalKey
	t.Authorizers = auths
	t.PayloadSignatures = payloadSigs
	t.EnvelopeSignatures = envelopeSigs
	t.Result = &txResult
	t.Links = self
	t.Expandable = &expandable
}

type Transactions []Transaction

func (t *Transactions) Build(transactions []*flow.TransactionBody, link LinkGenerator) {
	txs := make([]Transaction, len(transactions))
	for i, tr := range transactions {
		var tx Transaction
		tx.Build(tr, nil, link)
		txs[i] = tx
	}

	*t = txs
}

type TransactionSignatures []TransactionSignature

func (t *TransactionSignatures) Build(signatures []flow.TransactionSignature) {
	sigs := make([]TransactionSignature, len(signatures))
	for i, s := range signatures {
		var sig TransactionSignature
		sig.Build(s)
		sigs[i] = sig
	}

	*t = sigs
}

func (t *TransactionSignature) Build(sig flow.TransactionSignature) {
	t.Address = sig.Address.String()
	t.SignerIndex = fromUint64(uint64(sig.SignerIndex))
	t.KeyIndex = fromUint64(sig.KeyIndex)
	t.Signature = ToBase64(sig.Signature)
}

func (t *TransactionResult) Build(txr *access.TransactionResult, txID flow.Identifier, link LinkGenerator) {
	var status TransactionStatus
	status.Build(txr.Status)

	var events Events
	events.Build(txr.Events)

	t.BlockId = txr.BlockID.String()
	t.Status = &status
	t.ErrorMessage = txr.ErrorMessage
	t.ComputationUsed = fromUint64(0)
	t.Events = events

	self, _ := SelfLink(txID, link.TransactionResultLink)
	t.Links = self
}

func (t *TransactionStatus) Build(status flow.TransactionStatus) {
	switch status {
	case flow.TransactionStatusExpired:
		*t = EXPIRED
	case flow.TransactionStatusExecuted:
		*t = EXECUTED
	case flow.TransactionStatusFinalized:
		*t = FINALIZED
	case flow.TransactionStatusSealed:
		*t = SEALED
	case flow.TransactionStatusPending:
		*t = PENDING
	default:
		*t = ""
	}
}

func (p *ProposalKey) Build(key flow.ProposalKey) {
	p.Address = key.Address.String()
	p.KeyIndex = fromUint64(key.KeyIndex)
	p.SequenceNumber = fromUint64(key.SequenceNumber)
}
