package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/integration/convert"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestConvertTx(t *testing.T) {
	// TODO tests fail because sdk is using different ID generation method
	t.Skip("skipping until SDK is updated")
	tx := unittest.TransactionBodyFixture()
	sdkTx := convert.ToSDKTx(tx)
	assert.Equal(t, tx.Fingerprint(), sdkTx.Encode())
	assert.Equal(t, tx.ID().String(), sdkTx.ID().String())
}
