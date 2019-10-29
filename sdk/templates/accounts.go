package templates

import (
	"fmt"
	"strings"

	"github.com/dapperlabs/flow-go/pkg/types"
)

// CreateAccount generates a script that creates a new account.
func CreateAccount(accountKeys []types.AccountPublicKey, code []byte) ([]byte, error) {
	publicKeys := make([][]byte, len(accountKeys))

	for i, accountKey := range accountKeys {
		accountKeyBytes, err := types.EncodeAccountPublicKey(accountKey)
		if err != nil {
			return nil, err
		}

		publicKeys[i] = accountKeyBytes
	}

	publicKeysStr := cadenceEncodeBytesArray(publicKeys)
	codeStr := cadenceEncodeBytes(code)

	script := fmt.Sprintf(`
		fun main() {
			let publicKeys: [[Int]] = %s
			let code: [Int]? = %s
			createAccount(publicKeys, code)
		}
	`, publicKeysStr, codeStr)

	return []byte(script), nil
}

// UpdateAccountCode generates a script that updates the code associated with an account.
func UpdateAccountCode(code []byte) []byte {
	codeStr := cadenceEncodeBytes(code)

	script := fmt.Sprintf(`
		fun main(account: Account) {
			let code = %s
			updateAccountCode(account.address, code)
		}
	`, codeStr)

	return []byte(script)
}

// AddAccountKey generates a script that adds a key to an account.
func AddAccountKey(accountKey types.AccountPublicKey) ([]byte, error) {
	accountKeyBytes, err := types.EncodeAccountPublicKey(accountKey)
	if err != nil {
		return nil, err
	}

	publicKeyStr := cadenceEncodeBytes(accountKeyBytes)

	script := fmt.Sprintf(`
		fun main(account: Account) {
			let key = %s
			addAccountKey(account.address, key)
		}
	`, publicKeyStr)

	return []byte(script), nil
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

// cadenceEncodeBytes converts a byte slice to a comma-separated list of uint8 integers.
func cadenceEncodeBytes(b []byte) string {
	if b == nil || len(b) == 0 {
		return "[]"
	}

	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}

// cadenceEncodeBytesArray converts a slice of byte slices to a comma-separated list of uint8 integers.
//
// Example: [][]byte{[]byte{1}, []byte{2,3}} -> "[[1],[2,3]]"
func cadenceEncodeBytesArray(b [][]byte) string {
	if b == nil || len(b) == 0 {
		return "[]"
	}

	return strings.Join(strings.Fields(fmt.Sprintf("%d", b)), ",")
}

// cadenceEncodeIntArray converts a slice of integers to a comma-separated list.
func cadenceEncodeIntArray(i []int) string {
	if i == nil || len(i) == 0 {
		return "[]"
	}

	return strings.Join(strings.Fields(fmt.Sprintf("%d", i)), ",")
}
