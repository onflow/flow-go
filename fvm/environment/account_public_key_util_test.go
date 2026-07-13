package environment_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	envMock "github.com/onflow/flow-go/fvm/environment/mock"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func TestGetAccountPublicKey0(t *testing.T) {
	address := flow.BytesToAddress([]byte{1})

	t.Run("no key", func(t *testing.T) {
		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(nil, nil)

		_, err := environment.GetAccountPublicKey0(accounts, address)
		require.True(t, errors.IsAccountPublicKeyNotFoundError(err))
	})

	t.Run("invalid key", func(t *testing.T) {
		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return([]byte{0, 1, 2}, nil)

		_, err := environment.GetAccountPublicKey0(accounts, address)
		require.ErrorContains(t, err, "failed to decode account public key 0")
	})

	t.Run("has key", func(t *testing.T) {
		expectedPublicKey := newAccountPublicKey(t, 1000)
		encodedPublicKey, err := flow.EncodeAccountPublicKey(expectedPublicKey)
		require.NoError(t, err)

		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(encodedPublicKey, nil)

		publicKey, err := environment.GetAccountPublicKey0(accounts, address)
		require.NoError(t, err)
		require.Equal(t, expectedPublicKey, publicKey)
	})
}

func TestSetAccountPublicKey0(t *testing.T) {
	address := flow.BytesToAddress([]byte{1})

	expectedPublicKey := newAccountPublicKey(t, 1000)
	encodedPublicKey, err := flow.EncodeAccountPublicKey(expectedPublicKey)
	require.NoError(t, err)

	accounts := envMock.NewAccounts(t)
	accounts.On("SetValue", flow.AccountPublicKey0RegisterID(address), encodedPublicKey).
		Return(nil)

	err = environment.SetAccountPublicKey0(accounts, address, expectedPublicKey)
	require.NoError(t, err)
}

func TestRevokeAccountPublicKey0(t *testing.T) {
	address := flow.BytesToAddress([]byte{1})

	t.Run("no key", func(t *testing.T) {
		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(nil, nil)

		err := environment.RevokeAccountPublicKey0(accounts, address)
		require.True(t, errors.IsAccountPublicKeyNotFoundError(err))
	})

	t.Run("has key", func(t *testing.T) {
		publicKey := newAccountPublicKey(t, 1000)
		encodedPublicKey, err := flow.EncodeAccountPublicKey(publicKey)
		require.NoError(t, err)

		publicKey.Revoked = true
		encodedRevokedPublicKey, err := flow.EncodeAccountPublicKey(publicKey)
		require.NoError(t, err)

		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(encodedPublicKey, nil)
		accounts.On("SetValue", flow.AccountPublicKey0RegisterID(address), encodedRevokedPublicKey).
			Return(nil)

		err = environment.RevokeAccountPublicKey0(accounts, address)
		require.NoError(t, err)
	})
}

func TestGetSequenceNumber(t *testing.T) {
	address := flow.BytesToAddress([]byte{1})

	t.Run("no sequence number", func(t *testing.T) {
		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(nil, nil)
		accounts.On("GetValue", flow.AccountPublicKeySequenceNumberRegisterID(address, 1)).
			Return(nil, nil)

		_, err := environment.GetAccountPublicKeySequenceNumber(accounts, address, 0)
		require.True(t, errors.IsAccountPublicKeyNotFoundError(err))

		seqNum, err := environment.GetAccountPublicKeySequenceNumber(accounts, address, 1)
		require.NoError(t, err)
		require.Equal(t, uint64(0), seqNum)
	})

	t.Run("has sequence number", func(t *testing.T) {
		expectedSeqNum0 := uint64(1)
		expectedSeqNum1 := uint64(2)

		publicKey0 := newAccountPublicKey(t, 1000)
		publicKey0.SeqNumber = expectedSeqNum0
		encodedPublicKey0, err := flow.EncodeAccountPublicKey(publicKey0)
		require.NoError(t, err)

		encodedSeqNum1, err := flow.EncodeSequenceNumber(expectedSeqNum1)
		require.NoError(t, err)

		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(encodedPublicKey0, nil)
		accounts.On("GetValue", flow.AccountPublicKeySequenceNumberRegisterID(address, 1)).
			Return(encodedSeqNum1, nil)

		seqNum0, err := environment.GetAccountPublicKeySequenceNumber(accounts, address, 0)
		require.NoError(t, err)
		require.Equal(t, expectedSeqNum0, seqNum0)

		seqNum1, err := environment.GetAccountPublicKeySequenceNumber(accounts, address, 1)
		require.NoError(t, err)
		require.Equal(t, expectedSeqNum1, seqNum1)
	})
}

