package state

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"

	gethCommon "github.com/onflow/go-ethereum/common"

	"github.com/onflow/flow-go/fvm/evm/types"
)

type EVMState struct {
	Accounts map[gethCommon.Address]*Account
	Codes    map[gethCommon.Hash]*CodeInContext
	// account address -> key -> value
	Slots map[gethCommon.Address]map[gethCommon.Hash]*types.SlotEntry
}

func ToEVMState(
	accounts map[gethCommon.Address]*Account,
	codes []*CodeInContext,
	slots []*types.SlotEntry,
) (*EVMState, error) {
	state := &EVMState{
		Accounts: accounts,
		Codes:    make(map[gethCommon.Hash]*CodeInContext),
		Slots:    make(map[gethCommon.Address]map[gethCommon.Hash]*types.SlotEntry),
	}

	// Process codes
	for _, code := range codes {
		if _, ok := state.Codes[code.Hash]; ok {
			return nil, fmt.Errorf("duplicate code hash: %s", code.Hash)
		}
		state.Codes[code.Hash] = code
	}

	// Process slots
	for _, slot := range slots {
		if _, ok := state.Slots[slot.Address]; !ok {
			state.Slots[slot.Address] = make(map[gethCommon.Hash]*types.SlotEntry)
		}

		if _, ok := state.Slots[slot.Address][slot.Key]; ok {
			return nil, fmt.Errorf("duplicate slot key: %s", slot.Key)
		}

		state.Slots[slot.Address][slot.Key] = slot
	}

	return state, nil
}

func ImportEVMState(path string) (*EVMState, error) {
	accounts := make(map[gethCommon.Address]*Account)
	var codes []*CodeInContext
	var slots []*types.SlotEntry

	// Import codes
	codesFile, err := os.Open(filepath.Join(path, ExportedCodesFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening codes file: %w", err)
	}
	defer codesFile.Close()

	scanner := bufio.NewScanner(codesFile)
	for scanner.Scan() {
		code, err := CodeInContextFromEncoded(scanner.Bytes())
		if err != nil {
			return nil, fmt.Errorf("error decoding code in context: %w", err)
		}
		codes = append(codes, code)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading codes file: %w", err)
	}

	// Import slots
	slotsFile, err := os.Open(filepath.Join(path, ExportedSlotsFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening slots file: %w", err)
	}
	defer slotsFile.Close()

	scanner = bufio.NewScanner(slotsFile)
	for scanner.Scan() {
		slot, err := types.SlotEntryFromEncoded(scanner.Bytes())
		if err != nil {
			return nil, fmt.Errorf("error decoding slot entry: %w", err)
		}
		slots = append(slots, slot)
	}

	// Import accounts
	accountsFile, err := os.Open(filepath.Join(path, ExportedAccountsFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening accounts file: %w", err)
	}
	defer accountsFile.Close()

	scanner = bufio.NewScanner(accountsFile)
	for scanner.Scan() {
		acc, err := DecodeAccount(scanner.Bytes())
		if err != nil {
			fmt.Println("error decoding account: ", err, scanner.Bytes())
		} else {
			fmt.Println("decoded account", acc.Address)
		}
		accounts[acc.Address] = acc
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading accounts file: %w", err)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading slots file: %w", err)
	}

	return ToEVMState(accounts, codes, slots)
}
