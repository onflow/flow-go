package rest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
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

	validCode := []byte(`pub fun main(foo: String): String { return foo }`)
	validArgs := []byte(`{ "type": "String", "value": "hello world" }`)
	validBody, _ := json.Marshal(map[string]interface{}{
		"script":    toBase64(validCode),
		"arguments": []string{toBase64(validArgs)},
	})

	t.Run("get by Latest height", func(t *testing.T) {
		req, _ := http.NewRequest("POST", scriptsURL("", "latest"), bytes.NewBuffer(validBody))

		backend.Mock.
			On("ExecuteScriptAtLatestBlock", mocks.Anything, validCode, [][]byte{validArgs}).
			Return([]byte("hello world"), nil)

		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, fmt.Sprintf(
			"\"%s\"",
			base64.StdEncoding.EncodeToString([]byte(`hello world`)),
		), rr.Body.String())
	})

	t.Run("get by height", func(t *testing.T) {
		height := uint64(1337)
		req, _ := http.NewRequest("POST", scriptsURL("", fmt.Sprintf("%d", height)), bytes.NewBuffer(validBody))

		backend.Mock.
			On("ExecuteScriptAtBlockHeight", mocks.Anything, height, validCode, [][]byte{validArgs}).
			Return([]byte("hello world"), nil)

		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, fmt.Sprintf(
			"\"%s\"",
			base64.StdEncoding.EncodeToString([]byte(`hello world`)),
		), rr.Body.String())
	})

	t.Run("get by ID", func(t *testing.T) {
		id := "222dc5dd51b9e4910f687e475f892f495f3352362ba318b53e318b4d78131312"
		req, _ := http.NewRequest("POST", scriptsURL(id, ""), bytes.NewBuffer(validBody))

		flowID, _ := flow.HexStringToIdentifier(id)
		backend.Mock.
			On("ExecuteScriptAtBlockID", mocks.Anything, flowID, validCode, [][]byte{validArgs}).
			Return([]byte("hello world"), nil)

		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, fmt.Sprintf(
			"\"%s\"",
			base64.StdEncoding.EncodeToString([]byte(`hello world`)),
		), rr.Body.String())
	})

	t.Run("get error", func(t *testing.T) {
		req, _ := http.NewRequest("POST", scriptsURL("", "1337"), bytes.NewBuffer(validBody))

		backend = &mock.API{} // todo quick fix something is not remove on mock - solve tomorrow
		backend.Mock.
			On("ExecuteScriptAtBlockHeight", mocks.Anything, uint64(1337), validCode, [][]byte{validArgs}).
			Return(nil, status.Error(codes.Internal, "internal server error"))

		rr := executeRequest(req, backend)

		assert.Equal(t, http.StatusInternalServerError, rr.Code)
		assert.JSONEq(t, `{"code":500, "message":"rpc error: code = Internal desc = internal server error"}`, rr.Body.String())
	})

	t.Run("get invalid", func(t *testing.T) {
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
			{"", "1337", nil, `{"code":400,"message":"request body must not be empty"}`, http.StatusBadRequest},
		}

		for _, test := range tests {
			req, _ := http.NewRequest("POST", scriptsURL(test.id, test.height), bytes.NewBuffer(test.body))
			rr := executeRequest(req, backend)

			assert.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, test.out, rr.Body.String(), fmt.Sprintf("test details: %v", test))
		}
	})
}
