package request

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
)

func Test_GetAccount_InvalidParse(t *testing.T) {
	var getAccount GetAccount

	tests := []struct {
		address string
		height  string
		err     string
	}{
		{"", "", "invalid address"},
		{"f8d6e0586b0a20c7", "-1", "invalid height format"},
	}

	chain := flow.Localnet.Chain()
	for i, test := range tests {
		err := getAccount.Parse(test.address, test.height, chain)
		assert.EqualError(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

func Test_GetAccount_ValidParse(t *testing.T) {
	var getAccount GetAccount

	addr := "f8d6e0586b0a20c7"
	chain := flow.Localnet.Chain()
	err := getAccount.Parse(addr, "", chain)
	assert.NoError(t, err)
	assert.Equal(t, getAccount.Address.String(), addr)
	assert.Equal(t, getAccount.Height, SealedHeight)

	err = getAccount.Parse(addr, "100", chain)
	assert.NoError(t, err)
	assert.Equal(t, getAccount.Height, uint64(100))
}
