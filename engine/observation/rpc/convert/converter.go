package convert

import (
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"
)

func BlockHeaderToMessage(h *flow.Header) entities.BlockHeader {
	id := h.ID()
	bh := entities.BlockHeader{
		Hash:              id[:],
		PreviousBlockHash: h.ParentID[:],
		Number:            h.Height,
	}
	return bh
}

func TransactionToMessage(t *flow.TransactionBody) *entities.Transaction {

	scriptAccounts := make([][]byte, len(t.ScriptAccounts))
	for i, account := range t.ScriptAccounts {
		scriptAccounts[i] = account.Bytes()
	}

	signatures := make([]*entities.AccountSignature, len(t.Signatures))
	for i, accountSig := range t.Signatures {
		signatures[i] = SignatureToMessage(accountSig)
	}

	m := &entities.Transaction{
		Script:             t.Script,
		ReferenceBlockHash: t.ReferenceBlockID[:],
		Nonce:              t.Nonce,
		ComputeLimit:       t.ComputeLimit,
		PayerAccount:       t.PayerAccount.Bytes(),
		ScriptAccounts:     scriptAccounts,
		Signatures:         signatures,
		Status:             entities.TransactionStatus_STATUS_PENDING,
	}
	return m
}

func SignatureToMessage(s flow.AccountSignature) *entities.AccountSignature {
	return &entities.AccountSignature{
		Account:   s.Account.Bytes(),
		Signature: s.Signature,
	}
}
