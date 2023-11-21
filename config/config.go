package config

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"
	"github.com/rs/zerolog"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/onflow/flow-go/network/netconf"
	"github.com/onflow/flow-go/network/p2p/p2pconf"
)

var (
	conf     = viper.New()
	validate *validator.Validate
	//go:embed default-config.yml
	configFile string

	errPflagsNotParsed = errors.New("failed to bind flags to configuration values, pflags must be parsed before binding")
)

func init() {
	initialize()
}

// FlowConfig Flow configuration.
type FlowConfig struct {
	// ConfigFile used to set a path to a config.yml file used to override the default-config.yml file.
	ConfigFile    string          `validate:"filepath" mapstructure:"config-file"`
	NetworkConfig *netconf.Config `mapstructure:"network-config"`
}

// Validate checks validity of the Flow config. Errors indicate that either the configuration is broken,
// incompatible with the node's internal state, or that the node's internal state is corrupted. In all
// cases, continuation is impossible.
func (fc *FlowConfig) Validate() error {
	err := validate.Struct(fc)
	if err != nil {
		if validationErrors, ok := err.(validator.ValidationErrors); ok {
			return fmt.Errorf("failed to validate flow configuration: %w", validationErrors)
		}
		return fmt.Errorf("unexpeceted error encountered while validating flow configuration: %w", err)
	}
	return nil
}

// DefaultConfig initializes the flow configuration. All default values for the Flow
// configuration are stored in the default-config.yml file. These values can be overridden
// by node operators by setting the corresponding cli flag. DefaultConfig should be called
// before any pflags are parsed, this will allow the configuration to initialize with defaults
// from default-config.yml.
// Returns:
//
//	*FlowConfig: an instance of the network configuration fully initialized to the default values set in the config file
//	error: if there is any error encountered while initializing the configuration, all errors are considered irrecoverable.
func DefaultConfig() (*FlowConfig, error) {
	var flowConfig FlowConfig
	err := Unmarshall(&flowConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshall the Flow config: %w", err)
	}
	return &flowConfig, nil
}

// RawViperConfig returns the raw viper config store.
// Returns:
//
//	*viper.Viper: the raw viper config store.
func RawViperConfig() *viper.Viper {
	return conf
}

// BindPFlags binds the configuration to the cli pflag set. This should be called
// after all pflags have been parsed. If the --config-file flag has been set the config will
// be loaded from the specified config file.
// Args:
//
//	c: The Flow configuration that will be used to unmarshall the configuration values into after binding pflags.
//	This needs to be done because pflags may override a configuration value.
//
// Returns:
//
//	error: if there is any error encountered binding pflags or unmarshalling the config struct, all errors are considered irrecoverable.
//	bool: true if --config-file flag was set and config file was loaded, false otherwise.
//
// Note: As configuration management is improved, this func should accept the entire Flow config as the arg to unmarshall new config values into.
func BindPFlags(c *FlowConfig, flags *pflag.FlagSet) (bool, error) {
	if !flags.Parsed() {
		return false, errPflagsNotParsed
	}

	// update the config store values from config file if --config-file flag is set
	// if config file provided we will use values from the file and skip binding pflags
	overridden, err := overrideConfigFile(flags)
	if err != nil {
		return false, err
	}

	if !overridden {
		err = conf.BindPFlags(flags)
		if err != nil {
			return false, fmt.Errorf("failed to bind pflag set: %w", err)
		}
		setAliases()
	}

	err = Unmarshall(c)
	if err != nil {
		return false, fmt.Errorf("failed to unmarshall the Flow config: %w", err)
	}

	return overridden, nil
}

// Unmarshall unmarshalls the Flow configuration into the provided FlowConfig struct.
// Args:
//
//	flowConfig: the flow config struct used for unmarshalling.
//
// Returns:
//
//	error: if there is any error encountered unmarshalling the configuration, all errors are considered irrecoverable.
func Unmarshall(flowConfig *FlowConfig) error {
	err := conf.Unmarshal(flowConfig, func(decoderConfig *mapstructure.DecoderConfig) {
		// enforce all fields are set on the FlowConfig struct
		decoderConfig.ErrorUnset = true
		// currently the entire flow configuration has not been moved to this package
		// for now we allow key's in the config which are unused.
		decoderConfig.ErrorUnused = false
	})
	if err != nil {
		return fmt.Errorf("failed to unmarshal network config: %w", err)
	}
	return nil
}

