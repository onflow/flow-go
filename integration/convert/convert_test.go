package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/integration/convert"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestConvertTx(t *testing.T) {
	tx := unittest.TransactionBodyFixture()
	sdkTx := convert.ToSDKTx(tx)
	assert.Equal(t, tx.Encode(), sdkTx.Encode())
	assert.Equal(t, tx.ID().String(), sdkTx.ID().String())
}
