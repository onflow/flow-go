package accounts

import (
	"fmt"
	"strings"

	"github.com/dapperlabs/flow-go/pkg/types"
)

// CreateAccount generates a script that creates a new account.
func CreateAccount(accountKeys []types.AccountKey, code []byte) []byte {
	publicKeys := make([][]byte, len(accountKeys))
	keyWeights := make([]int, len(accountKeys))

	for i, accountKey := range accountKeys {
		publicKeys[i] = accountKey.PublicKey
		keyWeights[i] = accountKey.Weight
	}

	publicKeysStr := bytesArrayToString(publicKeys)
	keyWeightsStr := intArrayToString(keyWeights)
	codeStr := bytesToString(code)

	script := fmt.Sprintf(`
		fun main() {
			let publicKeys = %s
			let keyWeights = %s
			let code: [Int]? = %s
			createAccount(publicKeys, keyWeights, code)
		}
	`, publicKeysStr, keyWeightsStr, codeStr)

	return []byte(script)
}

// UpdateAccountCode generates a script that updates the code associated with an account.
func UpdateAccountCode(code []byte) []byte {
	codeStr := bytesToString(code)

	script := fmt.Sprintf(`
		fun main(account: Account) {
			let code = %s
			updateAccountCode(account.address, code)
		}
	`, codeStr)

	return []byte(script)
}

// AddAccountKey generates a script that adds a key to an account.
func AddAccountKey(accountKey types.AccountKey) []byte {
	publicKeyStr := bytesToString(accountKey.PublicKey)

	script := fmt.Sprintf(`
		fun main(account: Account) {
			let code = %s
			let weight = %d
			addAccountKey(account.address, code, weight)
		}
	`, publicKeyStr, accountKey.Weight)

	return []byte(script)
}

// RemoveAccountKey generates a script that removes a key from an account.
func RemoveAccountKey(index int) []byte {
	script := fmt.Sprintf(`
		fun main(account: Account) {
			let index = %d
			removeAccountKey(account.address, index)
		}
	`, index)

	return []byte(script)
}

// bytesToString converts a byte slice to a comma-separated list of uint8 integers.
func bytesToString(b []byte) string {
	if b == nil || len(b) == 0 {
		return "nil"
	}

	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}

// bytesArrayToString converts a slice of byte slices to a comma-separated list of uint8 integers.
//
// Example: [][]byte{[]byte{1}, []byte{2,3}} -> "[[1],[2,3]]"
func bytesArrayToString(b [][]byte) string {
	if b == nil || len(b) == 0 {
		return "nil"
	}

	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}

// intArrayToString converts a slice of integers to a comma-separated list.
func intArrayToString(i []int) string {
	if i == nil || len(i) == 0 {
		return "nil"
	}

	return strings.Join(strings.Fields(fmt.Sprintf("%d", i)), ",")
}
