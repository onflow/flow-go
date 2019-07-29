package messageable

import (
	"github.com/dapperlabs/bamboo-node/pkg/grpc/shared"
	"github.com/dapperlabs/bamboo-node/pkg/types"
)

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

func MessageToSignedTransaction(m *shared.SignedTransaction) *types.SignedTransaction {
	return &types.SignedTransaction{
		Script:         m.GetScript(),
		Nonce:          m.GetNonce(),
		ComputeLimit:   m.GetComputeLimit(),
		ComputeUsed:    m.GetComputeUsed(),
		PayerSignature: MessageToAccountSignature(m.GetPayerSignature()),
	}
}

func SignedTransactionToMessage(t *types.SignedTransaction) *shared.SignedTransaction {
	return &shared.SignedTransaction{
		Script:         t.Script,
		Nonce:          t.Nonce,
		ComputeLimit:   t.ComputeLimit,
		ComputeUsed:    t.ComputeUsed,
		PayerSignature: AccountSignatureToMessage(t.PayerSignature),
	}
}
