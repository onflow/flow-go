package utils

import (
	"encoding/json"

	"github.com/onflow/cadence"
	jsoncdc "github.com/onflow/cadence/encoding/json"
)

type cadenceArgument struct {
	Value cadence.Value
}

func (v *cadenceArgument) MarshalJSON() ([]byte, error) {
	return jsoncdc.Encode(v.Value)
}

func (v *cadenceArgument) UnmarshalJSON(b []byte) (err error) {
	v.Value, err = jsoncdc.Decode(nil, b)
	if err != nil {
		return err
	}
	return nil
}

// ParseJSON parses string representing JSON array with Cadence arguments.
//
// Cadence arguments must be defined in the JSON-Cadence format https://developers.flow.com/cadence/json-cadence-spec
func ParseJSON(args []byte) ([]cadence.Value, error) {
	var arg []cadenceArgument
	err := json.Unmarshal(args, &arg)

	if err != nil {
		return nil, err
	}

	cadenceArgs := make([]cadence.Value, len(arg))
	for i, arg := range arg {
		cadenceArgs[i] = arg.Value
	}
	return cadenceArgs, nil
}
