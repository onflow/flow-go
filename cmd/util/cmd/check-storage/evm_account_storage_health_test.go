package check_storage

import (
	"strconv"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"golang.org/x/exp/maps"

	"github.com/onflow/atree"
	"github.com/onflow/cadence/runtime"
	"github.com/onflow/cadence/runtime/common"
	"github.com/onflow/cadence/runtime/interpreter"
	gethCommon "github.com/onflow/go-ethereum/common"

	"github.com/onflow/flow-go/cmd/util/ledger/util"
	"github.com/onflow/flow-go/cmd/util/ledger/util/registers"
	"github.com/onflow/flow-go/fvm/evm/emulator/state"
	"github.com/onflow/flow-go/fvm/evm/testutils"
	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/ledger"
	"github.com/onflow/flow-go/ledger/common/convert"
	"github.com/onflow/flow-go/model/flow"
)

func TestEVMAccountStorageHealth(t *testing.T) {
	address := common.Address{1}

	t.Run("has storage slot", func(t *testing.T) {
		led := createPayloadLedger()

		createEVMStorage(t, led, address)

		createCadenceStorage(t, led, address)

		payloads := maps.Values(led.Payloads)

		accountRegisters, err := registers.NewAccountRegistersFromPayloads(string(address[:]), payloads)
		require.NoError(t, err)

		issues := checkEVMAccountStorageHealth(
			address,
			accountRegisters,
		)
		require.Equal(t, 0, len(issues))
	})

	t.Run("unreferenced slabs", func(t *testing.T) {
		led := createPayloadLedger()

		createEVMStorage(t, led, address)

		createCadenceStorage(t, led, address)

		payloads := maps.Values(led.Payloads)

		// Add unreferenced slabs
		slabIndex, err := led.AllocateSlabIndexFunc(address[:])
		require.NoError(t, err)

		registerID := flow.NewRegisterID(
			flow.BytesToAddress(address[:]),
			string(atree.SlabIndexToLedgerKey(slabIndex)))

		unreferencedPayload := ledger.NewPayload(
			convert.RegisterIDToLedgerKey(registerID),
			ledger.Value([]byte{1}))

		payloads = append(payloads, unreferencedPayload)

		accountRegisters, err := registers.NewAccountRegistersFromPayloads(string(address[:]), payloads)
		require.NoError(t, err)

		issues := checkEVMAccountStorageHealth(
			address,
			accountRegisters,
		)
		require.Equal(t, 1, len(issues))
		require.Equal(t, storageErrorKindString[evmAtreeStorageErrorKind], issues[0].Kind)
		require.Contains(t, issues[0].Msg, "unreferenced atree registers")
	})
}

func createEVMStorage(t *testing.T, ledger atree.Ledger, address common.Address) {
	view, err := state.NewBaseView(ledger, flow.BytesToAddress(address[:]))
	require.NoError(t, err)

	// Create an account without storage slot
	addr1 := testutils.RandomCommonAddress(t)

	err = view.CreateAccount(addr1, uint256.NewInt(1), 2, []byte("ABC"), gethCommon.Hash{3, 4, 5})
	require.NoError(t, err)

	// Create an account with storage slot
	addr2 := testutils.RandomCommonAddress(t)

	err = view.CreateAccount(addr2, uint256.NewInt(6), 7, []byte("DEF"), gethCommon.Hash{8, 9, 19})
	require.NoError(t, err)

	slot := types.SlotAddress{
		Address: addr2,
		Key:     testutils.RandomCommonHash(t),
	}

	err = view.UpdateSlot(slot, testutils.RandomCommonHash(t))
	require.NoError(t, err)

	err = view.Commit()
	require.NoError(t, err)
}

func createCadenceStorage(t *testing.T, ledger atree.Ledger, address common.Address) {
	storage := runtime.NewStorage(ledger, nil)

	inter, err := interpreter.NewInterpreter(
		nil,
		nil,
		&interpreter.Config{
			Storage: storage,
		},
	)
	require.NoError(t, err)

	// Create storage and public domains
	for _, domain := range []string{"storage", "public"} {
		storageDomain := storage.GetStorageMap(address, domain, true)

		// Create large domain map so there are more than one atree registers under the hood.
		for i := 0; i < 100; i++ {
			key := interpreter.StringStorageMapKey(domain + "_key_" + strconv.Itoa(i))
			value := interpreter.NewUnmeteredStringValue(domain + "_value_" + strconv.Itoa(i))
			storageDomain.SetValue(inter, key, value)
		}
	}

	// Commit domain data
	err = storage.Commit(inter, false)
	require.NoError(t, err)
}

func createPayloadLedger() *util.PayloadsLedger {
	nextSlabIndex := atree.SlabIndex{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x1}

	return &util.PayloadsLedger{
		Payloads: make(map[flow.RegisterID]*ledger.Payload),
		AllocateSlabIndexFunc: func([]byte) (atree.SlabIndex, error) {
			var slabIndex atree.SlabIndex
			slabIndex, nextSlabIndex = nextSlabIndex, nextSlabIndex.Next()
			return slabIndex, nil
		},
	}
}
