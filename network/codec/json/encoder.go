// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package json

import (
	"encoding/json"
	"fmt"

	"github.com/onflow/flow-go/utils/binstat"
)

// Encoder is an encoder to write serialized JSON to a writer.
type Encoder struct {
	enc *json.Encoder
}

// Encode will convert the given message into binary JSON and write it to the
// underlying encoder, followed by a new line.
func (e *Encoder) Encode(v interface{}) error {

	// encode the value
	env, err := v2envEncode(v, "~3net:strm<1")
	if err != nil {
		return fmt.Errorf("could not encode value: %w", err)
	}

	// write the envelope to network
	bs := binstat.EnterTimeVal("~3net:strm<2", int64(len(env.Data)))
	bs.Run(func() {
		err = e.enc.Encode(env)
	})
	bs.Leave()
	if err != nil {
		return fmt.Errorf("could not encode envelope: %w", err)
	}

	return nil
}
