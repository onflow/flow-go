// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/network/codec"

	_ "github.com/onflow/flow-go/utils/binstat"
)

func v2envEncode(v interface{}, via string) (*Envelope, error) {

	// determine the message type
	code, err := codec.MessageCodeFromV(v)
	if err != nil {
		return nil, fmt.Errorf("could not determine envelope code: %w", err)
	}

	what, err := code.String()
	if err != nil {
		return nil, fmt.Errorf("could not determine envelope code string: %w", err)
	}

	// encode the payload
	//bs := binstat.EnterTime(fmt.Sprintf("%s%s%s:%d", binstat.BinNet, via, what, code)) // e.g. ~3net::wire<1(json)CodeEntityRequest:23
	data, err := json.Marshal(v)
	//binstat.LeaveVal(bs, int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("could not encode json payload of type %s: %w", what, err)
	}

	env := Envelope{
		Code: code.Byte(),
		Data: data,
	}

	return &env, nil
}
