package convert

import (
	sdk "github.com/onflow/flow-go-sdk"
	sdkconvert "github.com/onflow/flow-go-sdk/client/convert"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/model/flow"
)

func ToSDKTx(tx flow.TransactionBody) sdk.Transaction {
	proto := convert.TransactionToMessage(tx)
	sdkTx, err := sdkconvert.MessageToTransaction(proto)
	if err != nil {
		panic(err)
	}
	return sdkTx
}

func IDFromSDK(sdkID sdk.Identifier) flow.Identifier {
	return flow.HashToID(sdkID[:])
}

func ToSDKID(id flow.Identifier) sdk.Identifier {
	return sdk.HashToID(id[:])
}

func ToSDKAddress(addr flow.Address) sdk.Address {
	return sdk.Address(addr)
}

func ToSDKTransactionSignature(sig flow.TransactionSignature) sdk.TransactionSignature {
	return sdk.TransactionSignature{
		Address:     sdk.Address(sig.Address),
		SignerIndex: sig.SignerIndex,
		Signature:   sig.Signature,
		KeyIndex:    int(sig.KeyIndex),
	}
}

func ToSDKProposalKey(key flow.ProposalKey) sdk.ProposalKey {
	return sdk.ProposalKey{
		Address:        sdk.Address(key.Address),
		KeyIndex:       int(key.KeyIndex),
		SequenceNumber: key.SequenceNumber,
	}
}
