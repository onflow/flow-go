package fingerprint

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/dapperlabs/flow-go/model/fingerprint/mock"
)

func TestFingerprint(t *testing.T) {
	tests := []{
		input  interface{}
		output string
	}{
		{"abc", "0x83616263"},
		{uint(3), "0x03"},
		{[]byte{}, "0x80"},
		{[]byte{0x01, 0xff}, "0x8201ff"},
		{struct{ a uint }{a: uint(2)}, "0xc0"},
	}

	for _, te := range tests {
		t.Run(fmt.Sprintf("Input %#v should produce output %s", te.input, te.output), func(t *testing.T) {
			assert.Equal(t, te.output, fmt.Sprintf("%#x", Fingerprint(te.input)))
		})
	}
}

func TestFingerprinter(t *testing.T) {
	input := new(mock.Fingerprinter)
	input.On("Fingerprint").Return([]byte{0xab, 0xcd, 0xef}).Once()

	assert.Equal(t, "0xabcdef", fmt.Sprintf("%#x", Fingerprint(input)))
	input.AssertExpectations(t)
}
