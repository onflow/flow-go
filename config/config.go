package config

import (
	"reflect"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func ParseConfig(envPrefix string, conf interface{}, cmd *cobra.Command) error {
	v := viper.New()

	v.SetEnvPrefix(envPrefix)

	bindPFlags(v, cmd, conf)
	setDefaults(v, conf)

	v.AutomaticEnv()

	return v.Unmarshal(conf)
}

func setDefaults(v *viper.Viper, conf interface{}) {
	t := reflect.TypeOf(conf).Elem()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if defaultValue, ok := field.Tag.Lookup("default"); ok {
			v.SetDefault(field.Name, defaultValue)
		}
	}
}

func bindPFlags(v *viper.Viper, cmd *cobra.Command, conf interface{}) {
	t := reflect.TypeOf(conf).Elem()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		if flag, ok := field.Tag.Lookup("flag"); ok {
			v.BindPFlag(flag, cmd.PersistentFlags().Lookup(flag))
		}
	}
}
