package accounts_test

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/sdk/accounts"
)

func TestCreateAccount(t *testing.T) {
	RegisterTestingT(t)

	publicKey := []byte{4, 136, 178, 30, 0, 0, 0, 0, 0, 0, 0, 0, 0, 111, 117, 56, 107, 245, 122, 184, 40, 127, 172, 19, 175, 225, 131, 184, 22, 122, 23, 90, 172, 214, 144, 150, 92, 69, 119, 218, 11, 191, 120, 226, 74, 2, 217, 156, 75, 44, 44, 121, 152, 143, 47, 180, 169, 205, 18, 77, 47, 135, 146, 34, 34, 157, 69, 149, 177, 141, 80, 99, 66, 186, 33, 25, 73, 179, 224, 166, 205, 172}

	// create account with no code
	scriptA := accounts.CreateAccount([][]byte{publicKey}, []byte{})

	Expect(scriptA).To(Equal([]byte(`
		fun main() {
			let publicKeys = [[4,136,178,30,0,0,0,0,0,0,0,0,0,111,117,56,107,245,122,184,40,127,172,19,175,225,131,184,22,122,23,90,172,214,144,150,92,69,119,218,11,191,120,226,74,2,217,156,75,44,44,121,152,143,47,180,169,205,18,77,47,135,146,34,34,157,69,149,177,141,80,99,66,186,33,25,73,179,224,166,205,172]]
			let code: [Int]? = nil
			createAccount(publicKeys, code)
		}
	`)))

	// create account with code
	scriptB := accounts.CreateAccount([][]byte{publicKey}, []byte("fun main() {}"))

	Expect(scriptB).To(Equal([]byte(`
		fun main() {
			let publicKeys = [[4,136,178,30,0,0,0,0,0,0,0,0,0,111,117,56,107,245,122,184,40,127,172,19,175,225,131,184,22,122,23,90,172,214,144,150,92,69,119,218,11,191,120,226,74,2,217,156,75,44,44,121,152,143,47,180,169,205,18,77,47,135,146,34,34,157,69,149,177,141,80,99,66,186,33,25,73,179,224,166,205,172]]
			let code: [Int]? = [102,117,110,32,109,97,105,110,40,41,32,123,125]
			createAccount(publicKeys, code)
		}
	`)))
}

func TestUpdateAccountCode(t *testing.T) {
	RegisterTestingT(t)

	address := types.HexToAddress("0000000000000000000000000000000000000001")
	script := accounts.UpdateAccountCode(address, []byte("fun main() {}"))

	Expect(script).To(Equal([]byte(`
		fun main() {
			let account = [0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0,1]
			let code = [102,117,110,32,109,97,105,110,40,41,32,123,125]
			updateAccountCode(account, code)
		}
	`)))
}
