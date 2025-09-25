package request

import (
	"fmt"
	"io"

	"github.com/onflow/flow-go/engine/access/api/rest/common"
	"github.com/onflow/flow-go/engine/access/api/rest/common/parser"
	"github.com/onflow/flow-go/engine/access/api/rest/util"
)

type scriptBody struct {
	Script    string   `json:"script,omitempty"`
	Arguments []string `json:"arguments,omitempty"`
}

type Script struct {
	Args   parser.Arguments
	Source []byte
}

func (s *Script) Parse(raw io.Reader) error {
	var body scriptBody
	err := common.ParseBody(raw, &body)
	if err != nil {
		return err
	}

	source, err := util.FromBase64(body.Script)
	if err != nil {
		return fmt.Errorf("invalid script source encoding")
	}

	var args parser.Arguments
	err = args.Parse(body.Arguments)
	if err != nil {
		return err
	}

	s.Source = source
	s.Args = args

	return nil
}
