package netconf

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

// TestSetAliases ensures every network configuration key prefixed with "network" has an alias without the "network" prefix.
func TestSetAliases(t *testing.T) {
	c := viper.New()
	for _, key := range AllFlagNames() {
		c.Set(fmt.Sprintf("network.%s", key), "not aliased")
		c.Set(key, "aliased")
	}

	// ensure network prefixed keys do not point to non-prefixed alias
	for _, key := range c.AllKeys() {
		parts := strings.Split(key, ".")
		if len(parts) != 2 {
			continue
		}
		require.NotEqual(t, c.GetString(parts[1]), c.GetString(key))
	}

	err := SetAliases(c)
	require.NoError(t, err)

	// ensure each network prefixed key now points to the non-prefixed alias
	for _, key := range c.AllKeys() {
		parts := strings.Split(key, ".")
		if len(parts) != 2 {
			continue
		}
		require.Equal(t, c.GetString(parts[1]), c.GetString(key))
	}
}
