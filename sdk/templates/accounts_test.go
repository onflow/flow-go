package templates_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/pkg/constants"
	"github.com/dapperlabs/flow-go/pkg/crypto"
	"github.com/dapperlabs/flow-go/pkg/types"
	"github.com/dapperlabs/flow-go/pkg/utils/unittest"
	"github.com/dapperlabs/flow-go/sdk/templates"
)

func TestCreateAccount(t *testing.T) {
	publicKey := unittest.PublicKeyFixtures()[0]

	accountKey := types.AccountKey{
		PublicKey: publicKey,
		SignAlgo:  publicKey.Algorithm(),
		HashAlgo:  crypto.SHA3_256,
		Weight:    constants.AccountKeyWeightThreshold,
	}

	// create account with no code
	scriptA, err := templates.CreateAccount([]types.AccountKey{accountKey}, []byte{})
	assert.Nil(t, err)

	expectedScriptA := []byte(`
		fun main() {
			let publicKeys: [[Int]] = [[248,98,184,91,48,89,48,19,6,7,42,134,72,206,61,2,1,6,8,42,134,72,206,61,3,1,7,3,66,0,4,114,176,116,164,82,208,167,100,161,218,52,49,143,68,203,22,116,13,241,207,171,30,107,80,229,228,20,93,192,110,93,21,28,156,37,36,79,18,62,83,201,182,254,35,117,4,163,126,119,121,144,10,173,83,202,38,227,181,124,92,61,112,48,196,1,2,130,3,232]]
			let code: [Int]? = []
			createAccount(publicKeys, code)
		}
	`)

	assert.Equal(t, expectedScriptA, scriptA)

	// create account with code
	scriptB, err := templates.CreateAccount([]types.AccountKey{accountKey}, []byte("fun main() {}"))
	assert.Nil(t, err)

	expectedScriptB := []byte(`
		fun main() {
			let publicKeys: [[Int]] = [[248,98,184,91,48,89,48,19,6,7,42,134,72,206,61,2,1,6,8,42,134,72,206,61,3,1,7,3,66,0,4,114,176,116,164,82,208,167,100,161,218,52,49,143,68,203,22,116,13,241,207,171,30,107,80,229,228,20,93,192,110,93,21,28,156,37,36,79,18,62,83,201,182,254,35,117,4,163,126,119,121,144,10,173,83,202,38,227,181,124,92,61,112,48,196,1,2,130,3,232]]
			let code: [Int]? = [102,117,110,32,109,97,105,110,40,41,32,123,125]
			createAccount(publicKeys, code)
		}
	`)

	assert.Equal(t, expectedScriptB, scriptB)
}

func TestUpdateAccountCode(t *testing.T) {
	script := templates.UpdateAccountCode([]byte("fun main() {}"))

	expectedScript := []byte(`
		fun main(account: Account) {
			let code = [102,117,110,32,109,97,105,110,40,41,32,123,125]
			updateAccountCode(account.address, code)
		}
	`)

	assert.Equal(t, expectedScript, script)
}
