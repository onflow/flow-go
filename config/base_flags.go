package config

import (
	"github.com/spf13/pflag"

	"github.com/onflow/flow-go/network/netconf"
)

const (
	configFileFlagName = "config-file"
)

// InitializePFlagSet initializes all CLI flags for the Flow node base configuration on the provided pflag set.
// Args:
//
//	*pflag.FlagSet: the pflag set of the Flow node.
//	*FlowConfig: the config used to set default values on the flags
//
// Note: in subsequent PR's all flag initialization for Flow node should be moved to this func.
func InitializePFlagSet(flags *pflag.FlagSet, config *FlowConfig) {
	flags.String(configFileFlagName, "", "provide a path to a Flow configuration file that will be used to set configuration values")
	netconf.InitializeNetworkFlags(flags, config.NetworkConfig)
}
