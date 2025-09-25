package request

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
)

func Test_GetAccountBalance_InvalidParse(t *testing.T) {
	var getAccountBalance GetAccountBalance

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
		err := getAccountBalance.Parse(test.address, test.height, chain)
		assert.EqualError(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

func Test_GetAccountBalance_ValidParse(t *testing.T) {

	var getAccountBalance GetAccountBalance

	addr := "f8d6e0586b0a20c7"
	chain := flow.Localnet.Chain()
	err := getAccountBalance.Parse(addr, "", chain)
	assert.NoError(t, err)
	assert.Equal(t, getAccountBalance.Address.String(), addr)
	assert.Equal(t, getAccountBalance.Height, SealedHeight)

	err = getAccountBalance.Parse(addr, "100", chain)
	assert.NoError(t, err)
	assert.Equal(t, getAccountBalance.Height, uint64(100))

	err = getAccountBalance.Parse(addr, sealed, chain)
	assert.NoError(t, err)
	assert.Equal(t, getAccountBalance.Height, SealedHeight)

	err = getAccountBalance.Parse(addr, final, chain)
	assert.NoError(t, err)
	assert.Equal(t, getAccountBalance.Height, FinalHeight)
}
