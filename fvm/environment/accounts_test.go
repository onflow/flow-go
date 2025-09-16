package environment_test

import (
	"maps"
	"math/rand"
	"slices"
	"testing"

	"github.com/onflow/atree"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/environment"
	"github.com/onflow/flow-go/fvm/errors"
	"github.com/onflow/flow-go/fvm/storage/snapshot"
	"github.com/onflow/flow-go/fvm/storage/testutils"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func TestAccounts_Create(t *testing.T) {
	t.Run("Sets registers", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// account status
		require.Equal(t, len(snapshot.AllRegisterIDs()), 1)
	})

	t.Run("Fails if account exists", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.Create(nil, address)

		require.Error(t, err)
	})

	t.Run("account with 0 keys", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
		}
		registerIDs := snapshot.AllRegisterIDs()
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		value, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, value)

		decodedAccountStatus, err := environment.AccountStatusFromBytes(value)
		require.NoError(t, err)
		require.Equal(t, uint8(4), decodedAccountStatus.Version())
		require.Equal(t, uint32(0), decodedAccountStatus.AccountPublicKeyCount())

		// Test account storage used
		expectedStorageUsed := uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			value))
		require.Equal(t, expectedStorageUsed, decodedAccountStatus.StorageUsed())
	})

	t.Run("account with 1 key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		key0.SeqNumber = 1

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create([]flow.AccountPublicKey{key0}, address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
		}
		registerIDs := snapshot.AllRegisterIDs()
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		decodedAccountStatus, err := environment.AccountStatusFromBytes(accountStatusValue)
		require.NoError(t, err)
		require.Equal(t, uint8(4), decodedAccountStatus.Version())
		require.Equal(t, uint32(1), decodedAccountStatus.AccountPublicKeyCount())

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		decodedKey0, err := flow.DecodeAccountPublicKey(key0Value, 0)
		require.NoError(t, err)
		require.Equal(t, key0, decodedKey0)

		// Test storage used
		var expectedStorageUsed uint64
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		require.Equal(t, expectedStorageUsed, decodedAccountStatus.StorageUsed())
	})

	t.Run("account with 2 unique keys", func(t *testing.T) {
		for _, hasSequenceNumber := range []bool{true, false} {
			key0 := newAccountPublicKey(t, 1000)
			key0.SeqNumber = 1

			key1 := newAccountPublicKey(t, 1000)
			if hasSequenceNumber {
				key1.SeqNumber = 2
			} else {
				key1.SeqNumber = 0
			}

			txnState := testutils.NewSimpleTransaction(nil)
			accounts := environment.NewAccounts(txnState)

			address := flow.HexToAddress("01")

			err := accounts.Create([]flow.AccountPublicKey{key0, key1}, address)
			require.NoError(t, err)

			snapshot, err := txnState.FinalizeMainTransaction()
			require.NoError(t, err)

			// New registers
			expectedNewRegisterIDs := []flow.RegisterID{
				flow.AccountStatusRegisterID(address),
				flow.AccountPublicKey0RegisterID(address),
				flow.AccountBatchPublicKeyRegisterID(address, 0),
			}
			if hasSequenceNumber {
				expectedNewRegisterIDs = append(
					expectedNewRegisterIDs,
					flow.AccountPublicKeySequenceNumberRegisterID(address, 1),
				)
			}
			registerIDs := snapshot.AllRegisterIDs()
			require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
			require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

			// Test account status
			accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
			require.True(t, exists)
			require.NotEmpty(t, accountStatusValue)

			decodedAccountStatus, err := environment.AccountStatusFromBytes(accountStatusValue)
			require.NoError(t, err)
			require.Equal(t, uint8(4), decodedAccountStatus.Version())
			require.Equal(t, uint32(2), decodedAccountStatus.AccountPublicKeyCount())

			// Test account public key 0
			key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
			require.True(t, exists)
			require.NotEmpty(t, key0Value)

			decodedKey0, err := flow.DecodeAccountPublicKey(key0Value, 0)
			require.NoError(t, err)
			require.Equal(t, key0, decodedKey0)

			// Test account public key 1
			key1Value, exists := snapshot.WriteSet[flow.AccountBatchPublicKeyRegisterID(address, 0)]
			require.True(t, exists)
			require.NotEmpty(t, key1Value)

			if hasSequenceNumber {
				// Test sequence number 1
				seqNum1Value, exists := snapshot.WriteSet[flow.AccountPublicKeySequenceNumberRegisterID(address, 1)]
				require.True(t, exists)
				decodedSeqNum1, err := flow.DecodeSequenceNumber(seqNum1Value)
				require.NoError(t, err)
				require.Equal(t, key1.SeqNumber, decodedSeqNum1)
			}

			// Test storage used
			var expectedStorageUsed uint64
			// Include account status register in storage used
			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountStatusRegisterID(address),
				accountStatusValue))
			// Include apk_0 register in storage used
			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountPublicKey0RegisterID(address),
				key0Value))
			// Include pk_b0 register in storage used
			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountBatchPublicKeyRegisterID(address, 1),
				key1Value))
			// Include predefined sequence number 1 register in storage used
			expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
			require.Equal(t, expectedStorageUsed, decodedAccountStatus.StorageUsed())
		}
	})

	t.Run("account with 2 duplicate key", func(t *testing.T) {
		for _, hasSequenceNumber := range []bool{true, false} {
			key0 := newAccountPublicKey(t, 1000)
			key0.SeqNumber = 1

			key1 := key0
			if hasSequenceNumber {
				key1.SeqNumber = 2
			} else {
				key1.SeqNumber = 0
			}

			txnState := testutils.NewSimpleTransaction(nil)
			accounts := environment.NewAccounts(txnState)

			address := flow.HexToAddress("01")

			err := accounts.Create([]flow.AccountPublicKey{key0, key1}, address)
			require.NoError(t, err)

			snapshot, err := txnState.FinalizeMainTransaction()
			require.NoError(t, err)

			// New registers
			expectedNewRegisterIDs := []flow.RegisterID{
				flow.AccountStatusRegisterID(address),
				flow.AccountPublicKey0RegisterID(address),
			}
			if hasSequenceNumber {
				expectedNewRegisterIDs = append(
					expectedNewRegisterIDs,
					flow.AccountPublicKeySequenceNumberRegisterID(address, 1),
				)
			}
			registerIDs := snapshot.AllRegisterIDs()
			require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
			require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

			// Test account status
			accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
			require.True(t, exists)
			require.NotEmpty(t, accountStatusValue)

			decodedAccountStatus, err := environment.AccountStatusFromBytes(accountStatusValue)
			require.NoError(t, err)
			require.Equal(t, uint8(4), decodedAccountStatus.Version())
			require.Equal(t, uint32(2), decodedAccountStatus.AccountPublicKeyCount())

			// Test account public key 0
			key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
			require.True(t, exists)
			require.NotEmpty(t, key0Value)

			decodedKey0, err := flow.DecodeAccountPublicKey(key0Value, 0)
			require.NoError(t, err)
			require.Equal(t, key0, decodedKey0)

			if hasSequenceNumber {
				// Test sequence number 1
				seqNum1Value, exists := snapshot.WriteSet[flow.AccountPublicKeySequenceNumberRegisterID(address, 1)]
				require.True(t, exists)
				decodedSeqNum1, err := flow.DecodeSequenceNumber(seqNum1Value)
				require.NoError(t, err)
				require.Equal(t, key1.SeqNumber, decodedSeqNum1)
			}

			// Test storage used
			var expectedStorageUsed uint64
			// Include account status register in storage used
			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountStatusRegisterID(address),
				accountStatusValue))
			// Include apk_0 register in storage used
			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountPublicKey0RegisterID(address),
				key0Value))
			// Include predefined sequence number 1 register in storage used
			expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
			require.Equal(t, expectedStorageUsed, decodedAccountStatus.StorageUsed())
		}
	})
}

