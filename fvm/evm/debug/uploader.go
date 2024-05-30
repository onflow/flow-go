package debug

import (
	"context"
	"encoding/json"
	"time"

	"cloud.google.com/go/storage"
)

type Uploader struct {
	client *storage.Client
}

func (u *Uploader) Upload(data json.RawMessage) {
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

	}

	if err := wc.Close(); err != nil {

	}

	return nil
}
