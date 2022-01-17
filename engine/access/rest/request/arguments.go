package request

import (
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest/util"
)

const maxArgumentsLength = 100

type Arguments [][]byte

func (a *Arguments) Parse(raw []string) error {
	args := make([][]byte, 0)
	for _, rawArg := range raw {
		if rawArg == "" { // skip empty
			continue
		}

		arg, err := util.FromBase64(rawArg)
		if err != nil {
			return fmt.Errorf("invalid argument encoding: %v", err)
		}
		args = append(args, arg)
	}

	if len(args) > maxArgumentsLength {
		return fmt.Errorf("too many arguments. Maximum arguments allowed: %d", maxAllowedScriptArguments)
	}

	*a = args
	return nil
}

func (a Arguments) Flow() [][]byte {
	return a
}