func TestAccounts_GetWithNoKeys(t *testing.T) {
	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)
	address := flow.HexToAddress("01")

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		_, _ = accounts.Get(address)
	})
}

func TestAccounts_GetPublicKey(t *testing.T) {

	t.Run("non-existent key index", func(t *testing.T) {

		address := flow.HexToAddress("01")
		registerId := flow.NewRegisterID(
			address,
			"apk_0")

		for _, value := range [][]byte{{}, nil} {
			txnState := testutils.NewSimpleTransaction(
				snapshot.MapStorageSnapshot{
					registerId: value,
				})
			accounts := environment.NewAccounts(txnState)

			err := accounts.Create(nil, address)
			require.NoError(t, err)

			_, err = accounts.GetAccountPublicKey(address, 0)
			require.True(t, errors.IsAccountPublicKeyNotFoundError(err))
		}
	})
}

func TestAccounts_GetPublicKeyCount(t *testing.T) {

	t.Run("non-existent key count", func(t *testing.T) {

		address := flow.HexToAddress("01")
		registerId := flow.NewRegisterID(
			address,
			"public_key_count")

		for _, value := range [][]byte{{}, nil} {
			txnState := testutils.NewSimpleTransaction(
				snapshot.MapStorageSnapshot{
					registerId: value,
				})
			accounts := environment.NewAccounts(txnState)

			err := accounts.Create(nil, address)
			require.NoError(t, err)

			count, err := accounts.GetAccountPublicKeyCount(address)
			require.NoError(t, err)
			require.Equal(t, uint32(0), count)
		}
	})
}

func TestAccounts_GetPublicKeys(t *testing.T) {

	t.Run("non-existent key count", func(t *testing.T) {

		address := flow.HexToAddress("01")
		registerId := flow.NewRegisterID(
			address,
			"public_key_count")

		for _, value := range [][]byte{{}, nil} {
			txnState := testutils.NewSimpleTransaction(
				snapshot.MapStorageSnapshot{
					registerId: value,
				})

			accounts := environment.NewAccounts(txnState)

			err := accounts.Create(nil, address)
			require.NoError(t, err)

			keys, err := accounts.GetAccountPublicKeys(address)
			require.NoError(t, err)
			require.Empty(t, keys)
		}
	})
}

func TestAccounts_SetContracts(t *testing.T) {

	address := flow.HexToAddress("0x01")

	t.Run("Setting a contract puts it in Contracts", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		a := environment.NewAccounts(txnState)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 1, "There should only be one contract")
		require.Equal(t, contractNames[0], "Dummy")
	})
	t.Run("Setting a contract again, does not add it to contracts", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		a := environment.NewAccounts(txnState)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 1, "There should only be one contract")
		require.Equal(t, contractNames[0], "Dummy")
	})
	t.Run("Setting more contracts always keeps them sorted", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		a := environment.NewAccounts(txnState)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		err = a.SetContract("ZedDummy", address, []byte("non empty string"))
		require.NoError(t, err)

		err = a.SetContract("ADummy", address, []byte("non empty string"))
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 3)
		require.Equal(t, contractNames[0], "ADummy")
		require.Equal(t, contractNames[1], "Dummy")
		require.Equal(t, contractNames[2], "ZedDummy")
	})
	t.Run("Removing a contract does not fail if there is none", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		a := environment.NewAccounts(txnState)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.DeleteContract("Dummy", address)
		require.NoError(t, err)
	})
	t.Run("Removing a contract removes it", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		a := environment.NewAccounts(txnState)
		err := a.Create(nil, address)
		require.NoError(t, err)

		err = a.SetContract("Dummy", address, []byte("non empty string"))
		require.NoError(t, err)

		err = a.DeleteContract("Dummy", address)
		require.NoError(t, err)

		contractNames, err := a.GetContractNames(address)
		require.NoError(t, err)

		require.Len(t, contractNames, 0, "There should be no contract")
	})
}

