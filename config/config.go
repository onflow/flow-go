package config

import (
	"reflect"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func ParseConfig(envPrefix string, conf interface{}, flags *pflag.FlagSet) error {
	v := viper.New()

	v.SetEnvPrefix(envPrefix)

	bindEnv(v, conf)
	bindFlags(v, conf, flags)
	setDefaults(v, conf)

	v.AutomaticEnv()

	return v.Unmarshal(conf)
}

func bindEnv(v *viper.Viper, conf interface{}) {
	forEachStructField(conf, func(field reflect.StructField) {
		v.BindEnv(field.Name)
	})
}

func bindFlags(v *viper.Viper, conf interface{}, flags *pflag.FlagSet) {
	forEachStructField(conf, func(field reflect.StructField) {
		if flag, ok := field.Tag.Lookup("flag"); ok {
			v.BindPFlag(flag, flags.Lookup(flag))
		}
	})
}

func setDefaults(v *viper.Viper, conf interface{}) {
	forEachStructField(conf, func(field reflect.StructField) {
		if defaultValue, ok := field.Tag.Lookup("default"); ok {
			v.SetDefault(field.Name, defaultValue)
		}
	})
}

func forEachStructField(s interface{}, f func(reflect.StructField)) {
	t := reflect.TypeOf(s).Elem()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		f(field)
	}
}
