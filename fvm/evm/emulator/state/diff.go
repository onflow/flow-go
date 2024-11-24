package state

import (
	"bytes"
	"fmt"
)

func AccountEqual(a, b *Account) bool {
	if a.Address != b.Address {
		return false
	}
	if !bytes.Equal(a.Balance.Bytes(), b.Balance.Bytes()) {
		return false
	}
	if a.Nonce != b.Nonce {
		return false
	}
	if a.CodeHash != b.CodeHash {
		return false
	}

	// CollectionID could be different
	return true
}

// find the difference and return as error
func Diff(a *EVMState, b *EVMState) []error {
	var differences []error

	// Compare Accounts
	for addr, accA := range a.Accounts {
		if accB, exists := b.Accounts[addr]; exists {
			if !AccountEqual(accA, accB) {
				differences = append(differences, fmt.Errorf("account %s differs", addr.Hex()))
			}
		} else {
			differences = append(differences, fmt.Errorf("account %s exists in a but not in b", addr.Hex()))
		}
	}
	for addr := range b.Accounts {
		if _, exists := a.Accounts[addr]; !exists {
			differences = append(differences, fmt.Errorf("account %s exists in b but not in a", addr.Hex()))
		}
	}

	// Compare Slots
	for addr, slotsA := range a.Slots {
		slotsB, exists := b.Slots[addr]
		if !exists {
			differences = append(differences, fmt.Errorf("slots for address %s exist in a but not in b", addr.Hex()))
			continue
		}
		for key, valueA := range slotsA {
			if valueB, exists := slotsB[key]; exists {
				if valueA.Value != valueB.Value {
					differences = append(differences, fmt.Errorf("slot value for address %s and key %s differs", addr.Hex(), key.Hex()))
				}
			} else {
				differences = append(differences, fmt.Errorf("slot with key %s for address %s exists in a but not in b", key.Hex(), addr.Hex()))
			}
		}
		for key := range slotsB {
			if _, exists := slotsA[key]; !exists {
				differences = append(differences, fmt.Errorf("slot with key %s for address %s exists in b but not in a", key.Hex(), addr.Hex()))
			}
		}
	}
	for addr := range b.Slots {
		if _, exists := a.Slots[addr]; !exists {
			differences = append(differences, fmt.Errorf("slots for address %s exist in b but not in a", addr.Hex()))
		}
	}

	// Compare Codes
	for hash, codeA := range a.Codes {
		if codeB, exists := b.Codes[hash]; exists {
			if !bytes.Equal(codeA.Code, codeB.Code) {
				differences = append(differences, fmt.Errorf("code for hash %s differs", hash.Hex()))
			}
		} else {
			differences = append(differences, fmt.Errorf("code with hash %s exists in a but not in b", hash.Hex()))
		}
	}
	for hash := range b.Codes {
		if _, exists := a.Codes[hash]; !exists {
			differences = append(differences, fmt.Errorf("code with hash %s exists in b but not in a", hash.Hex()))
		}
	}

	return differences
}
