package testutils

import (
	"encoding/json"

	"github.com/onflow/flow-go/fvm/evm/debug"
)

type MockUploader struct {
	uploadFunc func(string, json.RawMessage) error
}

func (m MockUploader) Upload(id string, data json.RawMessage) error {
	return m.uploadFunc(id, data)
}

var _ debug.Uploader = &MockUploader{}