func TestAccount_StorageUsed(t *testing.T) {
	emptyAccountSize := uint64(54)

	t.Run("Storage used on account creation is deterministic", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize, storageUsed)
	})

	t.Run("Storage used on register set increases", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")
		key := flow.NewRegisterID(address, "some_key")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(42), storageUsed)
	})

	t.Run("Storage used, set twice on same register to same value, stays the same", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")
		key := flow.NewRegisterID(address, "some_key")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(42), storageUsed)
	})

	t.Run("Storage used, set twice on same register to larger value, increases", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")
		key := flow.NewRegisterID(address, "some_key")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(key, createByteArray(13))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(43), storageUsed)
	})

	t.Run("Storage used, set twice on same register to smaller value, decreases", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")
		key := flow.NewRegisterID(address, "some_key")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(key, createByteArray(11))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(41), storageUsed)
	})

	t.Run("Storage used, after register deleted, decreases", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")
		key := flow.NewRegisterID(address, "some_key")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.SetValue(key, createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(key, nil)
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(0), storageUsed)
	})

	t.Run("Storage used on a complex scenario has correct value", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)
		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		key1 := flow.NewRegisterID(address, "some_key")
		err = accounts.SetValue(key1, createByteArray(12))
		require.NoError(t, err)
		err = accounts.SetValue(key1, createByteArray(11))
		require.NoError(t, err)

		key2 := flow.NewRegisterID(address, "some_key2")
		err = accounts.SetValue(key2, createByteArray(22))
		require.NoError(t, err)
		err = accounts.SetValue(key2, createByteArray(23))
		require.NoError(t, err)

		key3 := flow.NewRegisterID(address, "some_key3")
		err = accounts.SetValue(key3, createByteArray(22))
		require.NoError(t, err)
		err = accounts.SetValue(key3, createByteArray(0))
		require.NoError(t, err)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)
		require.Equal(t, emptyAccountSize+uint64(43+52), storageUsed)
	})
}

func TestStatefulAccounts_GenerateAccountLocalID(t *testing.T) {

	// Create 3 accounts
	addressA := flow.HexToAddress("0x01")
	addressB := flow.HexToAddress("0x02")
	addressC := flow.HexToAddress("0x03")
	txnState := testutils.NewSimpleTransaction(nil)
	a := environment.NewAccounts(txnState)
	err := a.Create(nil, addressA)
	require.NoError(t, err)
	err = a.Create(nil, addressB)
	require.NoError(t, err)
	err = a.Create(nil, addressC)
	require.NoError(t, err)

	// setup some state
	_, err = a.GenerateAccountLocalID(addressA)
	require.NoError(t, err)
	_, err = a.GenerateAccountLocalID(addressA)
	require.NoError(t, err)
	_, err = a.GenerateAccountLocalID(addressB)
	require.NoError(t, err)

	// assert

	// addressA
	id, err := a.GenerateAccountLocalID(addressA)
	require.NoError(t, err)
	require.Equal(t, uint64(3), id)

	// addressB
	id, err = a.GenerateAccountLocalID(addressB)
	require.NoError(t, err)
	require.Equal(t, uint64(2), id)

	// addressC
	id, err = a.GenerateAccountLocalID(addressC)
	require.NoError(t, err)
	require.Equal(t, uint64(1), id)
}

func createByteArray(size int) []byte {
	bytes := make([]byte, size)
	for i := range bytes {
		bytes[i] = 255
	}
	return bytes
}

func TestAccounts_AllocateSlabIndex(t *testing.T) {
	txnState := testutils.NewSimpleTransaction(nil)
	accounts := environment.NewAccounts(txnState)
	address := flow.HexToAddress("01")

	err := accounts.Create(nil, address)
	require.NoError(t, err)

	// no register set case
	i, err := accounts.AllocateSlabIndex(address)
	require.NoError(t, err)
	require.Equal(t, i, atree.SlabIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 1}))

	// register already set case
	i, err = accounts.AllocateSlabIndex(address)
	require.NoError(t, err)
	require.Equal(t, i, atree.SlabIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 2}))

	// register update successful
	i, err = accounts.AllocateSlabIndex(address)
	require.NoError(t, err)
	require.Equal(t, i, atree.SlabIndex([8]byte{0, 0, 0, 0, 0, 0, 0, 3}))
}

