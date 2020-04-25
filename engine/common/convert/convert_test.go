package convert_test

import (
	"fmt"
	"testing"

	sdk "github.com/onflow/flow-go-sdk"
	sdkconvert "github.com/onflow/flow-go-sdk/client/convert"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/dapperlabs/flow-go/engine/common/convert"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestConvertTransaction(t *testing.T) {
	tx := unittest.TransactionBodyFixture()

	msg := convert.TransactionToMessage(tx)
	converted, err := convert.MessageToTransaction(msg)
	assert.Nil(t, err)

	assert.Equal(t, tx, converted)
	assert.Equal(t, tx.ID(), converted.ID())
}

// Make sure we are compatible with SDK transactions.
func TestSDK(t *testing.T) {
	sdkTx := sdk.NewTransaction().
		SetReferenceBlockID(sdk.Identifier{0xff}).
		SetScript([]byte("asdf")).
		SetPayer(sdk.RootAddress).
		SetProposalKey(sdk.RootAddress, 0, 0)

	protoTx := sdkconvert.TransactionToMessage(*sdkTx)

	flowTx, err := convert.MessageToTransaction(protoTx)
	require.Nil(t, err)

	assert.Equal(t, sdkTx.Encode(), flowTx.Encode())
	assert.Equal(t, sdkTx.ID().String(), flowTx.ID().String())

	fmt.Printf("%x\n", sdkTx.Encode())
	fmt.Printf("%x\n", flowTx.Encode())

	fmt.Println(sdkTx.ID().String())
	fmt.Println(flowTx.ID().String())
}