// LogConfig logs configuration keys and values if they were overridden with a config file.
// It also returns a map of keys for which the values were set by a config file.
//
// Parameters:
//   - logger: *zerolog.Event to which the configuration keys and values will be logged.
//   - flags: *pflag.FlagSet containing the set flags.
//
// Returns:
//   - map[string]struct{}: map of keys for which the values were set by a config file.
func LogConfig(logger *zerolog.Event, flags *pflag.FlagSet) map[string]struct{} {
	keysToAvoid := make(map[string]struct{})

	if flags.Lookup(configFileFlagName).Changed {
		for _, key := range conf.AllKeys() {
			logger.Str(key, fmt.Sprint(conf.Get(key)))
			parts := strings.Split(key, ".")
			if len(parts) == 2 {
				keysToAvoid[parts[1]] = struct{}{}
			} else {
				keysToAvoid[key] = struct{}{}
			}
		}
	}

	return keysToAvoid
}

// setAliases sets aliases for config sub packages. This should be done directly after pflags are bound to the configuration store.
// Upon initialization the conf will be loaded with the default config values, those values are then used as the default values for
// all the CLI flags, the CLI flags are then bound to the configuration store and at this point all aliases should be set if configuration
// keys do not match the CLI flags 1:1. ie: networking-connection-pruning -> network-config.networking-connection-pruning. After aliases
// are set the conf store will override values with any CLI flag values that are set as expected.
func setAliases() {
	err := netconf.SetAliases(conf)
	if err != nil {
		panic(fmt.Errorf("failed to set network aliases: %w", err))
	}
}

// overrideConfigFile overrides the default config file by reading in the config file at the path set
// by the --config-file flag in our viper config store.
//
// Returns:
//
//	error: if there is any error encountered while reading new config file, all errors are considered irrecoverable.
//	bool: true if the config was overridden by the new config file, false otherwise or if an error is encountered reading the new config file.
func overrideConfigFile(flags *pflag.FlagSet) (bool, error) {
	configFileFlag := flags.Lookup(configFileFlagName)
	if configFileFlag.Changed {
		p := configFileFlag.Value.String()
		dirPath, fileName := splitConfigPath(p)
		conf.AddConfigPath(dirPath)
		conf.SetConfigName(fileName)
		err := conf.ReadInConfig()
		if err != nil {
			return false, fmt.Errorf("failed to read config file %s: %w", p, err)
		}
		if len(conf.AllKeys()) == 0 {
			return false, fmt.Errorf("failed to read in config file no config values found")
		}
		return true, nil
	}
	return false, nil
}

// splitConfigPath returns the directory and base name (without extension) of the config file from the provided path string.
// If the file name does not match the expected pattern, the function panics.
//
// The expected pattern for file names is that they must consist of alphanumeric characters, hyphens, or underscores,
// followed by a single dot and then the extension.
//
// Legitimate Inputs:
//   - /path/to/my_config.yaml
//   - /path/to/my-config123.yaml
//   - my-config.yaml (when in the current directory)
//
// Illegitimate Inputs:
//   - /path/to/my.config.yaml (contains multiple dots)
//   - /path/to/my config.yaml (contains spaces)
//   - /path/to/.config.yaml (does not have a file name before the dot)
//
// Args:
//   - path: The file path string to be split into directory and base name.
//
// Returns:
//   - The directory and base name without extension.
//
// Panics:
//   - If the file name does not match the expected pattern.
func splitConfigPath(path string) (string, string) {
	// Regex to match filenames like 'my_config.yaml' or 'my-config.yaml' but not 'my.config.yaml'
	validFileNamePattern := regexp.MustCompile(`^[a-zA-Z0-9_-]+\.[a-zA-Z0-9]+$`)

	dir, name := filepath.Split(path)

	// Panic if the file name does not match the expected pattern
	if !validFileNamePattern.MatchString(name) {
		panic(fmt.Errorf("Invalid config file name '%s'. Expected pattern: alphanumeric, hyphens, or underscores followed by a single dot and extension", name))
	}

	// Extracting the base name without extension
	baseName := strings.Split(name, ".")[0]
	return dir, baseName
}

func initialize() {
	buf := bytes.NewBufferString(configFile)
	conf.SetConfigType("yaml")
	if err := conf.ReadConfig(buf); err != nil {
		panic(fmt.Errorf("failed to initialize flow config failed to read in config file: %w", err))
	}

	// create validator, at this point you can register custom validation funcs
	// struct tag translation etc.
	validate = validator.New()
	err := validate.RegisterValidation("ScoringRegistryDecayAdjustIntervalValidator", p2pconf.ScoringRegistryDecayAdjustIntervalValidator)
	if err != nil {
		panic(fmt.Errorf("failed to initialize flow config failed to register custom struct field validatior ScoringRegistryDecayAdjustIntervalValidator: %w", err))
	}
}
