package testutils

import (
	"encoding/json"

	"github.com/onflow/flow-go/fvm/evm/debug"
)

type MockUploader struct {
	UploadFunc func(string, json.RawMessage) error
}

func (m MockUploader) Upload(id string, data json.RawMessage) error {
	return m.UploadFunc(id, data)
}

var _ debug.Uploader = &MockUploader{}