func TestIncrementSequenceNumber(t *testing.T) {
	address := flow.BytesToAddress([]byte{1})

	t.Run("no sequence number", func(t *testing.T) {
		encodedSeqNum, err := flow.EncodeSequenceNumber(1)
		require.NoError(t, err)

		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(nil, nil)
		accounts.On("GetValue", flow.AccountPublicKeySequenceNumberRegisterID(address, 1)).
			Return(nil, nil)
		accounts.On("SetValue", flow.AccountPublicKeySequenceNumberRegisterID(address, 1), encodedSeqNum).
			Return(nil, nil)

		err = environment.IncrementAccountPublicKeySequenceNumber(accounts, address, 0)
		require.True(t, errors.IsAccountPublicKeyNotFoundError(err))

		err = environment.IncrementAccountPublicKeySequenceNumber(accounts, address, 1)
		require.NoError(t, err)
	})

	t.Run("has sequence number", func(t *testing.T) {
		storedSeqNum0 := uint64(1)
		storedSeqNum1 := uint64(3)

		publicKey0 := newAccountPublicKey(t, 1000)
		publicKey0.SeqNumber = storedSeqNum0
		encodedPublicKey0, err := flow.EncodeAccountPublicKey(publicKey0)
		require.NoError(t, err)

		publicKey0.SeqNumber++
		encodedIncrementedPublicKey0, err := flow.EncodeAccountPublicKey(publicKey0)
		require.NoError(t, err)

		encodedSeqNum1, err := flow.EncodeSequenceNumber(storedSeqNum1)
		require.NoError(t, err)

		encodedIncrementedSeqNum1, err := flow.EncodeSequenceNumber(storedSeqNum1 + 1)
		require.NoError(t, err)

		accounts := envMock.NewAccounts(t)

		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(encodedPublicKey0, nil)
		accounts.On("SetValue", flow.AccountPublicKey0RegisterID(address), encodedIncrementedPublicKey0).
			Return(nil)

		accounts.On("GetValue", flow.AccountPublicKeySequenceNumberRegisterID(address, 1)).
			Return(encodedSeqNum1, nil)
		accounts.On("SetValue", flow.AccountPublicKeySequenceNumberRegisterID(address, 1), encodedIncrementedSeqNum1).
			Return(nil)

		err = environment.IncrementAccountPublicKeySequenceNumber(accounts, address, 0)
		require.NoError(t, err)

		err = environment.IncrementAccountPublicKeySequenceNumber(accounts, address, 1)
		require.NoError(t, err)
	})
}

