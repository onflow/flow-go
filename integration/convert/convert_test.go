package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/integration/convert"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConvertTx(t *testing.T) {
	tx := unittest.TransactionBodyFixture()
	sdkTx := convert.ToSDKTx(tx)
	assert.Equal(t, tx.Fingerprint(), sdkTx.Encode())
	assert.Equal(t, tx.ID().String(), sdkTx.ID().String())
}
