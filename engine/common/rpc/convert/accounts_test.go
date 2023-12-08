package convert_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/crypto"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/engine/common/rpc/convert"
	"github.com/onflow/flow-go/fvm"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestConvertAccount tests that converting an account to and from a protobuf message results in
// the same account
func TestConvertAccount(t *testing.T) {
	t.Parallel()

	a, err := unittest.AccountFixture()
	require.NoError(t, err)

	key2, err := unittest.AccountKeyFixture(128, crypto.ECDSAP256, hash.SHA3_256)
	require.NoError(t, err)

	a.Keys = append(a.Keys, key2.PublicKey(500))

	msg, err := convert.AccountToMessage(a)
	require.NoError(t, err)

	converted, err := convert.MessageToAccount(msg)
	require.NoError(t, err)

	assert.Equal(t, a, converted)
}

// TestConvertAccountKey tests that converting an account key to and from a protobuf message results
// in the same account key
func TestConvertAccountKey(t *testing.T) {
	t.Parallel()

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
