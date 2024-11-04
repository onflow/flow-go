package request

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/model/flow"
)

func TestAddress_InvalidParse(t *testing.T) {
	inputs := []string{
		"0x1",
		"",
		"foo",
		"1",
		"@",
		"ead892083b3e2c61222", // too long
	}

	chain := flow.Localnet.Chain()
	for _, input := range inputs {
		_, err := ParseAddress(input, chain)
		assert.EqualError(t, err, "invalid address")
	}
}

func TestAddress_InvalidNetwork(t *testing.T) {
	inputs := []string{
		"18eb4ee6b3c026d2",
		"0x18eb4ee6b3c026d2",
	}

	chain := flow.Localnet.Chain()
	for _, input := range inputs {
		_, err := ParseAddress(input, chain)
		require.Error(t, err)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	}
}

func TestAddress_ValidParse(t *testing.T) {
	inputs := []string{
		"f8d6e0586b0a20c7",
		"148602c0600814da",
		"0x0b807ae5da6210df",
	}

	chain := flow.Localnet.Chain()
	for _, input := range inputs {
		address, err := ParseAddress(input, chain)
		require.NoError(t, err)
		assert.Equal(t, strings.ReplaceAll(input, "0x", ""), address.String())
	}
}
