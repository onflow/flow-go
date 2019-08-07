package client_test

import (
	"strings"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/dapperlabs/bamboo-node/client"
)

func TestLoadAccount(t *testing.T) {
	RegisterTestingT(t)

	r := strings.NewReader(`
		{	
			"account": "0xdd2781f4c51bccdbe23e4d398b8a82261f585c278dbb4b84989fea70e76723a9",
			"seed": "elephant ears"
		}
	`)

	a, err := client.LoadAccount(r)
	Expect(err).ToNot(HaveOccurred())

	Expect(a.Account).ToNot(BeNil())
	Expect(a.KeyPair).ToNot(BeNil())

	// invalid json should fail
	r = strings.NewReader(`
		{	
			"account": "0xdd2781
		}
	`)

	a, err = client.LoadAccount(r)
	Expect(err).To(HaveOccurred())
}

func TestCreateAccount(t *testing.T) {
	RegisterTestingT(t)

	publicKey := []byte{4, 136, 178, 30, 0, 0, 0, 0, 0, 0, 0, 0, 0, 111, 117, 56, 107, 245, 122, 184, 40, 127, 172, 19, 175, 225, 131, 184, 22, 122, 23, 90, 172, 214, 144, 150, 92, 69, 119, 218, 11, 191, 120, 226, 74, 2, 217, 156, 75, 44, 44, 121, 152, 143, 47, 180, 169, 205, 18, 77, 47, 135, 146, 34, 34, 157, 69, 149, 177, 141, 80, 99, 66, 186, 33, 25, 73, 179, 224, 166, 205, 172}

	// create account with no code
	txA := client.CreateAccount(publicKey, []byte{})

	Expect(txA.Script).To(Equal([]byte(`
		fun main() {
			let publicKey = [4,136,178,30,0,0,0,0,0,0,0,0,0,111,117,56,107,245,122,184,40,127,172,19,175,225,131,184,22,122,23,90,172,214,144,150,92,69,119,218,11,191,120,226,74,2,217,156,75,44,44,121,152,143,47,180,169,205,18,77,47,135,146,34,34,157,69,149,177,141,80,99,66,186,33,25,73,179,224,166,205,172]
			let code = []
			createAccount(publicKey, code)
		}
	`)))

	// create account with code
	txB := client.CreateAccount(publicKey, []byte("fun main() {}"))

	Expect(txB.Script).To(Equal([]byte(`
		fun main() {
			let publicKey = [4,136,178,30,0,0,0,0,0,0,0,0,0,111,117,56,107,245,122,184,40,127,172,19,175,225,131,184,22,122,23,90,172,214,144,150,92,69,119,218,11,191,120,226,74,2,217,156,75,44,44,121,152,143,47,180,169,205,18,77,47,135,146,34,34,157,69,149,177,141,80,99,66,186,33,25,73,179,224,166,205,172]
			let code = [102,117,110,32,109,97,105,110,40,41,32,123,125]
			createAccount(publicKey, code)
		}
	`)))
}
