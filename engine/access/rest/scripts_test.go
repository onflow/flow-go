package rest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
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

	t.Run("get by ID Latest", func(t *testing.T) {
		tests := []struct {
			height string
			id     string
		}{
			{height: "", id: "latest"},
			{height: "latest", id: ""},
		}

		for _, test := range tests {
			code := `pub fun main(foo: String): String { return foo }`

			body, _ := json.Marshal(map[string]interface{}{
				"script":    code,
				"arguments": []string{`{ "type": "String", "value": "hello world" }`},
			})

			req, _ := http.NewRequest("POST", scriptsURL(test.id, test.height), bytes.NewBuffer(body))

			backend.Mock.
				On("ExecuteScriptAtLatestBlock", mocks.Anything, []byte(code), [][]byte{[]byte(`{ "type": "String", "value": "hello world" }`)}).
				Return([]byte("hello world"), nil)

			rr := executeRequest(req, backend)

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Equal(t, fmt.Sprintf(
				"\"%s\"",
				base64.StdEncoding.EncodeToString([]byte(`hello world`)),
			), rr.Body.String())
		}
	})

	t.Run("get by height", func(t *testing.T) {
		code := `pub fun main(foo: String): String { return foo }`
		height := uint64(1337)
		body, _ := json.Marshal(map[string]interface{}{
			"script":    code,
			"arguments": []string{`{ "type": "String", "value": "hello world" }`},
		})

		req, _ := http.NewRequest("POST", scriptsURL("", fmt.Sprintf("%d", height)), bytes.NewBuffer(body))

		backend.Mock.
			On("ExecuteScriptAtBlockHeight", mocks.Anything, height, []byte(code), [][]byte{[]byte(`{ "type": "String", "value": "hello world" }`)}).
			Return([]byte("hello world"), nil)

		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, fmt.Sprintf(
			"\"%s\"",
			base64.StdEncoding.EncodeToString([]byte(`hello world`)),
		), rr.Body.String())
	})

	t.Run("get by ID", func(t *testing.T) {
		code := `pub fun main(foo: String): String { return foo }`
		id := "222dc5dd51b9e4910f687e475f892f495f3352362ba318b53e318b4d78131312"
		body, _ := json.Marshal(map[string]interface{}{
			"script":    code,
			"arguments": []string{`{ "type": "String", "value": "hello world" }`},
		})

		req, _ := http.NewRequest("POST", scriptsURL(id, ""), bytes.NewBuffer(body))

		flowID, _ := flow.HexStringToIdentifier(id)
		backend.Mock.
			On("ExecuteScriptAtBlockID", mocks.Anything, flowID, []byte(code), [][]byte{[]byte(`{ "type": "String", "value": "hello world" }`)}).
			Return([]byte("hello world"), nil)

		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, fmt.Sprintf(
			"\"%s\"",
			base64.StdEncoding.EncodeToString([]byte(`hello world`)),
		), rr.Body.String())
	})

	t.Run("get invalid", func(t *testing.T) {

		validBody, _ := json.Marshal(map[string]interface{}{
			"script":    `pub fun main(foo: String): String { return foo }`,
			"arguments": []string{`{ "type": "String", "value": "hello world" }`},
		})

		tests := []struct {
			id     string
			height string
			body   []byte
			out    string
			status int
		}{
			{"invalidID", "", validBody, `{"code":400,"message":"invalid ID format"}`, http.StatusBadRequest},
			{"", "invalid", validBody, `{"code":400,"message":"invalid height format"}`, http.StatusBadRequest},
			{"", "-1", validBody, `{"code":400,"message":"invalid height format"}`, http.StatusBadRequest},
			{"", "1337", nil, `{"code":400,"message":"invalid script execution request"}`, http.StatusBadRequest},
		}

		for _, test := range tests {
			req, _ := http.NewRequest("POST", scriptsURL(test.id, test.height), bytes.NewBuffer(test.body))
			rr := executeRequest(req, backend)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.Equal(t, test.out, rr.Body.String())
		}
	})
}
