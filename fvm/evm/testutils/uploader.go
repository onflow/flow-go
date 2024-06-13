package testutils

import (
	"encoding/json"
)

type MockUploader struct {
	UploadFunc func(string, json.RawMessage) error
}

func (m MockUploader) Upload(id string, data json.RawMessage) error {
	return m.UploadFunc(id, data)
}
