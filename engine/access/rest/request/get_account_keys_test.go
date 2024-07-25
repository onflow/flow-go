package request

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
)

func Test_GetAccountKeys_InvalidParse(t *testing.T) {
	var getAccountKeys GetAccountKeys

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
		err := getAccountKeys.Parse(test.address, test.height, chain)
		assert.EqualError(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

func Test_GetAccountKeys_ValidParse(t *testing.T) {
	var getAccountKeys GetAccountKeys

	addr := "f8d6e0586b0a20c7"
	chain := flow.Localnet.Chain()
	err := getAccountKeys.Parse(addr, "", chain)
	assert.NoError(t, err)
	assert.Equal(t, getAccountKeys.Address.String(), addr)
	assert.Equal(t, getAccountKeys.Height, SealedHeight)

	err = getAccountKeys.Parse(addr, "100", chain)
	assert.NoError(t, err)
	assert.Equal(t, getAccountKeys.Height, uint64(100))
}
