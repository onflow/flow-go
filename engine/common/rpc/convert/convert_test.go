package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestConvertTransaction(t *testing.T) {
	tx := unittest.TransactionBodyFixture()

	msg := convert.TransactionToMessage(tx)
	converted, err := convert.MessageToTransaction(msg, flow.Testnet.Chain())
	assert.Nil(t, err)

	assert.Equal(t, tx, converted)
	assert.Equal(t, tx.ID(), converted.ID())
}

func TestConvertAccountKey(t *testing.T) {
	privateKey, _ := unittest.AccountKeyDefaultFixture()
	accountKey := privateKey.PublicKey(fvm.AccountKeyWeightThreshold)

	// Explicitly test if Revoked is properly converted
	accountKey.Revoked = true

	msg, err := convert.AccountKeyToMessage(accountKey)
	assert.Nil(t, err)

	converted, err := convert.MessageToAccountKey(msg)
	assert.Nil(t, err)

	assert.Equal(t, accountKey, *converted)
	assert.Equal(t, accountKey.PublicKey, converted.PublicKey)
	assert.Equal(t, accountKey.Revoked, converted.Revoked)
}

func TestConvertEvents(t *testing.T) {
	messages := convert.EventsToMessages(nil)
	assert.Len(t, messages, 0)
}
