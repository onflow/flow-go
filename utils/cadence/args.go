package cadence

import (
	"errors"

	"github.com/onflow/cadence"
	"github.com/onflow/cadence/encoding/json"
)

func EncodeArgs(argValues []cadence.Value) ([][]byte, error) {
	args := make([][]byte, len(argValues))
	for i, arg := range argValues {
		var err error
		args[i], err = json.Encode(arg)
		if err != nil {
			return nil, errors.New("couldn't encode cadence value: " + err.Error())
		}
	}
	return args, nil
}
