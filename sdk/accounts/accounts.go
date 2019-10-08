package accounts

import (
	"fmt"
	"strings"

	"github.com/dapperlabs/flow-go/pkg/types"
)

// CreateAccount generates a script that creates a new account.
func CreateAccount(publicKeys [][]byte, code []byte) []byte {
	publicKeysStr := bytesArrayToString(publicKeys)
	codeStr := bytesToString(code)

	script := fmt.Sprintf(`
		fun main() {
			let publicKeys = %s
			let code: [Int]? = %s
			createAccount(publicKeys, code)
		}
	`, publicKeysStr, codeStr)

	return []byte(script)
}

// UpdateAccountCode generates a script that updates the code associated with an account.
func UpdateAccountCode(account types.Address, code []byte) []byte {
	accountStr := bytesToString(account.Bytes())
	codeStr := bytesToString(code)

	script := fmt.Sprintf(`
		fun main() {
			let account = %s
			let code = %s
			updateAccountCode(account, code)
		}
	`, accountStr, codeStr)

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
