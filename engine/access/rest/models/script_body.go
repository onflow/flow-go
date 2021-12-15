package models

import (
	"fmt"
	"github.com/onflow/flow-go/engine/access/rest"
	"io"
)

type scriptBody struct {
	Script    string   `json:"script,omitempty"`
	Arguments []string `json:"arguments,omitempty"`
}

type Script struct {
	Args   [][]byte
	Source []byte
}

func (s *Script) Parse(raw io.Reader) error {
	var body scriptBody
	err := parseBody(raw, &body)
	if err != nil {
		return err
	}

	source, err := rest.FromBase64(body.Script)
	if err != nil {
		return fmt.Errorf("invalid script source encoding")
	}

	args := make([][]byte, len(body.Arguments))
	for i, a := range body.Arguments {
		arg, err := rest.FromBase64(a)
		if err != nil {
			return fmt.Errorf("invalid script encoding")
		}
		args[i] = arg
	}

	s.Source = source
	s.Args = args

	return nil
}
