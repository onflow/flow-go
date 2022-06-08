package request

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBlock_InvalidParse(t *testing.T) {
	var getBlock GetBlock

	tooLong := make([]string, MaxBlockRequestHeightRange+1)
	for i := range tooLong {
		tooLong[i] = fmt.Sprintf("%d", i)
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
		{tooLong, "", "", "at most 50 heights can be requested at a time"},
	}

	for i, test := range tests {
		err := getBlock.Parse(test.heights, test.start, test.end)
		assert.EqualError(t, err, test.outErr, fmt.Sprintf("test #%d -> %v", i, test))
	}
}

func TestGetBlock_ValidParse(t *testing.T) {
	tests := []struct {
		heights []string
		start   string
		end     string
	}{
		{[]string{"final"}, "", ""},
		{[]string{""}, "1", "5"},
		{[]string{"1", "2", "3"}, "", ""},
		{nil, "5", "final"},
		{[]string{"sealed"}, "", ""},
	}

	var getBlock GetBlock
	for i, test := range tests {
		err := getBlock.Parse(test.heights, test.start, test.end)
		assert.NoError(t, err, fmt.Sprintf("test #%d -> %v", i, test))
	}

	err := getBlock.Parse([]string{"1", "2"}, "", "")
	assert.NoError(t, err)
	assert.Equal(t, []uint64{1, 2}, getBlock.Heights)
	assert.Equal(t, getBlock.EndHeight, EmptyHeight)
	assert.Equal(t, getBlock.StartHeight, EmptyHeight)

	err = getBlock.Parse([]string{}, "10", "20")
	assert.NoError(t, err)
	assert.Equal(t, []uint64{}, getBlock.Heights)
	assert.Equal(t, getBlock.EndHeight, uint64(20))
	assert.Equal(t, getBlock.StartHeight, uint64(10))

	err = getBlock.Parse([]string{}, "final", "sealed")
	assert.NoError(t, err)
	assert.Equal(t, getBlock.EndHeight, SealedHeight)
	assert.Equal(t, getBlock.StartHeight, FinalHeight)
}
