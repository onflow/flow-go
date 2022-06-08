package request

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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

	for i, test := range tests {
		err := getAccount.Parse(test.address, test.height)
		assert.EqualError(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

func Test_GetAccount_ValidParse(t *testing.T) {
	var getAccount GetAccount

	addr := "f8d6e0586b0a20c7"
	err := getAccount.Parse(addr, "")
	assert.NoError(t, err)
	assert.Equal(t, getAccount.Address.String(), addr)
	assert.Equal(t, getAccount.Height, SealedHeight)

	err = getAccount.Parse(addr, "100")
	assert.NoError(t, err)
	assert.Equal(t, getAccount.Height, uint64(100))
}
