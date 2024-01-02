package netconf_test

import (
	"testing"

	"github.com/onflow/flow-go/network/netconf"
)

// TestBuildFlagName tests the BuildFlagName function for various cases
func TestBuildFlagName(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		expected string
	}{
		{
			name:     "Single key",
			keys:     []string{"key1"},
			expected: "key1",
		},
		{
			name:     "Two keys",
			keys:     []string{"key1", "key2"},
			expected: "key1-key2",
		},
		{
			name:     "Multiple keys",
			keys:     []string{"key1", "key2", "key3"},
			expected: "key1-key2-key3",
		},
		{
			name:     "No keys",
			keys:     []string{},
			expected: "",
		},
		{
			name:     "Key with spaces",
			keys:     []string{"key 1", "key 2"},
			expected: "key 1-key 2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := netconf.BuildFlagName(tt.keys...)
			if result != tt.expected {
				t.Errorf("BuildFlagName(%v) = %v, want %v", tt.keys, result, tt.expected)
			}
		})
	}
}