func TestGetStoredPublicKey(t *testing.T) {
	address := flow.BytesToAddress([]byte{1})

	t.Run("0 key", func(t *testing.T) {
		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(nil, nil)

		_, err := environment.GetStoredPublicKey(accounts, address, 0)
		require.True(t, errors.IsAccountPublicKeyNotFoundError(err))
	})

	t.Run("1 key", func(t *testing.T) {
		accountPublicKey1 := newAccountPublicKey(t, 1000)
		encodedAccountPublicKey1, err := flow.EncodeAccountPublicKey(accountPublicKey1)
		require.NoError(t, err)

		expectedStoredPublicKey1 := accountPublicKeyToStoredKey(accountPublicKey1)

		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(encodedAccountPublicKey1, nil)

		spk, err := environment.GetStoredPublicKey(accounts, address, 0)
		require.NoError(t, err)
		require.Equal(t, expectedStoredPublicKey1, spk)
	})

	t.Run("2 keys", func(t *testing.T) {
		accountPublicKey1 := newAccountPublicKey(t, 1000)
		encodedAccountPublicKey1, err := flow.EncodeAccountPublicKey(accountPublicKey1)
		require.NoError(t, err)

		expectedStoredPublicKey1 := accountPublicKeyToStoredKey(accountPublicKey1)
		expectedStoredPublicKey2 := accountPublicKeyToStoredKey(newAccountPublicKey(t, 1))

		encodedStoredPublicKey2, err := flow.EncodeStoredPublicKey(expectedStoredPublicKey2)
		require.NoError(t, err)

		encodedBatchPublicKey := newBatchPublicKey(t, []*flow.StoredPublicKey{nil, &expectedStoredPublicKey2})
		expectedBatchPublicKeys := [][]byte{[]byte{}, encodedStoredPublicKey2}

		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(encodedAccountPublicKey1, nil)
		accounts.On("GetValue", flow.AccountBatchPublicKeyRegisterID(address, 0)).
			Return(encodedBatchPublicKey, nil)

		spk, err := environment.GetStoredPublicKey(accounts, address, 0)
		require.NoError(t, err)
		require.Equal(t, expectedStoredPublicKey1, spk)

		spk, err = environment.GetStoredPublicKey(accounts, address, 1)
		require.NoError(t, err)
		require.Equal(t, expectedStoredPublicKey2, spk)

		encodedKeys, err := environment.DecodeBatchPublicKeys(encodedBatchPublicKey)
		require.NoError(t, err)
		require.Equal(t, expectedBatchPublicKeys, encodedKeys)
	})

	t.Run("one full batch", func(t *testing.T) {
		accountPublicKey1 := newAccountPublicKey(t, 1000)
		encodedAccountPublicKey1, err := flow.EncodeAccountPublicKey(accountPublicKey1)
		require.NoError(t, err)

		storedKeyCount := environment.MaxPublicKeyCountInBatch
		expectedStoredKeys := make([]*flow.StoredPublicKey, storedKeyCount)
		expectedBatchPublicKeys := make([][]byte, storedKeyCount)
		expectedBatchPublicKeys[0] = []byte{}

		for i := 1; i < environment.MaxPublicKeyCountInBatch; i++ {
			key := accountPublicKeyToStoredKey(newAccountPublicKey(t, 1))
			expectedStoredKeys[i] = &key

			encodedStoredPublicKey, err := flow.EncodeStoredPublicKey(key)
			require.NoError(t, err)

			expectedBatchPublicKeys[i] = encodedStoredPublicKey
		}

		encodedBatchPublicKey := newBatchPublicKey(t, expectedStoredKeys)

		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(encodedAccountPublicKey1, nil)
		accounts.On("GetValue", flow.AccountBatchPublicKeyRegisterID(address, 0)).
			Return(encodedBatchPublicKey, nil)

		spk, err := environment.GetStoredPublicKey(accounts, address, 0)
		require.NoError(t, err)
		require.Equal(t, accountPublicKeyToStoredKey(accountPublicKey1), spk)

		for i := 1; i < storedKeyCount; i++ {
			spk, err = environment.GetStoredPublicKey(accounts, address, uint32(i))
			require.NoError(t, err)
			require.Equal(t, *expectedStoredKeys[i], spk)
		}

		encodedKeys, err := environment.DecodeBatchPublicKeys(encodedBatchPublicKey)
		require.NoError(t, err)
		require.Equal(t, expectedBatchPublicKeys, encodedKeys)
	})

	t.Run("more than one batch", func(t *testing.T) {
		accountPublicKey1 := newAccountPublicKey(t, 1000)
		encodedAccountPublicKey1, err := flow.EncodeAccountPublicKey(accountPublicKey1)
		require.NoError(t, err)

		storedKeyCount := environment.MaxPublicKeyCountInBatch + 1

		expectedStoredKeys := make([]*flow.StoredPublicKey, storedKeyCount)
		expectedBatchPublicKeys := make([][]byte, storedKeyCount)
		expectedBatchPublicKeys[0] = []byte{}

		for i := 1; i < storedKeyCount; i++ {
			key := accountPublicKeyToStoredKey(newAccountPublicKey(t, 1))
			expectedStoredKeys[i] = &key

			encodedStoredPublicKey, err := flow.EncodeStoredPublicKey(key)
			require.NoError(t, err)

			expectedBatchPublicKeys[i] = encodedStoredPublicKey
		}

		encodedBatchPublicKey1 := newBatchPublicKey(t, expectedStoredKeys[:environment.MaxPublicKeyCountInBatch])
		encodedBatchPublicKey2 := newBatchPublicKey(t, expectedStoredKeys[environment.MaxPublicKeyCountInBatch:])

		expectedBatchPublicKeys1 := expectedBatchPublicKeys[:environment.MaxPublicKeyCountInBatch]
		expectedBatchPublicKeys2 := expectedBatchPublicKeys[environment.MaxPublicKeyCountInBatch:]

		accounts := envMock.NewAccounts(t)
		accounts.On("GetValue", flow.AccountPublicKey0RegisterID(address)).
			Return(encodedAccountPublicKey1, nil)
		accounts.On("GetValue", flow.AccountBatchPublicKeyRegisterID(address, 0)).
			Return(encodedBatchPublicKey1, nil)
		accounts.On("GetValue", flow.AccountBatchPublicKeyRegisterID(address, 1)).
			Return(encodedBatchPublicKey2, nil)

		spk, err := environment.GetStoredPublicKey(accounts, address, 0)
		require.NoError(t, err)
		require.Equal(t, accountPublicKeyToStoredKey(accountPublicKey1), spk)

		for i := 1; i < storedKeyCount; i++ {
			spk, err = environment.GetStoredPublicKey(accounts, address, uint32(i))
			require.NoError(t, err)
			require.Equal(t, *expectedStoredKeys[i], spk)
		}

		encodedKeys1, err := environment.DecodeBatchPublicKeys(encodedBatchPublicKey1)
		require.NoError(t, err)
		require.Equal(t, expectedBatchPublicKeys1, encodedKeys1)

		encodedKeys2, err := environment.DecodeBatchPublicKeys(encodedBatchPublicKey2)
		require.NoError(t, err)
		require.Equal(t, expectedBatchPublicKeys2, encodedKeys2)
	})
}

