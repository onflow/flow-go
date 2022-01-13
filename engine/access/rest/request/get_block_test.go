package request

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetBlock_InvalidParse(t *testing.T) {
	var getBlock GetBlock

	tooLong := make([]string, MaxAllowedHeights+1)
	for i := range tooLong {
		tooLong[i] = "1"
	}

	tests := []struct {
		heights []string
		start   string
		end     string
		outErr  string
	}{
		{nil, "", "", "must provide either heights or start and end height range"},
		{[]string{"1"}, "1", "2", "can only provide either heights or start and end height range"},
		{[]string{"final"}, "1", "2", "can only provide either heights or start and end height range"},
		{[]string{"final", "1"}, "", "", "can not provide 'final' or 'sealed' values with other height values"},
		{[]string{"final", "sealed"}, "", "", "can not provide 'final' or 'sealed' values with other height values"},
		{[]string{"", ""}, "", "", "must provide either heights or start and end height range"},
		{[]string{"-1"}, "", "", "invalid height format"},
		{[]string{""}, "-1", "-2", "invalid height format"},
		{[]string{""}, "foo", "10", "invalid height format"},
		{[]string{""}, "10", "1", "start height must be less than or equal to end height"},
		{[]string{""}, "1", "1000", "height range 999 exceeds maximum allowed of 50"},
		{[]string{""}, "1", "", "must provide either heights or start and end height range"},
		{[]string{"1", "1", "1"}, "", "", "must provide either heights or start and end height range"},
		{tooLong, "", "", "at most 50 heights can be requested at a time"},
	}

	for i, test := range tests {
		err := getBlock.Parse(test.heights, test.start, test.end)
		assert.EqualError(t, err, test.outErr, fmt.Sprintf("test #%d", i))
	}

}

//		{[]string{"final"}, "", "", "can only provide either heights or start and end height range"},
