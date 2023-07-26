package request

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_GetAccountKey_InvalidParse(t *testing.T) {
	var getAccountKey GetAccountKey

	tests := []struct {
		address string
		keyID   string
		err     string
	}{
		{"", "", "invalid address"},
		{"f8d6e0586b0a20c7", "-1.2", "invalid key index: value must be an unsigned 64 bit integer"},
	}

	for i, test := range tests {
		err := getAccountKey.Parse(test.address, test.keyID)
		assert.EqualError(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

func Test_GetAccountKey_ValidParse(t *testing.T) {
	var getAccountKey GetAccountKey

	addr := "f8d6e0586b0a20c7"
	keyID := "5"
	err := getAccountKey.Parse(addr, keyID)
	assert.NoError(t, err)
	assert.Equal(t, getAccountKey.Address.String(), addr)
	assert.Equal(t, getAccountKey.KeyID, uint64(5))
}
