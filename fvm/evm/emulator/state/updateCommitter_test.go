package state_test

import (
	"testing"

	"github.com/holiman/uint256"
	"github.com/onflow/crypto/hash"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/stretchr/testify/require"
)

func TestChangeCommitter(t *testing.T) {

	addr := testutils.RandomAddress(t).ToCommon()
	balance := uint256.NewInt(200)
	nonce := uint64(1)
	nonceBytes := []byte{0, 0, 0, 0, 0, 0, 0, 1}
	randomHash := testutils.RandomCommonHash(t)
	key := testutils.RandomCommonHash(t)
	value := testutils.RandomCommonHash(t)

	t.Run("test create account", func(t *testing.T) {
		dc := state.NewUpdateCommitter()
		dc.CreateAccount(addr, balance, nonce, randomHash)

		hasher := hash.NewSHA3_256()

		input := []byte{byte(state.AccountCreationOpCode)}
		input = append(input, addr.Bytes()...)
		encodedBalance := balance.Bytes32()
		input = append(input, encodedBalance[:]...)
		input = append(input, nonceBytes...)
		input = append(input, randomHash.Bytes()...)

		n, err := hasher.Write(input)
		require.NoError(t, err)
		require.Equal(t, 93, n)

		expectedCommit := hasher.SumHash()
		commit := dc.Commitment()
		require.Equal(t, expectedCommit, commit)
	})

	t.Run("test update account", func(t *testing.T) {
		dc := state.NewUpdateCommitter()
		dc.UpdateAccount(addr, balance, nonce, randomHash)

		hasher := hash.NewSHA3_256()
		input := []byte{byte(state.AccountUpdateOpCode)}
		input = append(input, addr.Bytes()...)
		encodedBalance := balance.Bytes32()
		input = append(input, encodedBalance[:]...)
		input = append(input, nonceBytes...)
		input = append(input, randomHash.Bytes()...)

		n, err := hasher.Write(input)
		require.NoError(t, err)
		require.Equal(t, 93, n)

		expectedCommit := hasher.SumHash()
		commit := dc.Commitment()
		require.Equal(t, expectedCommit, commit)
	})

	t.Run("test delete account", func(t *testing.T) {
		dc := state.NewUpdateCommitter()
		dc.DeleteAccount(addr)

		hasher := hash.NewSHA3_256()
		input := []byte{byte(state.AccountDeletionOpCode)}
		input = append(input, addr.Bytes()...)

		n, err := hasher.Write(input)
		require.NoError(t, err)
		require.Equal(t, 21, n)

		expectedCommit := hasher.SumHash()
		commit := dc.Commitment()
		require.Equal(t, expectedCommit, commit)
	})

	t.Run("test update slot", func(t *testing.T) {
		dc := state.NewUpdateCommitter()
		dc.UpdateSlot(addr, key, value)

		hasher := hash.NewSHA3_256()

		input := []byte{byte(state.SlotUpdateOpCode)}
		input = append(input, addr.Bytes()...)
		input = append(input, key[:]...)
		input = append(input, value[:]...)

		n, err := hasher.Write(input)
		require.NoError(t, err)
		require.Equal(t, 85, n)

		expectedCommit := hasher.SumHash()
		commit := dc.Commitment()
		require.Equal(t, expectedCommit, commit)
	})
}

func BenchmarkDeltaCommitter(b *testing.B) {
	addr := testutils.RandomAddress(b)
	balance := uint256.NewInt(200)
	nonce := uint64(100)
	randomHash := testutils.RandomCommonHash(b)
	dc := state.NewUpdateCommitter()

	numberOfAccountUpdates := 10
	for i := 0; i < numberOfAccountUpdates; i++ {
		dc.UpdateAccount(addr.ToCommon(), balance, nonce, randomHash)
	}

	numberOfSlotUpdates := 10
	for i := 0; i < numberOfSlotUpdates; i++ {
		dc.UpdateSlot(addr.ToCommon(), randomHash, randomHash)
	}
	dc.Commitment()
}
