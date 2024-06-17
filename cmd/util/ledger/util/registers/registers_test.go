package registers

import (
	"testing"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func newPayload(address flow.Address, key string, value []byte) *ledger.Payload {
	ledgerKey := convert.RegisterIDToLedgerKey(flow.NewRegisterID(address, key))
	return ledger.NewPayload(ledgerKey, value)
}

func TestNewByAccountFromPayloads(t *testing.T) {
	t.Parallel()

	payloads := []*ledger.Payload{
		newPayload(flow.Address{2}, "d", []byte{5}),
		newPayload(flow.Address{1}, "a", []byte{4}),
		newPayload(flow.Address{2}, "c", []byte{6}),
		newPayload(flow.Address{1}, "b", []byte{3}),
	}

	byAccount, err := NewByAccountFromPayloads(payloads)
	require.NoError(t, err)

	assert.Equal(t, 4, byAccount.Count())
	assert.Equal(t, 2, byAccount.AccountCount())

	value, err := byAccount.Get("\x01\x00\x00\x00\x00\x00\x00\x00", "a")
	require.NoError(t, err)
	assert.Equal(t, []byte{4}, value)

	value, err = byAccount.Get("\x01\x00\x00\x00\x00\x00\x00\x00", "b")
	require.NoError(t, err)
	assert.Equal(t, []byte{3}, value)

	value, err = byAccount.Get("\x02\x00\x00\x00\x00\x00\x00\x00", "c")
	require.NoError(t, err)
	assert.Equal(t, []byte{6}, value)

	value, err = byAccount.Get("\x02\x00\x00\x00\x00\x00\x00\x00", "d")
	require.NoError(t, err)
	assert.Equal(t, []byte{5}, value)
}

func TestByAccount_ForEachAccount(t *testing.T) {
	t.Parallel()

	payloads := []*ledger.Payload{
		newPayload(flow.Address{2}, "d", []byte{5}),
		newPayload(flow.Address{1}, "a", []byte{4}),
		newPayload(flow.Address{2}, "c", []byte{6}),
		newPayload(flow.Address{1}, "b", []byte{3}),
		newPayload(flow.Address{2}, "e", []byte{7}),
	}

	byAccount, err := NewByAccountFromPayloads(payloads)
	require.NoError(t, err)

	var seen1, seen2 bool

	_ = byAccount.ForEachAccount(func(accountRegisters *AccountRegisters) error {
		owner := accountRegisters.Owner()
		switch owner {
		case "\x01\x00\x00\x00\x00\x00\x00\x00":
			require.False(t, seen1)
			seen1 = true
			assert.Equal(t, 2, accountRegisters.Count())

		case "\x02\x00\x00\x00\x00\x00\x00\x00":
			require.False(t, seen2)
			seen2 = true
			assert.Equal(t, 3, accountRegisters.Count())

		default:
			t.Fatalf("unexpected owner: %v", owner)
		}

		return nil
	})

	assert.True(t, seen1)
	assert.True(t, seen2)
}

func TestByAccount_ForEach(t *testing.T) {
	t.Parallel()

	payloads := []*ledger.Payload{
		newPayload(flow.Address{2}, "d", []byte{5}),
		newPayload(flow.Address{1}, "a", []byte{4}),
		newPayload(flow.Address{2}, "c", []byte{6}),
		newPayload(flow.Address{1}, "b", []byte{3}),
		newPayload(flow.Address{2}, "e", []byte{7}),
	}

	byAccount, err := NewByAccountFromPayloads(payloads)
	require.NoError(t, err)

	type register struct {
		owner string
		key   string
		value []byte
	}

	var registers []register

	_ = byAccount.ForEach(func(owner string, key string, value []byte) error {
		registers = append(registers, register{
			owner: owner,
			key:   key,
			value: value,
		})

		return nil
	})

	require.ElementsMatch(t,
		[]register{
			{"\x02\x00\x00\x00\x00\x00\x00\x00", "d", []byte{5}},
			{"\x01\x00\x00\x00\x00\x00\x00\x00", "a", []byte{4}},
			{"\x02\x00\x00\x00\x00\x00\x00\x00", "c", []byte{6}},
			{"\x01\x00\x00\x00\x00\x00\x00\x00", "b", []byte{3}},
			{"\x02\x00\x00\x00\x00\x00\x00\x00", "e", []byte{7}},
		},
		registers,
	)
}

func TestByAccount_Set(t *testing.T) {
	t.Parallel()

	byAccount := NewByAccount()
	assert.Equal(t, 0, byAccount.AccountCount())
	assert.Equal(t, 0, byAccount.Count())

	// 0x1, a = 5

	err := byAccount.Set("\x01\x00\x00\x00\x00\x00\x00\x00", "a", []byte{5})
	require.NoError(t, err)

	assert.Equal(t, 1, byAccount.AccountCount())
	assert.Equal(t, 1, byAccount.Count())

	value, err := byAccount.Get("\x01\x00\x00\x00\x00\x00\x00\x00", "a")
	require.NoError(t, err)
	assert.Equal(t, []byte{5}, value)

	// 0x1, b = 6

	err = byAccount.Set("\x01\x00\x00\x00\x00\x00\x00\x00", "b", []byte{6})
	require.NoError(t, err)

	assert.Equal(t, 1, byAccount.AccountCount())
	assert.Equal(t, 2, byAccount.Count())

	value, err = byAccount.Get("\x01\x00\x00\x00\x00\x00\x00\x00", "a")
	require.NoError(t, err)
	assert.Equal(t, []byte{5}, value)

	value, err = byAccount.Get("\x01\x00\x00\x00\x00\x00\x00\x00", "b")
	require.NoError(t, err)
	assert.Equal(t, []byte{6}, value)

	// 0x2, c = 7

	err = byAccount.Set("\x02\x00\x00\x00\x00\x00\x00\x00", "c", []byte{7})
	require.NoError(t, err)

	assert.Equal(t, 2, byAccount.AccountCount())
	assert.Equal(t, 3, byAccount.Count())

	value, err = byAccount.Get("\x01\x00\x00\x00\x00\x00\x00\x00", "a")
	require.NoError(t, err)
	assert.Equal(t, []byte{5}, value)

	value, err = byAccount.Get("\x01\x00\x00\x00\x00\x00\x00\x00", "b")
	require.NoError(t, err)
	assert.Equal(t, []byte{6}, value)

	value, err = byAccount.Get("\x02\x00\x00\x00\x00\x00\x00\x00", "c")
	require.NoError(t, err)
	assert.Equal(t, []byte{7}, value)
}

func TestByAccount_DestructIntoPayloads(t *testing.T) {
	t.Parallel()

	payloads := []*ledger.Payload{
		newPayload(flow.Address{2}, "d", []byte{5}),
		newPayload(flow.Address{1}, "a", []byte{4}),
		newPayload(flow.Address{2}, "c", []byte{6}),
		newPayload(flow.Address{1}, "b", []byte{3}),
	}

	byAccount, err := NewByAccountFromPayloads(payloads)
	require.NoError(t, err)

	newPayloads := byAccount.DestructIntoPayloads(2)

	assert.Equal(t, 0, byAccount.AccountCount())
	assert.Equal(t, 0, byAccount.Count())
	assert.ElementsMatch(t, payloads, newPayloads)
}

func TestByAccount_SetAccountRegisters(t *testing.T) {
	t.Parallel()

	byAccount := NewByAccount()

	accountRegisters1 := NewAccountRegisters("\x01\x00\x00\x00\x00\x00\x00\x00")
	oldAccountRegisters := byAccount.SetAccountRegisters(accountRegisters1)
	require.Nil(t, oldAccountRegisters)

	accountRegisters2 := NewAccountRegisters("\x01\x00\x00\x00\x00\x00\x00\x00")
	oldAccountRegisters = byAccount.SetAccountRegisters(accountRegisters2)
	require.Same(t, accountRegisters1, oldAccountRegisters)
}

func TestAccountRegisters_Set(t *testing.T) {
	t.Parallel()

	const owner1 = "\x01\x00\x00\x00\x00\x00\x00\x00"
	const owner2 = "\x02\x00\x00\x00\x00\x00\x00\x00"
	accountRegisters := NewAccountRegisters(owner1)

	err := accountRegisters.Set(owner1, "a", []byte{5})
	require.NoError(t, err)

	err = accountRegisters.Set(owner2, "a", []byte{5})
	require.ErrorContains(t, err, "owner mismatch")
}

func TestAccountRegisters_Get(t *testing.T) {
	t.Parallel()

	const owner1 = "\x01\x00\x00\x00\x00\x00\x00\x00"
	const owner2 = "\x02\x00\x00\x00\x00\x00\x00\x00"
	accountRegisters := NewAccountRegisters(owner1)

	err := accountRegisters.Set(owner1, "a", []byte{5})
	require.NoError(t, err)

	value, err := accountRegisters.Get(owner1, "a")
	require.NoError(t, err)
	assert.Equal(t, []byte{5}, value)

	_, err = accountRegisters.Get(owner2, "a")
	require.ErrorContains(t, err, "owner mismatch")
}

func TestNewAccountRegistersFromPayloads(t *testing.T) {
	t.Parallel()

	const owner = "\x01\x00\x00\x00\x00\x00\x00\x00"

	payloads := []*ledger.Payload{
		newPayload(flow.Address{1}, "d", []byte{5}),
		newPayload(flow.Address{1}, "a", []byte{4}),
		newPayload(flow.Address{1}, "c", []byte{6}),
		newPayload(flow.Address{1}, "b", []byte{3}),
	}

	accountRegisters, err := NewAccountRegistersFromPayloads(owner, payloads)
	require.NoError(t, err)

	assert.Equal(t, 4, accountRegisters.Count())

	value, err := accountRegisters.Get(owner, "a")
	require.NoError(t, err)
	assert.Equal(t, []byte{4}, value)

	value, err = accountRegisters.Get(owner, "b")
	require.NoError(t, err)
	assert.Equal(t, []byte{3}, value)

	value, err = accountRegisters.Get(owner, "c")
	require.NoError(t, err)
	assert.Equal(t, []byte{6}, value)

	value, err = accountRegisters.Get(owner, "d")
	require.NoError(t, err)
	assert.Equal(t, []byte{5}, value)
}

func TestAccountRegisters_ForEach(t *testing.T) {
	t.Parallel()

	const owner = "\x01\x00\x00\x00\x00\x00\x00\x00"

	payloads := []*ledger.Payload{
		newPayload(flow.Address{1}, "d", []byte{5}),
		newPayload(flow.Address{1}, "a", []byte{4}),
		newPayload(flow.Address{1}, "c", []byte{6}),
		newPayload(flow.Address{1}, "b", []byte{3}),
		newPayload(flow.Address{1}, "e", []byte{7}),
	}

	accountRegisters, err := NewAccountRegistersFromPayloads(owner, payloads)
	require.NoError(t, err)

	type register struct {
		owner string
		key   string
		value []byte
	}

	var registers []register

	_ = accountRegisters.ForEach(func(owner string, key string, value []byte) error {
		registers = append(registers, register{
			owner: owner,
			key:   key,
			value: value,
		})

		return nil
	})

	require.ElementsMatch(t,
		[]register{
			{"\x01\x00\x00\x00\x00\x00\x00\x00", "d", []byte{5}},
			{"\x01\x00\x00\x00\x00\x00\x00\x00", "a", []byte{4}},
			{"\x01\x00\x00\x00\x00\x00\x00\x00", "c", []byte{6}},
			{"\x01\x00\x00\x00\x00\x00\x00\x00", "b", []byte{3}},
			{"\x01\x00\x00\x00\x00\x00\x00\x00", "e", []byte{7}},
		},
		registers,
	)
}

func TestAccountRegisters_Payloads(t *testing.T) {
	t.Parallel()

	const owner = "\x01\x00\x00\x00\x00\x00\x00\x00"

	payloads := []*ledger.Payload{
		newPayload(flow.Address{1}, "d", []byte{5}),
		newPayload(flow.Address{1}, "a", []byte{4}),
		newPayload(flow.Address{1}, "c", []byte{6}),
		newPayload(flow.Address{1}, "b", []byte{3}),
		newPayload(flow.Address{1}, "e", []byte{7}),
	}

	accountRegisters, err := NewAccountRegistersFromPayloads(owner, payloads)
	require.NoError(t, err)

	newPayloads := accountRegisters.Payloads()
	require.ElementsMatch(t, payloads, newPayloads)
}

func TestAccountRegisters_Merge(t *testing.T) {
	t.Parallel()

	const owner = "\x01\x00\x00\x00\x00\x00\x00\x00"

	payloads1 := []*ledger.Payload{
		newPayload(flow.Address{1}, "d", []byte{5}),
		newPayload(flow.Address{1}, "a", []byte{4}),
		newPayload(flow.Address{1}, "c", []byte{6}),
	}

	accountRegisters1, err := NewAccountRegistersFromPayloads(owner, payloads1)
	require.NoError(t, err)

	payloads2 := []*ledger.Payload{
		newPayload(flow.Address{1}, "b", []byte{3}),
		newPayload(flow.Address{1}, "e", []byte{7}),
	}

	accountRegisters2, err := NewAccountRegistersFromPayloads(owner, payloads2)
	require.NoError(t, err)

	err = accountRegisters1.Merge(accountRegisters2)
	require.NoError(t, err)

	allPayloads := append(payloads1, payloads2...)

	newPayloads := accountRegisters1.Payloads()
	require.ElementsMatch(t, allPayloads, newPayloads)
}

func TestApplyChanges_ByAccount(t *testing.T) {
	t.Parallel()

	const owner1 = "\x01\x00\x00\x00\x00\x00\x00\x00"
	const owner2 = "\x02\x00\x00\x00\x00\x00\x00\x00"

	payloads := []*ledger.Payload{
		newPayload(flow.Address{1}, "d", []byte{5}),
		newPayload(flow.Address{2}, "a", []byte{4}),
		newPayload(flow.Address{1}, "c", []byte{6}),
		newPayload(flow.Address{2}, "b", []byte{3}),
		newPayload(flow.Address{1}, "e", []byte{7}),
	}

	byAccount, err := NewByAccountFromPayloads(payloads)
	require.NoError(t, err)

	changes := map[flow.RegisterID]flow.RegisterValue{
		{Owner: owner1, Key: "c"}: {8},
		{Owner: owner1, Key: "f"}: {9},
		{Owner: owner1, Key: "a"}: {10},
		{Owner: owner2, Key: "a"}: {11},
	}

	err = ApplyChanges(
		byAccount,
		changes,
		map[flow.Address]struct{}{
			{1}: {},
			{2}: {},
		},
		zerolog.Nop(),
	)
	require.NoError(t, err)

	newPayloads := byAccount.DestructIntoPayloads(2)

	require.ElementsMatch(t,
		[]*ledger.Payload{
			newPayload(flow.Address{1}, "d", []byte{5}),
			newPayload(flow.Address{2}, "a", []byte{11}),
			newPayload(flow.Address{1}, "c", []byte{8}),
			newPayload(flow.Address{2}, "b", []byte{3}),
			newPayload(flow.Address{1}, "e", []byte{7}),
			newPayload(flow.Address{1}, "f", []byte{9}),
			newPayload(flow.Address{1}, "a", []byte{10}),
		},
		newPayloads,
	)
}

func TestApplyChanges_AccountRegisters(t *testing.T) {
	t.Parallel()

	const owner = "\x01\x00\x00\x00\x00\x00\x00\x00"

	payloads := []*ledger.Payload{
		newPayload(flow.Address{1}, "d", []byte{5}),
		newPayload(flow.Address{1}, "a", []byte{4}),
		newPayload(flow.Address{1}, "c", []byte{6}),
		newPayload(flow.Address{1}, "b", []byte{3}),
		newPayload(flow.Address{1}, "e", []byte{7}),
	}

	accountRegisters, err := NewAccountRegistersFromPayloads(owner, payloads)
	require.NoError(t, err)

	changes := map[flow.RegisterID]flow.RegisterValue{
		{Owner: owner, Key: "c"}: {8},
		{Owner: owner, Key: "f"}: {9},
		{Owner: owner, Key: "a"}: {10},
	}

	err = ApplyChanges(
		accountRegisters,
		changes,
		map[flow.Address]struct{}{
			{1}: {},
		},
		zerolog.Nop(),
	)
	require.NoError(t, err)

	newPayloads := accountRegisters.Payloads()

	require.ElementsMatch(t,
		[]*ledger.Payload{
			newPayload(flow.Address{1}, "d", []byte{5}),
			newPayload(flow.Address{1}, "a", []byte{10}),
			newPayload(flow.Address{1}, "c", []byte{8}),
			newPayload(flow.Address{1}, "b", []byte{3}),
			newPayload(flow.Address{1}, "e", []byte{7}),
			newPayload(flow.Address{1}, "f", []byte{9}),
		},
		newPayloads,
	)
}
