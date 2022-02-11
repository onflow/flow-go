package models

import (
	"github.com/onflow/flow-go/access"
	"github.com/onflow/flow-go/engine/access/rest/util"
	"github.com/onflow/flow-go/model/flow"
)

func (t *Transaction) Build(tx *flow.TransactionBody, txr *access.TransactionResult, link LinkGenerator) {
	args := make([]string, len(tx.Arguments))
	for i, arg := range tx.Arguments {
		args[i] = util.ToBase64(arg)
	}

	auths := make([]string, len(tx.Authorizers))
	for i, auth := range tx.Authorizers {
		auths[i] = auth.String()
	}

	// if transaction result is provided then add that to the response, else add the result link to the expandable
	t.Expandable = &TransactionExpandable{}
	if txr != nil {
		var txResult TransactionResult
		txResult.Build(txr, tx.ID(), link)
		t.Result = &txResult
	} else {
		resultLink, _ := link.TransactionResultLink(tx.ID())
		t.Expandable.Result = resultLink
	}

	var payloadSigs TransactionSignatures
	payloadSigs.Build(tx.PayloadSignatures)

	var envelopeSigs TransactionSignatures
	envelopeSigs.Build(tx.EnvelopeSignatures)

	var proposalKey ProposalKey
	proposalKey.Build(tx.ProposalKey)

	t.Id = tx.ID().String()
	t.Script = util.ToBase64(tx.Script)
	t.Arguments = args
	t.ReferenceBlockId = tx.ReferenceBlockID.String()
	t.GasLimit = util.FromUint64(tx.GasLimit)
	t.Payer = tx.Payer.String()
	t.ProposalKey = &proposalKey
	t.Authorizers = auths
	t.PayloadSignatures = payloadSigs
	t.EnvelopeSignatures = envelopeSigs

	self, _ := SelfLink(tx.ID(), link.TransactionLink)
	t.Links = self
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
	t.KeyIndex = util.FromUint64(sig.KeyIndex)
	t.Signature = util.ToBase64(sig.Signature)
}

func (t *TransactionResult) Build(txr *access.TransactionResult, txID flow.Identifier, link LinkGenerator) {
	var status TransactionStatus
	status.Build(txr.Status)

	var events Events
	events.Build(txr.Events)

	if txr.BlockID != flow.ZeroID { // don't send back 0 ID
		t.BlockId = txr.BlockID.String()
	}

	t.Status = &status
	t.StatusCode = int32(txr.StatusCode)
	t.ErrorMessage = txr.ErrorMessage
	t.ComputationUsed = util.FromUint64(0)
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
	p.KeyIndex = util.FromUint64(key.KeyIndex)
	p.SequenceNumber = util.FromUint64(key.SequenceNumber)
}
