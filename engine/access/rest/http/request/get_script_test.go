package request

import (
	"fmt"
	"strings"
	"testing"

	"github.com/onflow/flow-go/engine/access/rest/util"

	"github.com/stretchr/testify/assert"
)

func TestGetScript_InvalidParse(t *testing.T) {
	var getScript GetScript

	validScript := fmt.Sprintf(`{ "script": "%s", "arguments": [] }`, util.ToBase64([]byte(`access(all) fun main() {}`)))
	tests := []struct {
		height string
		id     string
		script string
		err    string
	}{
		{"", "", "", "request body must not be empty"},
		{"1", "7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7", validScript, "can not provide both block ID and block height"},
		{"final", "7bc42fe85d32ca513769a74f97f7e1a7bad6c9407f0d934c2aa645ef9cf613c7", validScript, "can not provide both block ID and block height"},
		{"", "2", validScript, "invalid ID format"},
		{"1", "", `{ "foo": "zoo" }`, `request body contains unknown field "foo"`},
	}

	for i, test := range tests {
		err := getScript.Parse(test.height, test.id, strings.NewReader(test.script))
		assert.EqualError(t, err, test.err, fmt.Sprintf("test #%d failed", i))
	}
}

func TestGetScript_ValidParse(t *testing.T) {
	var getScript GetScript

	source := "access(all) fun main() {}"
	validScript := strings.NewReader(fmt.Sprintf(`{ "script": "%s", "arguments": [] }`, util.ToBase64([]byte(source))))

	err := getScript.Parse("1", "", validScript)
	assert.NoError(t, err)
	assert.Equal(t, getScript.BlockHeight, uint64(1))
	assert.Equal(t, string(getScript.Script.Source), source)

	validScript1 := strings.NewReader(fmt.Sprintf(`{ "script": "%s", "arguments": [] }`, util.ToBase64([]byte(source))))
	err = getScript.Parse("", "", validScript1)
	assert.NoError(t, err)
	assert.Equal(t, getScript.BlockHeight, SealedHeight)
}
