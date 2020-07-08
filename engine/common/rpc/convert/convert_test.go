package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/engine/common/rpc/convert"
	"github.com/dapperlabs/flow-go/model/flow"
	"github.com/dapperlabs/flow-go/utils/unittest"
)

func TestConvertTransaction(t *testing.T) {
	tx := unittest.TransactionBodyFixture()

	msg := convert.TransactionToMessage(tx)
	converted, err := convert.MessageToTransaction(msg, flow.Mainnet.Chain())
	assert.Nil(t, err)

	assert.Equal(t, tx, converted)
	assert.Equal(t, tx.ID(), converted.ID())
}
