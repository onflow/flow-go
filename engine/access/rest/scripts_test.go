package rest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/onflow/flow-go/access/mock"
	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"net/http"
	"net/url"
	"testing"
)

func scriptsURL(id string, height string) string {
	u, _ := url.ParseRequestURI("/v1/scripts")
	q := u.Query()

	if id != "" {
		q.Add("block_id", id)
	}
	if height != "" {
		q.Add("block_height", height)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func TestScripts(t *testing.T) {
	backend := &mock.API{}

	t.Run("get by ID", func(t *testing.T) {
		code := `pub fun main(foo: String): String { return foo }`
		args := []string{`{ "type": "String", "value": "hello world" }`}
		argsO := [][]byte{[]byte(`{ "type": "String", "value": "hello world" }`)}

		body, _ := json.Marshal(map[string]interface{}{
			"script":    "pub fun main(foo: String): String { return foo }",
			"arguments": args,
		})

		req, _ := http.NewRequest("POST", scriptsURL("", "latest"), bytes.NewBuffer(body))

		backend.Mock.
			On("ExecuteScriptAtLatestBlock", mocks.Anything, []byte(code), argsO).
			Return([]byte("hello world"), nil)

		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, fmt.Sprintf(
			"\"%s\"",
			base64.StdEncoding.EncodeToString([]byte(`hello world`)),
		), rr.Body.String())
	})
}
