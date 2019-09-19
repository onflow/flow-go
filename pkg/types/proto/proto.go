package proto

import (
	"errors"

	"github.com/golang/protobuf/ptypes"

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

func MessageToSignedTransaction(m *shared.SignedTransaction) (types.SignedTransaction, error) {
	if m == nil {
		return types.SignedTransaction{}, ErrEmptyMessage
	}

	timestamp, err := ptypes.Timestamp(m.GetTimestamp())
	if err != nil {
		return types.SignedTransaction{}, err
	}

	return types.SignedTransaction{
		Script:         m.GetScript(),
		Nonce:          m.GetNonce(),
		ComputeLimit:   m.GetComputeLimit(),
		ComputeUsed:    m.GetComputeUsed(),
		Timestamp:      timestamp,
		PayerSignature: MessageToAccountSignature(m.GetPayerSignature()),
	}, nil
}

func SignedTransactionToMessage(t types.SignedTransaction) (*shared.SignedTransaction, error) {
	timestamp, err := ptypes.TimestampProto(t.Timestamp)
	if err != nil {
		return nil, err
	}

	return &shared.SignedTransaction{
		Script:         t.Script,
		Nonce:          t.Nonce,
		ComputeLimit:   t.ComputeLimit,
		ComputeUsed:    t.ComputeUsed,
		Timestamp:      timestamp,
		PayerSignature: AccountSignatureToMessage(t.PayerSignature),
	}, nil
}
