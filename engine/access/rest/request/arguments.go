package request

import (
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest"
)

const maxArgumentsLength = 100

type Arguments [][]byte

func (a *Arguments) Parse(raw []string) error {
	args := make([][]byte, len(raw))
	for i, a := range raw {
		arg, err := rest.FromBase64(a)
		if err != nil {
			return fmt.Errorf("invalid script encoding")
		}
		args[i] = arg
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
