package state_test

import (
	"fmt"
	"math/big"
	"testing"

	gethCommon "github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
)

var emptyRefund = func() uint64 {
	return 0
}

func TestDeltaView(t *testing.T) {
	t.Parallel()

	t.Run("test account exist/creation/suicide functionality", func(t *testing.T) {
		addr1 := testutils.RandomCommonAddress(t)
		addr2 := testutils.RandomCommonAddress(t)
		addr3 := testutils.RandomCommonAddress(t)

		view := state.NewDeltaView(
			&MockedReadOnlyView{
				// we need get refund for parent
				GetRefundFunc: emptyRefund,
				ExistFunc: func(addr gethCommon.Address) (bool, error) {
					switch addr {
					case addr1:
						return true, nil
					case addr2:
						return false, nil
					default:
						return false, fmt.Errorf("some error")
					}
				},
				HasSuicidedFunc: func(gethCommon.Address) bool {
					return false
				},
			})

		// check existing account on the parent
		found, err := view.Exist(addr1)
		require.NoError(t, err)
		require.True(t, found)

		// account doesn't exist on parent
		found, err = view.Exist(addr2)
		require.NoError(t, err)
		require.False(t, found)

		// handling error on the parent
		_, err = view.Exist(addr3)
		require.Error(t, err)

		// create a account at address 2
		err = view.CreateAccount(addr2)
		require.NoError(t, err)

		// now it should be found
		found, err = view.Exist(addr2)
		require.NoError(t, err)
		require.True(t, found)

		// test HasSuicided first
		success := view.HasSuicided(addr1)
		require.False(t, success)

		// set addr1 for deletion
		success, err = view.Suicide(addr1)
		require.NoError(t, err)
		require.True(t, success)

		// check HasSuicided now
		success = view.HasSuicided(addr1)
		require.True(t, success)

		// addr1 should still exist after suicide call
		found, err = view.Exist(addr1)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("test account balance functionality", func(t *testing.T) {
		addr1 := testutils.RandomCommonAddress(t)
		addr1InitBal := big.NewInt(10)
		addr2 := testutils.RandomCommonAddress(t)
		addr2InitBal := big.NewInt(5)
		addr3 := testutils.RandomCommonAddress(t)

		view := state.NewDeltaView(
			&MockedReadOnlyView{
				// we need get refund for parent
				GetRefundFunc: emptyRefund,
				ExistFunc: func(addr gethCommon.Address) (bool, error) {
					switch addr {
					case addr1, addr2:
						return true, nil
					default:
						return false, nil
					}
				},
				HasSuicidedFunc: func(gethCommon.Address) bool {
					return false
				},
				GetBalanceFunc: func(addr gethCommon.Address) (*big.Int, error) {
					switch addr {
					case addr1:
						return addr1InitBal, nil
					case addr2:
						return addr2InitBal, nil
					default:
						return nil, fmt.Errorf("some error")
					}
				},
			})

		// get balance through parent
		bal, err := view.GetBalance(addr1)
		require.NoError(t, err)
		require.Equal(t, addr1InitBal, bal)

		// call suicide on addr
		success, err := view.Suicide(addr1)
		require.NoError(t, err)
		require.True(t, success)

		// now it should return balance of zero
		bal, err = view.GetBalance(addr1)
		require.NoError(t, err)
		require.Equal(t, big.NewInt(0), bal)

		// add balance to addr2
		amount := big.NewInt(7)
		expected := new(big.Int).Add(addr2InitBal, amount)
		err = view.AddBalance(addr2, amount)
		require.NoError(t, err)
		newBal, err := view.GetBalance(addr2)
		require.NoError(t, err)
		require.Equal(t, expected, newBal)

		// sub balance from addr2
		amount = big.NewInt(9)
		expected = new(big.Int).Sub(newBal, amount)
		err = view.SubBalance(addr2, amount)
		require.NoError(t, err)
		bal, err = view.GetBalance(addr2)
		require.NoError(t, err)
		require.Equal(t, expected, bal)

		// negative balance error
		err = view.SubBalance(addr2, big.NewInt(100))
		require.Error(t, err)

		// handling error on the parent
		_, err = view.GetBalance(addr3)
		require.Error(t, err)

		// create a new account at addr3
		err = view.CreateAccount(addr3)
		require.NoError(t, err)

		// now the balance should return 0
		bal, err = view.GetBalance(addr3)
		require.NoError(t, err)
		require.Equal(t, big.NewInt(0), bal)
	})

	t.Run("test nonce functionality", func(t *testing.T) {
		addr1 := testutils.RandomCommonAddress(t)
		addr1InitNonce := uint64(1)
		addr2 := testutils.RandomCommonAddress(t)

		view := state.NewDeltaView(
			&MockedReadOnlyView{
				// we need get refund for parent
				GetRefundFunc: emptyRefund,
				ExistFunc: func(addr gethCommon.Address) (bool, error) {
					switch addr {
					case addr1:
						return true, nil
					default:
						return false, nil
					}
				},

				GetNonceFunc: func(addr gethCommon.Address) (uint64, error) {
					switch addr {
					case addr1:
						return addr1InitNonce, nil
					default:
						return 0, fmt.Errorf("some error")
					}
				},
			})

		// get nonce through parent
		nonce, err := view.GetNonce(addr1)
		require.NoError(t, err)
		require.Equal(t, addr1InitNonce, nonce)

		// set nonce
		new := uint64(100)
		err = view.SetNonce(addr1, new)
		require.NoError(t, err)
		nonce, err = view.GetNonce(addr1)
		require.NoError(t, err)
		require.Equal(t, new, nonce)

		// handling error on the parent
		_, err = view.GetNonce(addr2)
		require.Error(t, err)

		// create a new account at addr2
		err = view.CreateAccount(addr2)
		require.NoError(t, err)

		// now the nonce should return 0
		nonce, err = view.GetNonce(addr2)
		require.NoError(t, err)
		require.Equal(t, uint64(0), nonce)
	})

	// test for dirty addresses (different operations)

}

type MockedReadOnlyView struct {
	ExistFunc               func(gethCommon.Address) (bool, error)
	HasSuicidedFunc         func(gethCommon.Address) bool
	GetBalanceFunc          func(gethCommon.Address) (*big.Int, error)
	GetNonceFunc            func(gethCommon.Address) (uint64, error)
	GetCodeFunc             func(gethCommon.Address) ([]byte, error)
	GetCodeHashFunc         func(gethCommon.Address) (gethCommon.Hash, error)
	GetCodeSizeFunc         func(gethCommon.Address) (int, error)
	GetStateFunc            func(types.SlotAddress) (gethCommon.Hash, error)
	GetTransientStateFunc   func(types.SlotAddress) gethCommon.Hash
	GetRefundFunc           func() uint64
	AddressInAccessListFunc func(gethCommon.Address) bool
	SlotInAccessListFunc    func(types.SlotAddress) (addressOk bool, slotOk bool)
}

var _ types.ReadOnlyView = &MockedReadOnlyView{}

func (v *MockedReadOnlyView) Exist(addr gethCommon.Address) (bool, error) {
	if v.ExistFunc == nil {
		panic("Exist is not set in this mocked view")
	}
	return v.ExistFunc(addr)
}

func (v *MockedReadOnlyView) HasSuicided(addr gethCommon.Address) bool {
	if v.HasSuicidedFunc == nil {
		panic("HasSuicided is not set in this mocked view")
	}
	return v.HasSuicidedFunc(addr)
}

func (v *MockedReadOnlyView) GetBalance(addr gethCommon.Address) (*big.Int, error) {
	if v.GetBalanceFunc == nil {
		panic("GetBalance is not set in this mocked view")
	}
	return v.GetBalanceFunc(addr)
}

func (v *MockedReadOnlyView) GetNonce(addr gethCommon.Address) (uint64, error) {
	if v.GetNonceFunc == nil {
		panic("GetNonce is not set in this mocked view")
	}
	return v.GetNonceFunc(addr)
}

func (v *MockedReadOnlyView) GetCode(addr gethCommon.Address) ([]byte, error) {
	if v.GetCodeFunc == nil {
		panic("GetCode is not set in this mocked view")
	}
	return v.GetCodeFunc(addr)
}

func (v *MockedReadOnlyView) GetCodeHash(addr gethCommon.Address) (gethCommon.Hash, error) {
	if v.GetCodeHashFunc == nil {
		panic("GetCodeHash is not set in this mocked view")
	}
	return v.GetCodeHashFunc(addr)
}

func (v *MockedReadOnlyView) GetCodeSize(addr gethCommon.Address) (int, error) {
	if v.GetCodeSizeFunc == nil {
		panic("GetCodeSize is not set in this mocked view")
	}
	return v.GetCodeSizeFunc(addr)
}

func (v *MockedReadOnlyView) GetState(slot types.SlotAddress) (gethCommon.Hash, error) {
	if v.GetStateFunc == nil {
		panic("GetState is not set in this mocked view")
	}
	return v.GetStateFunc(slot)
}

func (v *MockedReadOnlyView) GetTransientState(slot types.SlotAddress) gethCommon.Hash {
	if v.GetTransientStateFunc == nil {
		panic("GetTransientState is not set in this mocked view")
	}
	return v.GetTransientStateFunc(slot)
}

func (v *MockedReadOnlyView) GetRefund() uint64 {
	if v.GetRefundFunc == nil {
		panic("GetRefund is not set in this mocked view")
	}
	return v.GetRefundFunc()
}

func (v *MockedReadOnlyView) AddressInAccessList(addr gethCommon.Address) bool {
	if v.AddressInAccessListFunc == nil {
		panic("AddressInAccessList is not set in this mocked view")
	}
	return v.AddressInAccessListFunc(addr)
}

func (v *MockedReadOnlyView) SlotInAccessList(slot types.SlotAddress) (addressOk bool, slotOk bool) {
	if v.SlotInAccessListFunc == nil {
		panic("SlotInAccessList is not set in this mocked view")
	}
	return v.SlotInAccessListFunc(slot)
}