func TestAccounts_AppendAndGetAccountPublicKey(t *testing.T) {
	t.Run("account with 0 keys", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(0), keyCount)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		_, err = accounts.GetAccountPublicKey(address, 0)
		require.True(t, errors.IsAccountPublicKeyNotFoundError(err))

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		value, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, value)

		// Test account storage used
		expectedStorageUsed := uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			value))
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	t.Run("account with 1 key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.AppendAccountPublicKey(address, key0)
		require.NoError(t, err)

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(1), keyCount)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		retrievedKey0, err := accounts.GetAccountPublicKey(address, 0)
		require.NoError(t, err)
		require.Equal(t, key0, retrievedKey0)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
		}
		registerIDs := snapshot.AllRegisterIDs()
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		// Test storage used
		var expectedStorageUsed uint64
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	t.Run("account with 2 unique keys", func(t *testing.T) {
		for _, hasSequenceNumber := range []bool{true, false} {
			key0 := newAccountPublicKey(t, 1000)
			key0.SeqNumber = 1

			key1 := newAccountPublicKey(t, 1000)
			if hasSequenceNumber {
				key1.SeqNumber = 2
			} else {
				key1.SeqNumber = 0
			}

			txnState := testutils.NewSimpleTransaction(nil)
			accounts := environment.NewAccounts(txnState)

			address := flow.HexToAddress("01")

			err := accounts.Create(nil, address)
			require.NoError(t, err)

			err = accounts.AppendAccountPublicKey(address, key0)
			require.NoError(t, err)

			err = accounts.AppendAccountPublicKey(address, key1)
			require.NoError(t, err)

			keyCount, err := accounts.GetAccountPublicKeyCount(address)
			require.NoError(t, err)
			require.Equal(t, uint32(2), keyCount)

			storageUsed, err := accounts.GetStorageUsed(address)
			require.NoError(t, err)

			retrievedKey0, err := accounts.GetAccountPublicKey(address, 0)
			require.NoError(t, err)
			require.Equal(t, key0, retrievedKey0)

			retrievedKey1, err := accounts.GetAccountPublicKey(address, 1)
			require.NoError(t, err)
			key1.Index = 1
			require.Equal(t, key1, retrievedKey1)

			snapshot, err := txnState.FinalizeMainTransaction()
			require.NoError(t, err)

			// New registers
			expectedNewRegisterIDs := []flow.RegisterID{
				flow.AccountStatusRegisterID(address),
				flow.AccountPublicKey0RegisterID(address),
				flow.AccountBatchPublicKeyRegisterID(address, 0),
			}
			if hasSequenceNumber {
				expectedNewRegisterIDs = append(
					expectedNewRegisterIDs,
					flow.AccountPublicKeySequenceNumberRegisterID(address, 1),
				)
			}
			registerIDs := slices.Collect(maps.Keys(snapshot.WriteSet))
			require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
			require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

			// Test account status
			accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
			require.True(t, exists)
			require.NotEmpty(t, accountStatusValue)

			// Test account public key 0
			key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
			require.True(t, exists)
			require.NotEmpty(t, key0Value)

			// Test account public key 1
			key1Value, exists := snapshot.WriteSet[flow.AccountBatchPublicKeyRegisterID(address, 0)]
			require.True(t, exists)
			require.NotEmpty(t, key1Value)

			// Test storage used
			var expectedStorageUsed uint64
			// Include account status register in storage used
			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountStatusRegisterID(address),
				accountStatusValue))
			// Include apk_0 register in storage used
			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountPublicKey0RegisterID(address),
				key0Value))
			// Include pk_b0 register in storage used
			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountBatchPublicKeyRegisterID(address, 1),
				key1Value))
			// Include predefined sequence number 1 register in storage used
			expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
			require.Equal(t, expectedStorageUsed, storageUsed)
		}
	})

	t.Run("account with 2 duplicate keys", func(t *testing.T) {
		for _, hasSequenceNumber := range []bool{true, false} {
			key0 := newAccountPublicKey(t, 1000)
			key0.SeqNumber = 1

			key1 := key0
			key1.Weight = 1
			if hasSequenceNumber {
				key1.SeqNumber = 2
			} else {
				key1.SeqNumber = 0
			}

			txnState := testutils.NewSimpleTransaction(nil)
			accounts := environment.NewAccounts(txnState)

			address := flow.HexToAddress("01")

			err := accounts.Create(nil, address)
			require.NoError(t, err)

			err = accounts.AppendAccountPublicKey(address, key0)
			require.NoError(t, err)

			err = accounts.AppendAccountPublicKey(address, key1)
			require.NoError(t, err)

			keyCount, err := accounts.GetAccountPublicKeyCount(address)
			require.NoError(t, err)
			require.Equal(t, uint32(2), keyCount)

			storageUsed, err := accounts.GetStorageUsed(address)
			require.NoError(t, err)

			retrievedKey0, err := accounts.GetAccountPublicKey(address, 0)
			require.NoError(t, err)
			require.Equal(t, key0, retrievedKey0)

			retrievedKey1, err := accounts.GetAccountPublicKey(address, 1)
			require.NoError(t, err)
			key1.Index = 1
			require.Equal(t, key1, retrievedKey1)

			snapshot, err := txnState.FinalizeMainTransaction()
			require.NoError(t, err)

			// New registers
			expectedNewRegisterIDs := []flow.RegisterID{
				flow.AccountStatusRegisterID(address),
				flow.AccountPublicKey0RegisterID(address),
			}
			if hasSequenceNumber {
				expectedNewRegisterIDs = append(
					expectedNewRegisterIDs,
					flow.AccountPublicKeySequenceNumberRegisterID(address, 1),
				)
			}
			registerIDs := slices.Collect(maps.Keys(snapshot.WriteSet))
			require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
			require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

			// Test account status
			accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
			require.True(t, exists)
			require.NotEmpty(t, accountStatusValue)

			// Test account public key 0
			key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
			require.True(t, exists)
			require.NotEmpty(t, key0Value)

			// Test storage used
			var expectedStorageUsed uint64
			// Include account status register in storage used
			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountStatusRegisterID(address),
				accountStatusValue))
			// Include apk_0 register in storage used
			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountPublicKey0RegisterID(address),
				key0Value))
			// Include predefined sequence number 1 register in storage used
			expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
			require.Equal(t, expectedStorageUsed, storageUsed)
		}
	})

	t.Run("account with 3 duplicate key with mixed sequence number", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		key0.SeqNumber = 1

		key1 := key0
		key1.Weight = 1
		key1.SeqNumber = 0

		key2 := key1
		key2.Weight = 1000
		key2.SeqNumber = 3

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.AppendAccountPublicKey(address, key0)
		require.NoError(t, err)

		err = accounts.AppendAccountPublicKey(address, key1)
		require.NoError(t, err)

		err = accounts.AppendAccountPublicKey(address, key2)
		require.NoError(t, err)

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(3), keyCount)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		retrievedKey0, err := accounts.GetAccountPublicKey(address, 0)
		require.NoError(t, err)
		require.Equal(t, key0, retrievedKey0)

		retrievedKey1, err := accounts.GetAccountPublicKey(address, 1)
		require.NoError(t, err)
		key1.Index = 1
		require.Equal(t, key1, retrievedKey1)

		retrievedKey2, err := accounts.GetAccountPublicKey(address, 2)
		require.NoError(t, err)
		key2.Index = 2
		require.Equal(t, key2, retrievedKey2)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
			flow.AccountPublicKeySequenceNumberRegisterID(address, 2),
		}
		registerIDs := slices.Collect(maps.Keys(snapshot.WriteSet))
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		// Test storage used
		var expectedStorageUsed uint64
		// Include account status register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		// Include apk_0 register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		// Include predefined sequence number 1 register in storage used
		expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
		// Include predefined sequence number 2 register in storage used
		expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 2)
		require.Equal(t, expectedStorageUsed, storageUsed)
	})
}

