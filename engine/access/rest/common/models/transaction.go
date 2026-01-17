package models

import (
	"github.com/onflow/flow-go/engine/access/rest/util"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
)

func (t *Transaction) Build(tx *flow.TransactionBody, txr *accessmodel.TransactionResult, link LinkGenerator) {
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
		txResult := NewTransactionResult(txr, tx.ID(), link, nil, false)
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
	t.GasLimit = util.FromUint(tx.GasLimit)
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
	t.KeyIndex = util.FromUint(sig.KeyIndex)
	t.Signature = util.ToBase64(sig.Signature)
	t.ExtensionData = util.ToBase64(sig.ExtensionData)
}

// NewTransactionResult builds the REST API response model for GetTransactionResult.
func NewTransactionResult(
	txr *accessmodel.TransactionResult,
	txID flow.Identifier,
	link LinkGenerator,
	metadata *accessmodel.ExecutorMetadata,
	shouldIncludeMetadata bool,
) TransactionResult {
	var status TransactionStatus
	status.Build(txr.Status)

	var execution TransactionExecution
	execution.Build(txr)

	var meta *Metadata
	if shouldIncludeMetadata {
		meta = NewMetadata(metadata)
	}
	self, _ := SelfLink(txID, link.TransactionResultLink)

	transactionResult := TransactionResult{
		Status:          &status,
		Execution:       &execution,
		StatusCode:      int32(txr.StatusCode),
		ErrorMessage:    txr.ErrorMessage,
		ComputationUsed: util.FromUint(uint64(0)), // todo: define this
		Events:          NewEvents(txr.Events),
		Links:           self,
		Metadata:        meta,
	}

	if txr.BlockID != flow.ZeroID { // don't send back 0 ID
		transactionResult.BlockId = txr.BlockID.String()
	}

	if txr.CollectionID != flow.ZeroID { // don't send back 0 ID
		transactionResult.CollectionId = txr.CollectionID.String()
	}

	return transactionResult
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

func (t *TransactionExecution) Build(result *accessmodel.TransactionResult) {
	*t = PENDING_RESULT

	if result.Status == flow.TransactionStatusSealed && result.ErrorMessage == "" {
		*t = SUCCESS_RESULT
	}
	if result.ErrorMessage != "" {
		*t = FAILURE_RESULT
	}
	if result.Status == flow.TransactionStatusExpired {
		*t = FAILURE_RESULT
	}
}

func (p *ProposalKey) Build(key flow.ProposalKey) {
	p.Address = key.Address.String()
	p.KeyIndex = util.FromUint(key.KeyIndex)
	p.SequenceNumber = util.FromUint(key.SequenceNumber)
}
