package state

import (
	"github.com/onflow/atree"
	gethCommon "github.com/onflow/go-ethereum/common"

	"github.com/onflow/flow-go/fvm/evm/types"
	"github.com/onflow/flow-go/model/flow"
)

func Extract(
	ledger atree.Ledger,
	root flow.Address,
	baseView *BaseView,
) (*EVMState, error) {

	accounts := make(map[gethCommon.Address]*Account, 0)

	itr, err := baseView.AccountIterator()

	if err != nil {
		return nil, err
	}
	// make a list of accounts with storage
	addrWithSlots := make([]gethCommon.Address, 0)
	for {
		// TODO: we can optimize by returning the encoded value
		acc, err := itr.Next()
		if err != nil {
			return nil, err
		}
		if acc == nil {
			break
		}
		if acc.HasStoredValues() {
			addrWithSlots = append(addrWithSlots, acc.Address)
		}
		accounts[acc.Address] = acc
	}

	codes := make(map[gethCommon.Hash]*CodeInContext, 0)
	codeItr, err := baseView.CodeIterator()
	if err != nil {
		return nil, err
	}
	for {
		cic, err := codeItr.Next()
		if err != nil {
			return nil, err
		}
		if cic == nil {
			break
		}
		codes[cic.Hash] = cic
	}

	// account address -> key -> value
	slots := make(map[gethCommon.Address]map[gethCommon.Hash]*types.SlotEntry)

	for _, addr := range addrWithSlots {
		slots[addr] = make(map[gethCommon.Hash]*types.SlotEntry)
		slotItr, err := baseView.AccountStorageIterator(addr)
		if err != nil {
			return nil, err
		}
		for {
			slot, err := slotItr.Next()
			if err != nil {
				return nil, err
			}
			if slot == nil {
				break
			}

			slots[addr][slot.Key] = slot
		}
	}

	return &EVMState{
		Accounts: accounts,
		Codes:    codes,
		Slots:    slots,
	}, nil
}
