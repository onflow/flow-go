package debug

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/storage"
)

type Uploader interface {
	Upload(data json.RawMessage) error
}

var _ Uploader = &GCPUploader{}

func NewGCPUploader() *GCPUploader {
	return &GCPUploader{}
}

type GCPUploader struct {
	client *storage.Client
}

func (g *GCPUploader) Upload(data json.RawMessage) error {
	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {

	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(ctx, time.Second*50)
	defer cancel()

	wc := client.
		Bucket("evm-tx-traces").
		Object("tx-id1").
		NewWriter(ctx)

	if _, err = wc.Write(data); err != nil {
		return err
	}

	if err := wc.Close(); err != nil {
		return err
	}

	return nil
}
