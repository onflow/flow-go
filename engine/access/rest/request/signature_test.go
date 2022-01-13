package request

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestSignature_InvalidParse(t *testing.T) {
	var signature Signature

	tests := []struct {
		in  string
		err string
	}{
		{"s", "invalid encoding"},
		{"", "missing value"},
	}

	for _, test := range tests {
		err := signature.Parse(test.in)
		assert.EqualError(t, err, test.err)
	}

}
func TestSignature_ValidParse(t *testing.T) {
	var signature Signature
	err := signature.Parse("Mzg0MTQ5ODg4ZTg4MjRmYjMyNzM4MmM2ZWQ4ZjNjZjk1ODRlNTNlMzk4NGNhMDAxZmZjMjgwNzM4NmM0MzY3NTYxNmYwMTAwMTMzNDVkNjhmNzZkMmQ5YTBkYmI1MDA0MmEzOWRlOThlYzAzNTJjYTBkZWY3YjBlNjQ0YWJjOTQ=")
	assert.NoError(t, err)
	signature.Flow()
}
