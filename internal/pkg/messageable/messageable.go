package messageable

import (
	"github.com/golang/protobuf/ptypes"

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
	timestamp, _ := ptypes.Timestamp(m.GetTimestamp())

	return &types.SignedTransaction{
		Script:         m.GetScript(),
		Nonce:          m.GetNonce(),
		ComputeLimit:   m.GetComputeLimit(),
		ComputeUsed:    m.GetComputeUsed(),
		Timestamp:      timestamp,
		PayerSignature: MessageToAccountSignature(m.GetPayerSignature()),
	}
}

func SignedTransactionToMessage(t *types.SignedTransaction) *shared.SignedTransaction {
	timestamp, _ := ptypes.TimestampProto(t.Timestamp)

	return &shared.SignedTransaction{
		Script:         t.Script,
		Nonce:          t.Nonce,
		ComputeLimit:   t.ComputeLimit,
		ComputeUsed:    t.ComputeUsed,
		Timestamp:      timestamp,
		PayerSignature: AccountSignatureToMessage(t.PayerSignature),
	}
}
