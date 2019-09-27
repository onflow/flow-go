package proto

import (
	"errors"

	"github.com/dapperlabs/flow-go/pkg/grpc/shared"
	"github.com/dapperlabs/flow-go/pkg/types"
)

var ErrEmptyMessage = errors.New("protobuf message is empty")

func MessageToAccountSignature(m *shared.AccountSignature) types.AccountSignature {
	return types.AccountSignature{
		Account:   types.BytesToAddress(m.GetAccount()),
		Signature: m.GetSignature(),
	}
}

func AccountSignatureToMessage(t types.AccountSignature) *shared.AccountSignature {
	return &shared.AccountSignature{
		Account:   t.Account.Bytes(),
		Signature: t.Signature,
	}
}

func MessageToTransaction(m *shared.Transaction) (types.Transaction, error) {
	if m == nil {
		return types.Transaction{}, ErrEmptyMessage
	}

	scriptAccounts := make([]types.Address, len(m.ScriptAccounts))
	for i, account := range m.ScriptAccounts {
		scriptAccounts[i] = types.BytesToAddress(account)
	}

	signatures := make([]types.AccountSignature, len(m.Signatures))
	for i, accountSig := range m.Signatures {
		signatures[i] = MessageToAccountSignature(accountSig)
	}

	return types.Transaction{
		Script:             m.GetScript(),
		ReferenceBlockHash: m.ReferenceBlockHash,
		Nonce:              m.GetNonce(),
		ComputeLimit:       m.GetComputeLimit(),
		PayerAccount:       types.BytesToAddress(m.PayerAccount),
		ScriptAccounts:     scriptAccounts,
		Signatures:         signatures,
	}, nil
}

func TransactionToMessage(t types.Transaction) *shared.Transaction {
	scriptAccounts := make([][]byte, len(t.ScriptAccounts))
	for i, account := range t.ScriptAccounts {
		scriptAccounts[i] = account.Bytes()
	}

	signatures := make([]*shared.AccountSignature, len(t.Signatures))
	for i, accountSig := range t.Signatures {
		signatures[i] = AccountSignatureToMessage(accountSig)
	}

	return &shared.Transaction{
		Script:             t.Script,
		ReferenceBlockHash: t.ReferenceBlockHash,
		Nonce:              t.Nonce,
		ComputeLimit:       t.ComputeLimit,
		PayerAccount:       t.PayerAccount.Bytes(),
		ScriptAccounts:     scriptAccounts,
		Signatures:         signatures,
	}
}
