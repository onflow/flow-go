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

// TestCrossReferenceFlagsWithConfigs ensures that each flag is cross-referenced with the config file, i.e., that each
// flag has a corresponding config key.
func TestCrossReferenceFlagsWithConfigs(t *testing.T) {
	// reads the default config file
	c := config.RawViperConfig()
	err := netconf.SetAliases(c)
	require.NoError(t, err)
}

// TestCrossReferenceConfigsWithFlags ensures that each config is cross-referenced with the flags, i.e., that each config
// key has a corresponding flag.
func TestCrossReferenceConfigsWithFlags(t *testing.T) {
	c := config.RawViperConfig()
	// keeps all flag names
	m := make(map[string]struct{})

	// each flag name should correspond to exactly one key in our config store after it is loaded with the default config
	for _, flagName := range netconf.AllFlagNames() {
		m[flagName] = struct{}{}
	}

	for _, key := range c.AllKeys() {
		s := strings.Split(key, ".")
		flag := strings.Join(s[1:], "-")
		if len(flag) == 0 {
			continue
		}
		_, ok := m[flag]
		require.Truef(t, ok, "config key %s does not have a corresponding flag", flag)
	}
}