func TestAccounts_RevokeAndGetAccountPublicKey(t *testing.T) {
	t.Run("account with 0 keys", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(0), keyCount)

		_, err = accounts.GetAccountPublicKeyRevokedStatus(address, 0)
		require.True(t, errors.IsAccountPublicKeyNotFoundError(err))

		err = accounts.RevokeAccountPublicKey(address, 0)
		require.True(t, errors.IsAccountPublicKeyNotFoundError(err))

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		value, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, value)

		// Test account storage used
		expectedStorageUsed := uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			value))
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	t.Run("account with 1 key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.AppendAccountPublicKey(address, key0)
		require.NoError(t, err)

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(1), keyCount)

		revoked, err := accounts.GetAccountPublicKeyRevokedStatus(address, 0)
		require.NoError(t, err)
		require.False(t, revoked)

		err = accounts.RevokeAccountPublicKey(address, 0)
		require.NoError(t, err)

		revoked, err = accounts.GetAccountPublicKeyRevokedStatus(address, 0)
		require.NoError(t, err)
		require.True(t, revoked)

		retrievedKey, err := accounts.GetAccountPublicKey(address, 0)
		require.NoError(t, err)
		key0.Index = 0
		key0.Revoked = true
		require.Equal(t, key0, retrievedKey)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
		}
		registerIDs := snapshot.AllRegisterIDs()
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		// Test storage used
		var expectedStorageUsed uint64
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	t.Run("account with 2 unique key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		key1 := newAccountPublicKey(t, 1000)
		keys := []flow.AccountPublicKey{key0, key1}

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		for _, key := range keys {
			err = accounts.AppendAccountPublicKey(address, key)
			require.NoError(t, err)
		}

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(len(keys)), keyCount)

		for i, key := range keys {
			revoked, err := accounts.GetAccountPublicKeyRevokedStatus(address, uint32(i))
			require.NoError(t, err)
			require.False(t, revoked)

			err = accounts.RevokeAccountPublicKey(address, uint32(i))
			require.NoError(t, err)

			revoked, err = accounts.GetAccountPublicKeyRevokedStatus(address, uint32(i))
			require.NoError(t, err)
			require.True(t, revoked)

			retrievedKey, err := accounts.GetAccountPublicKey(address, uint32(i))
			require.NoError(t, err)
			key.Index = uint32(i)
			key.Revoked = true
			require.Equal(t, key, retrievedKey)
		}

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
			flow.AccountBatchPublicKeyRegisterID(address, 0),
		}
		registerIDs := slices.Collect(maps.Keys(snapshot.WriteSet))
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		// Test account public key 1
		key1Value, exists := snapshot.WriteSet[flow.AccountBatchPublicKeyRegisterID(address, 0)]
		require.True(t, exists)
		require.NotEmpty(t, key1Value)

		// Test storage used
		var expectedStorageUsed uint64
		// Include account status register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		// Include apk_0 register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		// Include pk_b0 register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountBatchPublicKeyRegisterID(address, 1),
			key1Value))
		// Include predefined sequence number 1 register in storage used
		expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	t.Run("account with 2 duplicate key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		key1 := key0
		key1.Weight = 1
		keys := []flow.AccountPublicKey{key0, key1}

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		for _, key := range keys {
			err = accounts.AppendAccountPublicKey(address, key)
			require.NoError(t, err)
		}

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(len(keys)), keyCount)

		for i, key := range keys {
			revoked, err := accounts.GetAccountPublicKeyRevokedStatus(address, uint32(i))
			require.NoError(t, err)
			require.False(t, revoked)

			err = accounts.RevokeAccountPublicKey(address, uint32(i))
			require.NoError(t, err)

			revoked, err = accounts.GetAccountPublicKeyRevokedStatus(address, uint32(i))
			require.NoError(t, err)
			require.True(t, revoked)

			retrievedKey, err := accounts.GetAccountPublicKey(address, uint32(i))
			require.NoError(t, err)
			key.Index = uint32(i)
			key.Revoked = true
			require.Equal(t, key, retrievedKey)
		}

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
		}
		registerIDs := slices.Collect(maps.Keys(snapshot.WriteSet))
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		// Test storage used
		var expectedStorageUsed uint64
		// Include account status register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		// Include apk_0 register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		// Include predefined sequence number 1 register in storage used
		expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	t.Run("account with 3 duplicate key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		key1 := key0
		key1.Weight = 1
		key2 := key1
		key2.Weight = 1000
		keys := []flow.AccountPublicKey{
			key0, key1, key2,
		}

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		for _, key := range keys {
			err = accounts.AppendAccountPublicKey(address, key)
			require.NoError(t, err)
		}

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(3), keyCount)

		for i, key := range keys {
			revoked, err := accounts.GetAccountPublicKeyRevokedStatus(address, uint32(i))
			require.NoError(t, err)
			require.False(t, revoked)

			err = accounts.RevokeAccountPublicKey(address, uint32(i))
			require.NoError(t, err)

			revoked, err = accounts.GetAccountPublicKeyRevokedStatus(address, uint32(i))
			require.NoError(t, err)
			require.True(t, revoked)

			retrievedKey, err := accounts.GetAccountPublicKey(address, uint32(i))
			require.NoError(t, err)
			key.Index = uint32(i)
			key.Revoked = true
			require.Equal(t, key, retrievedKey)
		}

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
		}
		registerIDs := slices.Collect(maps.Keys(snapshot.WriteSet))
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		// Test storage used
		var expectedStorageUsed uint64
		// Include account status register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		// Include apk_0 register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		// Include predefined sequence number 1 register in storage used
		expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
		// Include predefined sequence number 2 register in storage used
		expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 2)
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	testcases := []struct {
		name              string
		uniqueKeyCount    uint32
		duplicateKeyCount uint32
	}{
		{
			name:              "account with > 20 stored keys (all keys are unique)",
			uniqueKeyCount:    21,
			duplicateKeyCount: 0, // all keys are unique
		},
		{
			name:              "account with > 20 stored key (some keys are duplicate)",
			uniqueKeyCount:    21,
			duplicateKeyCount: 10,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			uniqueKeys := make([]flow.AccountPublicKey, tc.uniqueKeyCount)
			for i := range len(uniqueKeys) {
				weight := rand.Intn(1001)
				uniqueKeys[i] = newAccountPublicKey(t, weight)
			}

			keys := uniqueKeys
			for range tc.duplicateKeyCount {
				insertPos := rand.Intn(len(keys))
				if insertPos == 0 {
					keys = slices.Insert(keys, insertPos, keys[0])
				} else {
					keys = slices.Insert(keys, insertPos, keys[insertPos-1])
				}
			}

			txnState := testutils.NewSimpleTransaction(nil)
			accounts := environment.NewAccounts(txnState)

			address := flow.HexToAddress("01")

			err := accounts.Create(nil, address)
			require.NoError(t, err)

			for _, key := range keys {
				err = accounts.AppendAccountPublicKey(address, key)
				require.NoError(t, err)
			}

			keyCount, err := accounts.GetAccountPublicKeyCount(address)
			require.NoError(t, err)
			require.Equal(t, tc.uniqueKeyCount+tc.duplicateKeyCount, keyCount)

			for i, key := range keys {
				revoked, err := accounts.GetAccountPublicKeyRevokedStatus(address, uint32(i))
				require.NoError(t, err)
				require.False(t, revoked)

				err = accounts.RevokeAccountPublicKey(address, uint32(i))
				require.NoError(t, err)

				revoked, err = accounts.GetAccountPublicKeyRevokedStatus(address, uint32(i))
				require.NoError(t, err)
				require.True(t, revoked)

				retrievedKey, err := accounts.GetAccountPublicKey(address, uint32(i))
				require.NoError(t, err)
				key.Index = uint32(i)
				key.Revoked = true
				require.Equal(t, key, retrievedKey)
			}

			storageUsed, err := accounts.GetStorageUsed(address)
			require.NoError(t, err)

			snapshot, err := txnState.FinalizeMainTransaction()
			require.NoError(t, err)

			accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
			require.True(t, exists)
			require.NotEmpty(t, accountStatusValue)

			// Test storage used
			var expectedStorageUsed uint64

			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountStatusRegisterID(address),
				accountStatusValue))

			if tc.uniqueKeyCount > 0 {
				// Include account public key 0 in storage used
				key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
				require.True(t, exists)
				require.NotEmpty(t, key0Value)

				expectedStorageUsed += uint64(environment.RegisterSize(
					flow.AccountPublicKey0RegisterID(address),
					key0Value))

				// Include pk_b register in storage used
				batchNum := uint32(0)
				for {
					batchValue, exists := snapshot.WriteSet[flow.AccountBatchPublicKeyRegisterID(address, batchNum)]
					if !exists || len(batchValue) == 0 {
						break
					}
					require.True(t, len(batchValue) > 1)

					expectedStorageUsed += uint64(environment.RegisterSize(
						flow.AccountPublicKey0RegisterID(address),
						batchValue))

					batchNum++
				}
			}

			for i := 1; i < int(tc.uniqueKeyCount+tc.duplicateKeyCount); i++ {
				// Include predefined sequence number in storage used
				expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, uint32(i))
			}

			require.Equal(t, expectedStorageUsed, storageUsed)
		})
	}
}

