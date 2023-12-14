package netconf_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/config"
	"github.com/onflow/flow-go/network/netconf"
)

// TestSetAliases ensures every network configuration key prefixed with "network" has an alias without the "network" prefix.
func TestSetAliases(t *testing.T) {
	c := viper.New()
	for _, key := range netconf.AllFlagNames() {
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

	err := netconf.SetAliases(c)
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

// TestCrossReferenceFlagsAndConfigs ensures every network configuration in the config file has a corresponding CLI flag.
func TestCrossReferenceFlagsAndConfigs(t *testing.T) {
	// reads the default config file
	c := config.RawViperConfig()
	err := netconf.SetAliases(c)
	require.NoError(t, err)
}
