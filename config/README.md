## config 
config is a package to hold all configuration values for each Flow component. This package centralizes configuration management providing access 
to the entire FlowConfig and utilities to add a new config value, corresponding CLI flag, and validation.

### Package structure
The root config package contains the FlowConfig struct and the default config file [default-config.yml](https://github.com/onflow/flow-go/blob/master/config/default-config.yml). The `default-config.yml` file is the default configuration that is loaded when the config package is initialized.
The `default-config.yml` is a snapshot of all the configuration values defined for Flow.
Each subpackage contains configuration structs and utilities for components and their related subcomponents. These packages also contain the CLI flags for each configuration value. The [netconf](https://github.com/onflow/flow-go/tree/master/network/netconf) package
is a good example of this pattern. The network component is a large component made of many other large components and subcomponents. Each configuration 
struct is defined for all of these network related components in the netconf subpackage and CLI flags. 

### Overriding default values
The entire default config can be overridden using the `--config-file` CLI flag. When set the config package will attempt to parse the specified config file and override all the values 
defined. A single default value can be overridden by setting the CLI flag for that specific config. For example `--networking-connection-pruning=false` will override the default network connection pruning 
config to false.
Override entire config file.
```shell
go build -tags relic -o flow-access-node ./cmd/access
./flow-access-node --config-file=config/config.yml
```
Override a single configuration value.
```shell
go build -tags relic -o flow-access-node ./cmd/access
./flow-access-node --networking-connection-pruning=false
```
### Adding a new config value
Adding a new config to the FlowConfig can be done in a few easy steps.

The network package can be used as a good example of how to add CLI flags and will be used in the steps below.

1. Create a new configuration package for the new configuration structs and CLI flags. Although it is encouraged to define all package configuration structs and CLI flags as a subpackage of the [config package](https://github.com/onflow/flow-go/tree/master/config),
the configuration package can live anywhere. This configuration package will define the configuration structs and CLI flags for overriding.
    ```shell
    mkdir example_config 
    ```
    For the network package we have a configuration subpackage created in [network/netconf](https://github.com/onflow/flow-go/tree/master/network/netconf). 

2. Add a new CLI flag for the config value. 
    ```go
    const workersCLIFlag = "app-workers"
    flags.String(workersCLIFlag, 1, "number of app workers")
    ```

   All network package CLI flags are defined in [network/netconf/flags.go](https://github.com/onflow/flow-go/blob/master/network/netconf/flags.go) in:
    - `const` block
    - `AllFlagNames` function
    - `InitializeNetworkFlags` function

    `InitializeNetworkFlags` is used during initialization of all flags 
    in the `InitializePFlagSet` function in the [config/base_flags.go](https://github.com/onflow/flow-go/blob/master/config/base_flags.go).

3. Add the configuration as a new field to an existing configuration struct or create a new configuration struct. Each configuration struct must be a field on the FlowConfig struct so that it is unmarshalled during configuration initialization.
    Each field on a configuration struct must contain the following field tags.
   1. `validate` - validate tag is used to perform validation on field structs using the [validator](https://github.com/go-playground/validator) package. In the example below you will notice 
   the `validate:"gt=0"` tag, this will ensure that the value of `AppWorkers` is greater than 0. The top level `FlowConfig` struct has a Validate method that performs struct validation. This 
   validation is done with the validator package, each validate tag on every struct field and sub struct field will be validated and validation errors are returned.
   2. `mapstructure` - [mapstructure](https://github.com/mitchellh/mapstructure) tag is used for unmarshalling and must match the CLI flag name defined in step or else the field will not be set when the config is unmarshalled.
   ```go
        type MyComponentConfig struct {
            AppWorkers int `validate:"gt=0" mapstructure:"app-workers"`
        }
    ```
   It's important to make sure that each mapstructure field tag matches the CLI flag name in [config/default-config.yml](https://github.com/onflow/flow-go/blob/master/config/default-config.yml) to avoid parsing errors.
   
   All network package configuration structs are defined in [network/netconf/config.go](https://github.com/onflow/flow-go/blob/master/network/netconf/config.go)

4. Add the new configuration properties and default values to [config/default-config.yml](https://github.com/onflow/flow-go/blob/master/config/default-config.yml). Ensure that each new property added matches the mapstructure value of the configuration struct field.
    ```yaml
      config-file: "./default-config.yml"
      network-config:
      ...
      my-component:
        app-workers: 1
    ```
   
    All network package configuration values are defined under `network-config` in `default-config.yml`

5. If a new struct was created in step 3, add it as a new field to `FlowConfig` struct in [config/config.go](https://github.com/onflow/flow-go/blob/master/config/config.go). In the previous steps we added a new config struct and added a new property to [config/default-config.yml](https://github.com/onflow/flow-go/blob/master/config/default-config.yml) for the `my-component` struct. This property name
    must match the mapstructure field tag for the struct when added to `FlowConfig`.
    ```go
    // FlowConfig Flow configuration.
    type FlowConfig struct {
        ConfigFile    string          `validate:"filepath" mapstructure:"config-file"`
        NetworkConfig *network.Config `mapstructure:"network-config"`
        MyComponentConfig *mypackage.MyComponentConfig `mapstructure:"my-component"`
    }
    ```
   
    The network package configuration struct, `NetworkConfig`, is already embedded as a field in `FlowConfig` struct.
    This means that new fields can be added to the network package configuration struct without having to update the `FlowConfig` struct.

### Nested structs
In an effort to keep the configuration yaml structure readable some configuration properties will be defined in nested structs. When this is the case the mapstructure [squash](https://pkg.go.dev/github.com/mitchellh/mapstructure#example-Decode-EmbeddedStruct) tag must be used in the parent struct so that the corresponding nested struct will be 
flattened before the configuration is unmarshalled. This is used in the network package which is a collection of configuration structs nested in the `network.Config` struct:
```go
type Config struct {
    UnicastConfig                 `mapstructure:",squash"`
    p2pconf.ResourceManagerConfig `mapstructure:",squash"`
    ConnectionManagerConfig       `mapstructure:",squash"`
    
    p2pconf.GossipSubConfig `mapstructure:",squash"`
    AlspConfig              `mapstructure:",squash"`
    ...
}
```
Each nested struct must have the mapstructure squash tag so that the nested struct will be flattened before unmarshalling.
The nesting can be as deep as needed. For example, the `UnicastConfig` struct in the `Config` struct has a nested `UnicastRateLimitersConfig` struct that also uses the `squash` tag:

```go
type UnicastConfig struct {
    // UnicastRateLimitersConfig configuration for all unicast rate limiters.
    UnicastRateLimitersConfig `mapstructure:",squash"`
    ...
}
```

### Setting Aliases
Most configs will not be defined on the top layer FlowConfig but instead be defined on nested structs and in nested properties of the configuration yaml. When the default config is initially loaded the underlying config [viper](https://github.com/spf13/viper) store will store 
each configuration with a key that is prefixed with each parent property. For example, because `networking-connection-pruning` is found on the `network-config` property of the configuration yaml, the key used by the config store to 
store this config value will be prefixed with `network-config` e.g.
```network-config.networking-connection-pruning```

Later in the config process we bind the underlying config store with our pflag set, this allows us to override default values using CLI flags.
At this time the underlying config store would have 2 separate keys `networking-connection-pruning` and `network-config.networking-connection-pruning` for the same configuration value. This is because we don't use the network prefix for the CLI flags
used to override network configs. As a result, an alias must be set from `network-config.networking-connection-pruning` -> `networking-connection-pruning` so that they both point to the value loaded from the CLI flag. See `SetAliases` in [network/netconf/flags.go](https://github.com/onflow/flow-go/blob/master/config/network/netconf/flags.go) in the network package for a reference. 
