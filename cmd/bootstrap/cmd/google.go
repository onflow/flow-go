package cmd

import (
	"context"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

type GoogleBucket struct {
	name string
}

func NewGoogleBucket(bucketName string) *GoogleBucket {
	return &GoogleBucket{
		name: bucketName,
	}
}

func (g *Google) NewClient(ctx context.Context) (*storage.Client, error) {
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (g *GoogleBucket) GetFiles(ctx context.Context, client *storage.Client, prefix, delimiter string) ([]string, error) {
	it := client.Bucket(g.name).Objects(ctx, &storage.Query{
		Prefix:    prefix,
		Delimiter: delimiter,
	})

	var files []string
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		if strings.Contains(attrs.Name, "node-info.pub") {
			files = append(files, attrs.Name)
		}
	}

	return files, nil
}
