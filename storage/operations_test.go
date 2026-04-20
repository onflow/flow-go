package storage_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/storage"
)

func TestPrefixInclusiveEnd(t *testing.T) {
	tests := []struct {
		name     string
		prefix   []byte
		start    []byte
		expected []byte
	}{
		{
			name:     "pads remaining bytes with 0xff",
			prefix:   []byte{0x01},
			start:    []byte{0x01, 0x02, 0x03},
			expected: []byte{0x01, 0xff, 0xff},
		},
		{
			name:     "multi-byte prefix pads remaining bytes",
			prefix:   []byte{0x01, 0x02},
			start:    []byte{0x01, 0x02, 0x03, 0x04},
			expected: []byte{0x01, 0x02, 0xff, 0xff},
		},
		{
			name:     "start same length as prefix - no padding added",
			prefix:   []byte{0x01, 0x02},
			start:    []byte{0x01, 0x02},
			expected: []byte{0x01, 0x02},
		},
		{
			name:     "start bytes after prefix are replaced regardless of value",
			prefix:   []byte{0x01},
			start:    []byte{0x01, 0x00},
			expected: []byte{0x01, 0xff},
		},
		{
			// A shorter start is lexicographically less than prefix, so prefix is returned directly.
			name:     "start shorter than prefix - prefix returned directly",
			prefix:   []byte{0x01, 0x02, 0x03},
			start:    []byte{0x01, 0x02},
			expected: []byte{0x01, 0x02, 0x03},
		},
		{
			// start is lexicographically before prefix, so prefix is already beyond start - no padding needed.
			name:     "start lexicographically before prefix - prefix returned directly",
			prefix:   []byte{0x05},
			start:    []byte{0x03, 0x04},
			expected: []byte{0x05},
		},
		{
			// start content is entirely replaced by prefix + 0xff padding.
			name:     "start lexicographically after prefix - content ignored",
			prefix:   []byte{0x01},
			start:    []byte{0x08, 0x09},
			expected: []byte{0x01, 0xff},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := storage.PrefixInclusiveEnd(tt.prefix, tt.start)
			assert.Equal(t, tt.expected, result)

			// When start is within or past the prefix namespace, result must start with prefix
			// and be padded to len(start).
			if len(tt.start) >= len(tt.prefix) && string(tt.start) >= string(tt.prefix) {
				assert.Len(t, result, len(tt.start))
				assert.Equal(t, tt.prefix, result[:len(tt.prefix)])
			}
		})
	}
}

// TestPrefixInclusiveEnd_GreaterThanAnyKeyWithPrefix verifies that the result is
// always >= any key of the same length that starts with basePrefix.
func TestPrefixInclusiveEnd_GreaterThanAnyKeyWithPrefix(t *testing.T) {
	basePrefix := []byte{0x01, 0x02}
	start := []byte{0x01, 0x02, 0x10, 0x20} // a mid-range start key

	end := storage.PrefixInclusiveEnd(basePrefix, start)

	// Any key of len(start) starting with basePrefix must sort <= end.
	candidates := [][]byte{
		{0x01, 0x02, 0x00, 0x00},
		{0x01, 0x02, 0x10, 0x20},
		{0x01, 0x02, 0xff, 0xfe},
	}
	for _, key := range candidates {
		assert.True(t, string(key) <= string(end),
			"expected key %x <= end %x", key, end)
	}
}
