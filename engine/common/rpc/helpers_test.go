package rpc

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCheckScriptSize(t *testing.T) {
	tests := []struct {
		name      string
		script    []byte
		arguments [][]byte
		maxSize   uint
		expected  bool
	}{
		{
			name:      "empty script and arguments within limit",
			script:    []byte{},
			arguments: [][]byte{},
			maxSize:   100,
			expected:  true,
		},
		{
			name:      "script within limit, no arguments",
			script:    []byte("test script"),
			arguments: [][]byte{},
			maxSize:   100,
			expected:  true,
		},
		{
			name:      "script and arguments within limit",
			script:    []byte("test script"),
			arguments: [][]byte{[]byte("arg1"), []byte("arg2")},
			maxSize:   100,
			expected:  true,
		},
		{
			name:      "script exactly at limit, no arguments",
			script:    make([]byte, 50),
			arguments: [][]byte{},
			maxSize:   50,
			expected:  true,
		},
		{
			name:      "script and arguments exactly at limit",
			script:    make([]byte, 30),
			arguments: [][]byte{make([]byte, 20)},
			maxSize:   50,
			expected:  true,
		},
		{
			name:      "script exceeds limit",
			script:    make([]byte, 60),
			arguments: [][]byte{},
			maxSize:   50,
			expected:  false,
		},
		{
			name:      "script within limit but arguments exceed limit",
			script:    make([]byte, 30),
			arguments: [][]byte{make([]byte, 25)},
			maxSize:   50,
			expected:  false,
		},
		{
			name:      "script and arguments combined exceed limit",
			script:    make([]byte, 30),
			arguments: [][]byte{make([]byte, 15), make([]byte, 10)},
			maxSize:   50,
			expected:  false,
		},
		{
			name:      "multiple arguments exceed limit",
			script:    make([]byte, 10),
			arguments: [][]byte{make([]byte, 15), make([]byte, 20), make([]byte, 10)},
			maxSize:   50,
			expected:  false,
		},
		{
			name:      "zero max size with empty inputs",
			script:    []byte{},
			arguments: [][]byte{},
			maxSize:   0,
			expected:  true,
		},
		{
			name:      "zero max size with non-empty inputs",
			script:    []byte("test"),
			arguments: [][]byte{},
			maxSize:   0,
			expected:  false,
		},
		{
			name:      "large script with large arguments",
			script:    make([]byte, 1000),
			arguments: [][]byte{make([]byte, 500), make([]byte, 300)},
			maxSize:   2000,
			expected:  true,
		},
		{
			name:      "large script with large arguments exceeding limit",
			script:    make([]byte, 1000),
			arguments: [][]byte{make([]byte, 500), make([]byte, 600)},
			maxSize:   2000,
			expected:  false,
		},
		{
			name:      "nil script and arguments",
			script:    nil,
			arguments: [][]byte{nil, nil},
			maxSize:   100,
			expected:  true,
		},
		{
			name:      "nil arguments",
			script:    []byte("test"),
			arguments: nil,
			maxSize:   100,
			expected:  true,
		},
		{
			name:      "mixed nil and non-nil arguments",
			script:    []byte("test"),
			arguments: [][]byte{nil, []byte("arg"), nil},
			maxSize:   100,
			expected:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CheckScriptSize(tt.script, tt.arguments, tt.maxSize)
			assert.Equal(t, tt.expected, result)
		})
	}
}
