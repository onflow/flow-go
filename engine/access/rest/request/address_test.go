package request

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestAddress_InvalidParse(t *testing.T) {
	var address Address
	inputs := []string{
		"0x1",
		"",
		"foo",
		"1",
		"@",
		"ead892083b3e2c61222",
	}

	for _, input := range inputs {
		err := address.Parse(input)
		assert.EqualError(t, err, "invalid address")
	}
}

func TestAddress_ValidParse(t *testing.T) {
	var address Address
	inputs := []string{
		"f8d6e0586b0a20c7",
		"f3ad66eea58c97d2",
		"0xead892083b3e2c6c",
	}

	for _, input := range inputs {
		err := address.Parse(input)
		assert.NoError(t, err)
		assert.Equal(t, strings.ReplaceAll(input, "0x", ""), address.Flow().String())
	}
}
