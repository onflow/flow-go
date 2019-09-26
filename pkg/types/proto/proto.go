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

	return types.Transaction{
		Script:         m.GetScript(),
		Nonce:          m.GetNonce(),
		ComputeLimit:   m.GetComputeLimit(),
		PayerSignature: MessageToAccountSignature(m.GetPayerSignature()),
	}, nil
}

func TransactionToMessage(t types.Transaction) *shared.Transaction {
	return &shared.Transaction{
		Script:         t.Script,
		Nonce:          t.Nonce,
		ComputeLimit:   t.ComputeLimit,
		PayerSignature: AccountSignatureToMessage(t.PayerSignature),
	}
}
