package rest

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	mocks "github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/model/flow"
)

func scriptReq(id string, height string, body interface{}) *http.Request {
	u, _ := url.ParseRequestURI("/v1/scripts")
	q := u.Query()

	if id != "" {
		q.Add("block_id", id)
	}
	if height != "" {
		q.Add("block_height", height)
	}

	u.RawQuery = q.Encode()

	jsonBody, _ := json.Marshal(body)
	req, _ := http.NewRequest("POST", u.String(), bytes.NewBuffer(jsonBody))

	return req
}

func TestScripts(t *testing.T) {
	validCode := []byte(`pub fun main(foo: String): String { return foo }`)
	validArgs := []byte(`{ "type": "String", "value": "hello world" }`)
	validBody := map[string]interface{}{
		"script":    toBase64(validCode),
		"arguments": []string{toBase64(validArgs)},
	}

	t.Run("get by Latest height", func(t *testing.T) {
		backend := &mock.API{}
		backend.Mock.
			On("ExecuteScriptAtLatestBlock", mocks.Anything, validCode, [][]byte{validArgs}).
			Return([]byte("hello world"), nil)

		req := scriptReq("", sealedHeightQueryParam, validBody)
		assertOKResponse(t, req, fmt.Sprintf(
			"\"%s\"",
			base64.StdEncoding.EncodeToString([]byte(`hello world`)),
		), backend)
	})

	t.Run("get by height", func(t *testing.T) {
		backend := &mock.API{}
		height := uint64(1337)

		backend.Mock.
			On("ExecuteScriptAtBlockHeight", mocks.Anything, height, validCode, [][]byte{validArgs}).
			Return([]byte("hello world"), nil)

		req := scriptReq("", fmt.Sprintf("%d", height), validBody)
		assertOKResponse(t, req, fmt.Sprintf(
			"\"%s\"",
			base64.StdEncoding.EncodeToString([]byte(`hello world`)),
		), backend)
	})

	t.Run("get by ID", func(t *testing.T) {
		backend := &mock.API{}
		id, _ := flow.HexStringToIdentifier("222dc5dd51b9e4910f687e475f892f495f3352362ba318b53e318b4d78131312")

		backend.Mock.
			On("ExecuteScriptAtBlockID", mocks.Anything, id, validCode, [][]byte{validArgs}).
			Return([]byte("hello world"), nil)

		req := scriptReq(id.String(), "", validBody)
		assertOKResponse(t, req, fmt.Sprintf(
			"\"%s\"",
			base64.StdEncoding.EncodeToString([]byte(`hello world`)),
		), backend)
	})

	t.Run("get error", func(t *testing.T) {
		backend := &mock.API{}
		backend.Mock.
			On("ExecuteScriptAtBlockHeight", mocks.Anything, uint64(1337), validCode, [][]byte{validArgs}).
			Return(nil, status.Error(codes.Internal, "internal server error"))

		req := scriptReq("", "1337", validBody)
		assertResponse(
			t,
			req,
			http.StatusInternalServerError,
			`{"code":500, "message":"internal server error"}`,
			backend,
		)
	})

	t.Run("get invalid", func(t *testing.T) {
		backend := &mock.API{}
		backend.Mock.
			On("ExecuteScriptAtBlockHeight", mocks.Anything, mocks.Anything, mocks.Anything, mocks.Anything).
			Return(nil, nil)

		tests := []struct {
			id     string
			height string
			body   map[string]interface{}
			out    string
			status int
		}{
			{"invalidID", "", validBody, `{"code":400,"message":"invalid ID format"}`, http.StatusBadRequest},
			{"", "invalid", validBody, `{"code":400,"message":"invalid height format"}`, http.StatusBadRequest},
			{"", "-1", validBody, `{"code":400,"message":"invalid height format"}`, http.StatusBadRequest},
			{"", "1337", nil, `{"code":400,"message":"request body must not be empty"}`, http.StatusBadRequest},
		}

		for _, test := range tests {
			req := scriptReq(test.id, test.height, test.body)
			assertResponse(t, req, http.StatusBadRequest, test.out, backend)
		}
	})
}
