package convert

import (
	"fmt"

	sdk "github.com/onflow/flow-go-sdk"
	sdkconvert "github.com/onflow/flow-go-sdk/client/convert"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/model/flow"
)

func TxFromSDK(sdkTx sdk.Transaction) flow.TransactionBody {
	fmt.Printf("sdk: %s\n", sdkTx.ReferenceBlockID)
	proto := sdkconvert.TransactionToMessage(sdkTx)
	fmt.Printf("proto: %x\n", proto.GetReferenceBlockId())
	tx, err := convert.MessageToTransaction(proto)
	if err != nil {
		panic(err)
	}
	fmt.Printf("flow: %s\n", tx.ReferenceBlockID)
	return tx
}

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

func AddressFromSDK(sdkAddr sdk.Address) flow.Address {
	return flow.BytesToAddress(sdkAddr[:])
}

func ToSDKAddress(addr flow.Address) sdk.Address {
	return sdk.BytesToAddress(addr[:])
}
