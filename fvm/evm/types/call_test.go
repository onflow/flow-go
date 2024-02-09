package types

import "testing"

func TestDirectCall_Hash(t *testing.T) {

	dc := &DirectCall{
		Type:     0,
		SubType:  0,
		From:     Address{},
		To:       Address{},
		Data:     nil,
		Value:    nil,
		GasLimit: 0,
	}

}
