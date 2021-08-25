package flow_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestAccountPublicKey_MarshalJSON(t *testing.T) {
	accountPrivateKey, err := unittest.AccountKeyDefaultFixture()
	assert.NoError(t, err)

	accountKeyA := accountPrivateKey.PublicKey(42)

	encAccountKey, err := json.Marshal(&accountKeyA)
	assert.NoError(t, err)

	var accountKeyB flow.AccountPublicKey

	err = json.Unmarshal(encAccountKey, &accountKeyB)
	assert.NoError(t, err)

	assert.Equal(t, accountKeyA, accountKeyB)
}

func TestAccountPublicKey_CompactEncodingV0(t *testing.T) {
	// test with revoke flag
	accountPrivateKey, err := unittest.AccountKeyDefaultFixture()
	assert.NoError(t, err)
	accountKeyA := accountPrivateKey.PublicKey(42)

	t.Run("too short data", func(t *testing.T) {
		encAccountKey, err := accountKeyA.CompactEncode(0)
		assert.NoError(t, err)

		var accountKeyB flow.AccountPublicKey
		err = accountKeyB.CompactDecode(encAccountKey[:10])
		assert.Error(t, err)
	})

	t.Run("invalid version", func(t *testing.T) {
		encAccountKey, err := accountKeyA.CompactEncode(0)
		assert.NoError(t, err)

		var accountKeyB flow.AccountPublicKey
		err = accountKeyB.CompactDecode(encAccountKey)
		assert.NoError(t, err)
		assert.Equal(t, accountKeyA, accountKeyB)

		encAccountKey, err = accountKeyA.CompactEncode(1)
		assert.Error(t, err)
	})

	t.Run("with different revoke flags", func(t *testing.T) {
		accountKeyA.Revoked = true
		encAccountKey, err := accountKeyA.CompactEncode(0)
		assert.NoError(t, err)

		var accountKeyB flow.AccountPublicKey
		err = accountKeyB.CompactDecode(encAccountKey)
		assert.NoError(t, err)

		assert.True(t, accountKeyB.Revoked)
		assert.Equal(t, accountKeyA, accountKeyB)

		accountKeyA.Revoked = false
		encAccountKey, err = accountKeyA.CompactEncode(0)
		assert.NoError(t, err)

		err = accountKeyB.CompactDecode(encAccountKey)
		assert.NoError(t, err)
		assert.False(t, accountKeyB.Revoked)
		assert.Equal(t, accountKeyA, accountKeyB)
	})

	t.Run("test with various key weights", func(t *testing.T) {
		accountKeyA.Weight = 10
		encAccountKey, err := accountKeyA.CompactEncode(0)
		assert.NoError(t, err)

		var accountKeyB flow.AccountPublicKey
		err = accountKeyB.CompactDecode(encAccountKey)
		assert.NoError(t, err)

		assert.Equal(t, accountKeyB.Weight, 10)
		assert.Equal(t, accountKeyA, accountKeyB)

		accountKeyA.Weight = 1000
		encAccountKey, err = accountKeyA.CompactEncode(0)
		assert.NoError(t, err)

		err = accountKeyB.CompactDecode(encAccountKey)
		assert.NoError(t, err)
		assert.Equal(t, accountKeyB.Weight, 1000)
		assert.Equal(t, accountKeyA, accountKeyB)

		accountKeyA.Weight = 2000
		encAccountKey, err = accountKeyA.CompactEncode(0)
		assert.Error(t, err)

		accountKeyA.Weight = 1000
	})

	t.Run("test with payload size tampered", func(t *testing.T) {
		encAccountKey, err := accountKeyA.CompactEncode(0)
		assert.NoError(t, err)

		// add extra bytes
		encAccountKey = append(encAccountKey, []byte("extra")...)

		var accountKeyB flow.AccountPublicKey
		err = accountKeyB.CompactDecode(encAccountKey)
		assert.Error(t, err)

		encAccountKey, err = accountKeyA.CompactEncode(0)
		assert.NoError(t, err)

		// remove some data
		shortEncodedData := encAccountKey[:len(encAccountKey)-2]
		err = accountKeyB.CompactDecode(shortEncodedData)
		assert.Error(t, err)
	})
}

func BenchmarkAccountPublicKeyDecode_JSON(b *testing.B) {
	accountPrivateKey, err := unittest.AccountKeyDefaultFixture()
	assert.NoError(b, err)
	accountKeyA := accountPrivateKey.PublicKey(1000)

	var accountKeyB flow.AccountPublicKey
	encAccountKey, _ := accountKeyA.MarshalJSON()
	for i := 0; i < b.N; i++ {
		_ = accountKeyB.UnmarshalJSON(encAccountKey)
	}
}
func BenchmarkAccountPublicKeyDecode_Compact(b *testing.B) {
	accountPrivateKey, err := unittest.AccountKeyDefaultFixture()
	assert.NoError(b, err)
	accountKeyA := accountPrivateKey.PublicKey(1000)

	var accountKeyB flow.AccountPublicKey
	encAccountKey, _ := accountKeyA.CompactEncode(0)
	for i := 0; i < b.N; i++ {
		_ = accountKeyB.CompactDecode(encAccountKey)
	}
}

func BenchmarkAccountPublicKeyDecode_RLP(b *testing.B) {
	accountPrivateKey, err := unittest.AccountKeyDefaultFixture()
	assert.NoError(b, err)
	accountKeyA := accountPrivateKey.PublicKey(1000)

	encAccountKey, _ := flow.EncodeAccountPublicKey(accountKeyA)
	for i := 0; i < b.N; i++ {
		_, _ = flow.DecodeAccountPublicKey(encAccountKey, 0)
	}
}
