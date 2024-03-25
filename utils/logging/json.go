package logging

import (
	"encoding/json"
	"fmt"
)

func AsJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("could not encode as JSON: %s", err))
	}
	return data
}
