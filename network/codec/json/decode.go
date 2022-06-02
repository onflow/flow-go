// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/network/codec"

	_ "github.com/onflow/flow-go/utils/binstat"
)

// decode will decode the envelope into an entity.
func env2vDecode(env Envelope, via string) (interface{}, error) {

	// create the desired message
	code, err := codec.MessageCodeFromByte(env.Code)
	if err != nil {
		return nil, fmt.Errorf("could not determine envelope code: %w", err)
	}

	what, v, err := code.Message()
	if err != nil {
		return nil, fmt.Errorf("could not determine envelope code string: %w", err)
	}

	// unmarshal the payload
	//bs := binstat.EnterTimeVal(fmt.Sprintf("%s%s%s:%d", binstat.BinNet, via, what, env.Code), int64(len(env.Data))) // e.g. ~3net:wire>4(json)CodeEntityRequest:23
	err = json.Unmarshal(env.Data, v)
	//binstat.Leave(bs)
	if err != nil {
		return nil, fmt.Errorf("could not decode json payload of type %s: %w", what, err)
	}

	return v, nil
}