func TestAccounts_IncrementAndGetAccountPublicKeySequenceNumber(t *testing.T) {
	t.Run("account with 0 keys", func(t *testing.T) {
		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(0), keyCount)

		_, err = accounts.GetAccountPublicKeySequenceNumber(address, 0)
		require.True(t, errors.IsAccountPublicKeyNotFoundError(err))

		err = accounts.IncrementAccountPublicKeySequenceNumber(address, 0)
		require.True(t, errors.IsAccountPublicKeyNotFoundError(err))

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		value, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, value)

		// Test account storage used
		expectedStorageUsed := uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			value))
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	t.Run("account with 1 key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		err = accounts.AppendAccountPublicKey(address, key0)
		require.NoError(t, err)

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(1), keyCount)

		seqNum, err := accounts.GetAccountPublicKeySequenceNumber(address, 0)
		require.NoError(t, err)
		require.Equal(t, uint64(0), seqNum)

		err = accounts.IncrementAccountPublicKeySequenceNumber(address, 0)
		require.NoError(t, err)

		seqNum, err = accounts.GetAccountPublicKeySequenceNumber(address, 0)
		require.NoError(t, err)
		require.Equal(t, uint64(1), seqNum)

		retrievedKey, err := accounts.GetAccountPublicKey(address, 0)
		require.NoError(t, err)
		key0.Index = 0
		key0.SeqNumber = 1
		require.Equal(t, key0, retrievedKey)

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
		}
		registerIDs := snapshot.AllRegisterIDs()
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		// Test storage used
		var expectedStorageUsed uint64
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	t.Run("account with 2 unique key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		key1 := newAccountPublicKey(t, 1000)
		keys := []flow.AccountPublicKey{key0, key1}

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		for _, key := range keys {
			err = accounts.AppendAccountPublicKey(address, key)
			require.NoError(t, err)
		}

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(len(keys)), keyCount)

		for i, key := range keys {
			seqNum, err := accounts.GetAccountPublicKeySequenceNumber(address, uint32(i))
			require.NoError(t, err)
			require.Equal(t, uint64(0), seqNum)

			err = accounts.IncrementAccountPublicKeySequenceNumber(address, uint32(i))
			require.NoError(t, err)

			seqNum, err = accounts.GetAccountPublicKeySequenceNumber(address, uint32(i))
			require.NoError(t, err)
			require.Equal(t, uint64(1), seqNum)

			retrievedKey, err := accounts.GetAccountPublicKey(address, uint32(i))
			require.NoError(t, err)
			key.Index = uint32(i)
			key.SeqNumber = 1
			require.Equal(t, key, retrievedKey)
		}

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
			flow.AccountBatchPublicKeyRegisterID(address, 0),
			flow.AccountPublicKeySequenceNumberRegisterID(address, 1),
		}
		registerIDs := slices.Collect(maps.Keys(snapshot.WriteSet))
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		// Test account public key 1
		key1Value, exists := snapshot.WriteSet[flow.AccountBatchPublicKeyRegisterID(address, 0)]
		require.True(t, exists)
		require.NotEmpty(t, key1Value)

		// Test storage used
		var expectedStorageUsed uint64
		// Include account status register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		// Include apk_0 register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		// Include pk_b0 register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountBatchPublicKeyRegisterID(address, 1),
			key1Value))
		// Include predefined sequence number 1 register in storage used
		expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	t.Run("account with 2 duplicate key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		key1 := key0
		key1.Weight = 1
		keys := []flow.AccountPublicKey{key0, key1}

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		for _, key := range keys {
			err = accounts.AppendAccountPublicKey(address, key)
			require.NoError(t, err)
		}

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(len(keys)), keyCount)

		for i, key := range keys {
			seqNum, err := accounts.GetAccountPublicKeySequenceNumber(address, uint32(i))
			require.NoError(t, err)
			require.Equal(t, uint64(0), seqNum)

			err = accounts.IncrementAccountPublicKeySequenceNumber(address, uint32(i))
			require.NoError(t, err)

			seqNum, err = accounts.GetAccountPublicKeySequenceNumber(address, uint32(i))
			require.NoError(t, err)
			require.Equal(t, uint64(1), seqNum)

			retrievedKey, err := accounts.GetAccountPublicKey(address, uint32(i))
			require.NoError(t, err)
			key.Index = uint32(i)
			key.SeqNumber = 1
			require.Equal(t, key, retrievedKey)
		}

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
			flow.AccountPublicKeySequenceNumberRegisterID(address, 1),
		}
		registerIDs := slices.Collect(maps.Keys(snapshot.WriteSet))
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		// Test storage used
		var expectedStorageUsed uint64
		// Include account status register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		// Include apk_0 register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		// Include predefined sequence number 1 register in storage used
		expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	t.Run("account with 3 duplicate key", func(t *testing.T) {
		key0 := newAccountPublicKey(t, 1000)
		key1 := key0
		key1.Weight = 1
		key2 := key1
		key2.Weight = 1000
		keys := []flow.AccountPublicKey{
			key0, key1, key2,
		}

		txnState := testutils.NewSimpleTransaction(nil)
		accounts := environment.NewAccounts(txnState)

		address := flow.HexToAddress("01")

		err := accounts.Create(nil, address)
		require.NoError(t, err)

		for _, key := range keys {
			err = accounts.AppendAccountPublicKey(address, key)
			require.NoError(t, err)
		}

		keyCount, err := accounts.GetAccountPublicKeyCount(address)
		require.NoError(t, err)
		require.Equal(t, uint32(3), keyCount)

		for i, key := range keys {
			seqNum, err := accounts.GetAccountPublicKeySequenceNumber(address, uint32(i))
			require.NoError(t, err)
			require.Equal(t, uint64(0), seqNum)

			err = accounts.IncrementAccountPublicKeySequenceNumber(address, uint32(i))
			require.NoError(t, err)

			seqNum, err = accounts.GetAccountPublicKeySequenceNumber(address, uint32(i))
			require.NoError(t, err)
			require.Equal(t, uint64(1), seqNum)

			retrievedKey, err := accounts.GetAccountPublicKey(address, uint32(i))
			require.NoError(t, err)
			key.Index = uint32(i)
			key.SeqNumber = 1
			require.Equal(t, key, retrievedKey)
		}

		storageUsed, err := accounts.GetStorageUsed(address)
		require.NoError(t, err)

		snapshot, err := txnState.FinalizeMainTransaction()
		require.NoError(t, err)

		// New registers
		expectedNewRegisterIDs := []flow.RegisterID{
			flow.AccountStatusRegisterID(address),
			flow.AccountPublicKey0RegisterID(address),
			flow.AccountPublicKeySequenceNumberRegisterID(address, 1),
			flow.AccountPublicKeySequenceNumberRegisterID(address, 2),
		}
		registerIDs := slices.Collect(maps.Keys(snapshot.WriteSet))
		require.Equal(t, len(expectedNewRegisterIDs), len(registerIDs))
		require.ElementsMatch(t, expectedNewRegisterIDs, registerIDs)

		// Test account status
		accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, accountStatusValue)

		// Test account public key 0
		key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
		require.True(t, exists)
		require.NotEmpty(t, key0Value)

		// Test storage used
		var expectedStorageUsed uint64
		// Include account status register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountStatusRegisterID(address),
			accountStatusValue))
		// Include apk_0 register in storage used
		expectedStorageUsed += uint64(environment.RegisterSize(
			flow.AccountPublicKey0RegisterID(address),
			key0Value))
		// Include predefined sequence number 1 register in storage used
		expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 1)
		// Include predefined sequence number 2 register in storage used
		expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, 2)
		require.Equal(t, expectedStorageUsed, storageUsed)
	})

	testcases := []struct {
		name              string
		storedKeyCount    uint32
		duplicateKeyCount uint32
	}{
		{
			name:              "account with > 20 stored keys (all keys are unique)",
			storedKeyCount:    21,
			duplicateKeyCount: 0,
		},
		{
			name:              "account with > 20 stored key (some keys are duplicate)",
			storedKeyCount:    21,
			duplicateKeyCount: 10,
		},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			uniqueKeys := make([]flow.AccountPublicKey, tc.storedKeyCount)
			for i := range len(uniqueKeys) {
				weight := rand.Intn(1001)
				uniqueKeys[i] = newAccountPublicKey(t, weight)
			}

			keys := uniqueKeys
			for range tc.duplicateKeyCount {
				insertPos := rand.Intn(len(keys))
				if insertPos == 0 {
					keys = slices.Insert(keys, insertPos, keys[0])
				} else {
					keys = slices.Insert(keys, insertPos, keys[insertPos-1])
				}
			}

			txnState := testutils.NewSimpleTransaction(nil)
			accounts := environment.NewAccounts(txnState)

			address := flow.HexToAddress("01")

			err := accounts.Create(nil, address)
			require.NoError(t, err)

			for _, key := range keys {
				err = accounts.AppendAccountPublicKey(address, key)
				require.NoError(t, err)
			}

			keyCount, err := accounts.GetAccountPublicKeyCount(address)
			require.NoError(t, err)
			require.Equal(t, tc.storedKeyCount+tc.duplicateKeyCount, keyCount)

			for i, key := range keys {
				seqNum, err := accounts.GetAccountPublicKeySequenceNumber(address, uint32(i))
				require.NoError(t, err)
				require.Equal(t, uint64(0), seqNum)

				err = accounts.IncrementAccountPublicKeySequenceNumber(address, uint32(i))
				require.NoError(t, err)

				seqNum, err = accounts.GetAccountPublicKeySequenceNumber(address, uint32(i))
				require.NoError(t, err)
				require.Equal(t, uint64(1), seqNum)

				retrievedKey, err := accounts.GetAccountPublicKey(address, uint32(i))
				require.NoError(t, err)
				key.Index = uint32(i)
				key.SeqNumber = 1
				require.Equal(t, key, retrievedKey)
			}

			storageUsed, err := accounts.GetStorageUsed(address)
			require.NoError(t, err)

			snapshot, err := txnState.FinalizeMainTransaction()
			require.NoError(t, err)

			accountStatusValue, exists := snapshot.WriteSet[flow.AccountStatusRegisterID(address)]
			require.True(t, exists)
			require.NotEmpty(t, accountStatusValue)

			// Test storage used
			var expectedStorageUsed uint64

			expectedStorageUsed += uint64(environment.RegisterSize(
				flow.AccountStatusRegisterID(address),
				accountStatusValue))

			if tc.storedKeyCount > 0 {
				// Include account public key 0 in storage used
				key0Value, exists := snapshot.WriteSet[flow.AccountPublicKey0RegisterID(address)]
				require.True(t, exists)
				require.NotEmpty(t, key0Value)

				expectedStorageUsed += uint64(environment.RegisterSize(
					flow.AccountPublicKey0RegisterID(address),
					key0Value))

				// Include pk_b register in storage used
				batchNum := uint32(0)
				for {
					batchValue, exists := snapshot.WriteSet[flow.AccountBatchPublicKeyRegisterID(address, batchNum)]
					if !exists || len(batchValue) == 0 {
						break
					}
					require.True(t, len(batchValue) > 1)

					expectedStorageUsed += uint64(environment.RegisterSize(
						flow.AccountPublicKey0RegisterID(address),
						batchValue))

					batchNum++
				}
			}

			for i := 1; i < int(tc.storedKeyCount+tc.duplicateKeyCount); i++ {
				// Include predefined sequence number in storage used
				expectedStorageUsed += environment.PredefinedSequenceNumberPayloadSize(address, uint32(i))
			}

			require.Equal(t, expectedStorageUsed, storageUsed)
		})
	}
}

func TestRegisterSize(t *testing.T) {
	address := flow.Address{0x01}

	registerID := flow.AccountStatusRegisterID(address)
	registerValue := environment.NewAccountStatus().ToBytes()
	registerSize := environment.RegisterSize(registerID, registerValue)

	payload := ledger.NewPayload(
		convert.RegisterIDToLedgerKey(registerID),
		registerValue)
	payloadSize := payload.Size()

	require.Equal(t, payloadSize, registerSize)
}
