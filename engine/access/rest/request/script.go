package request

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
	Args   Arguments
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

	var args Arguments
	err = args.Parse(body.Arguments)
	if err != nil {
		return err
	}

	s.Source = source
	s.Args = args

	return nil
}
