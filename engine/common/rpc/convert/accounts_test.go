package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/utils/unittest"
)

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
