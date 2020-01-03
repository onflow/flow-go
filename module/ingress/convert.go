package ingress

import (
	"fmt"

	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/protobuf/sdk/entities"
)

func MessageToAccountSignature(m *entities.AccountSignature) flow.AccountSignature {
	return flow.AccountSignature{
		Account:   flow.BytesToAddress(m.GetAccount()),
		Signature: m.GetSignature(),
	}
}

func AccountSignatureToMessage(a flow.AccountSignature) *entities.AccountSignature {
	return &entities.AccountSignature{
		Account:   a.Account.Bytes(),
		Signature: a.Signature,
	}
}

func MessageToTransaction(m *entities.Transaction) (flow.Transaction, error) {
	if m == nil {
		return flow.Transaction{}, fmt.Errorf("message is empty")
	}

	scriptAccounts := make([]flow.Address, len(m.ScriptAccounts))
	for i, account := range m.ScriptAccounts {
		scriptAccounts[i] = flow.BytesToAddress(account)
	}

	signatures := make([]flow.AccountSignature, len(m.Signatures))
	for i, accountSig := range m.Signatures {
		signatures[i] = MessageToAccountSignature(accountSig)
	}

	return flow.Transaction{TransactionBody: flow.TransactionBody{
		Script:             m.GetScript(),
		ReferenceBlockHash: m.ReferenceBlockHash,
		Nonce:              m.GetNonce(),
		ComputeLimit:       m.GetComputeLimit(),
		PayerAccount:       flow.BytesToAddress(m.PayerAccount),
		ScriptAccounts:     scriptAccounts,
		Signatures:         signatures,
	},
		Status: flow.TransactionStatus(m.GetStatus()),
	}, nil
}

func TransactionToMessage(t flow.Transaction) *entities.Transaction {
	scriptAccounts := make([][]byte, len(t.ScriptAccounts))
	for i, account := range t.ScriptAccounts {
		scriptAccounts[i] = account.Bytes()
	}

	signatures := make([]*entities.AccountSignature, len(t.Signatures))
	for i, accountSig := range t.Signatures {
		signatures[i] = AccountSignatureToMessage(accountSig)
	}

	return &entities.Transaction{
		Script:             t.Script,
		ReferenceBlockHash: t.ReferenceBlockHash,
		Nonce:              t.Nonce,
		ComputeLimit:       t.ComputeLimit,
		PayerAccount:       t.PayerAccount.Bytes(),
		ScriptAccounts:     scriptAccounts,
		Signatures:         signatures,
		Status:             entities.TransactionStatus(t.Status),
	}
}
