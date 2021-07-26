package utils

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/cadence"

	jsoncdc "github.com/onflow/cadence/encoding/json"
)

// EncodeArgs JSON encodes the cadence arguments to the format require by the `flow transactions build` command line.
// Example of output: https://app.zenhub.com/files/189500829/12503cd3-923e-40db-9f63-8e5bb96b6a99/download
func EncodeArgs(args []cadence.Value) ([]byte, error) {

	// will hold unmarshalled cadence JSON
	parsedArgs := make([]interface{}, len(args))

	for index, cdcVal := range args {

		// json encode cadence argument
		encoded, err := jsoncdc.Encode(cdcVal)
		if err != nil {
			return nil, fmt.Errorf("failed to encode cadence arguments: %w", err)
		}

		// unmarshal json to interface and append to array
		var arg interface{}
		err = json.Unmarshal(encoded, &arg)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal cadence arguments: %w", err)
		}
		parsedArgs[index] = arg
	}

	// encode array of parsed args to regular JSON
	encodedBytes, err := json.Marshal(parsedArgs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal interface: %w", err)
	}

	return encodedBytes, nil
}

// BytesToCadenceUInt8Array converts a `[]byte` into the cadence representation of `[Uint8]`
func BytesToCadenceUInt8Array(b []byte) cadence.Array {
	values := make([]cadence.Value, len(b))
	for i, v := range b {
		values[i] = cadence.NewUInt8(v)
	}
	return cadence.NewArray(values)
}