func TestAppendStoredPublicKey(t *testing.T) {
	address := flow.BytesToAddress([]byte{1})

	storedKeyCount := environment.MaxPublicKeyCountInBatch + 2

	expectedStoredKeys := make([]*flow.StoredPublicKey, storedKeyCount)
	for i := 1; i < storedKeyCount; i++ {
		key := accountPublicKeyToStoredKey(newAccountPublicKey(t, 1))
		expectedStoredKeys[i] = &key
	}

	for i := 1; i < storedKeyCount; i++ {
		batchNum := i / environment.MaxPublicKeyCountInBatch
		keyIndexInBatch := i % environment.MaxPublicKeyCountInBatch

		startStoredKeyIndexInBatch := environment.MaxPublicKeyCountInBatch * batchNum
		endStoredKeyIndexInBatch := startStoredKeyIndexInBatch + keyIndexInBatch

		batchRegisterID := flow.AccountBatchPublicKeyRegisterID(address, uint32(batchNum))

		accounts := envMock.NewAccounts(t)
		if !(batchNum == 0 && keyIndexInBatch == 1) && !(batchNum > 0 && keyIndexInBatch == 0) {
			accounts.On(
				"GetValue",
				batchRegisterID).
				Return(newBatchPublicKey(t, expectedStoredKeys[startStoredKeyIndexInBatch:endStoredKeyIndexInBatch]), nil)
		}
		accounts.On(
			"SetValue",
			batchRegisterID,
			newBatchPublicKey(t, expectedStoredKeys[startStoredKeyIndexInBatch:endStoredKeyIndexInBatch+1])).
			Return(nil)

		encodedKey, err := flow.EncodeStoredPublicKey(*expectedStoredKeys[i])
		require.NoError(t, err)

		err = environment.AppendStoredKey(accounts, address, uint32(i), encodedKey)
		require.NoError(t, err)
	}
}

func newAccountPublicKey(t *testing.T, weight int) flow.AccountPublicKey {
	privateKey, err := unittest.AccountKeyDefaultFixture()
	require.NoError(t, err)

	return privateKey.PublicKey(weight)
}

func accountPublicKeyToStoredKey(apk flow.AccountPublicKey) flow.StoredPublicKey {
	return flow.StoredPublicKey{
		PublicKey: apk.PublicKey,
		SignAlgo:  apk.SignAlgo,
		HashAlgo:  apk.HashAlgo,
	}
}

func newBatchPublicKey(t *testing.T, storedPublicKeys []*flow.StoredPublicKey) []byte {
	var buf []byte
	var err error

	for _, k := range storedPublicKeys {
		var encodedKey []byte

		if k != nil {
			encodedKey, err = flow.EncodeStoredPublicKey(*k)
			require.NoError(t, err)
		}

		b, err := environment.EncodeBatchedPublicKey(encodedKey)
		require.NoError(t, err)

		buf = append(buf, b...)
	}
	return buf
}
