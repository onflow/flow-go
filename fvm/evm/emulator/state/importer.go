package state

import (
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

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

func ImportEVMStateFromGob(path string) (*EVMState, error) {
	fileName := filepath.Join(path, ExportedStateGobFileName)
	// Open the file for reading
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Prepare the map to store decoded data
	var data EVMState

	// Use gob to decode data
	decoder := gob.NewDecoder(file)
	err = decoder.Decode(&data)
	if err != nil {
		return nil, err
	}

	return &data, nil
}

func ImportEVMState(path string) (*EVMState, error) {
	accounts := make(map[gethCommon.Address]*Account)
	var codes []*CodeInContext
	var slots []*types.SlotEntry
	// Import codes
	codesData, err := ioutil.ReadFile(filepath.Join(path, ExportedCodesFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening codes file: %w", err)
	}
	codesLines := strings.Split(string(codesData), "\n")
	for _, line := range codesLines {
		if line == "" {
			continue
		}
		code, err := CodeInContextFromEncoded([]byte(line))
		if err != nil {
			return nil, fmt.Errorf("error decoding code in context: %w", err)
		}
		codes = append(codes, code)
	}

	// Import slots
	slotsData, err := ioutil.ReadFile(filepath.Join(path, ExportedSlotsFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening slots file: %w", err)
	}
	slotsLines := strings.Split(string(slotsData), "\n")
	for _, line := range slotsLines {
		if line == "" {
			continue
		}
		slot, err := types.SlotEntryFromEncoded([]byte(line))
		if err != nil {
			return nil, fmt.Errorf("error decoding slot entry: %w", err)
		}
		slots = append(slots, slot)
	}

	// Import accounts
	accountsData, err := ioutil.ReadFile(filepath.Join(path, ExportedAccountsFileName))
	if err != nil {
		return nil, fmt.Errorf("error opening accounts file: %w", err)
	}
	accountsLines := strings.Split(string(accountsData), "\n")
	for _, line := range accountsLines {
		if line == "" {
			continue
		}
		acc, err := DecodeAccount([]byte(line))
		if err != nil {
			fmt.Println("error decoding account: ", err, line)
		} else {
			fmt.Println("decoded account", acc.Address)
			accounts[acc.Address] = acc
		}
	}
	return ToEVMState(accounts, codes, slots)
}
