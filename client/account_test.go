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
