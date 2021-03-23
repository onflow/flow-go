package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// GoogleBucket ...
type googleBucket struct {
	Name string
}

// NewGoogleBucket ...
func NewGoogleBucket(bucketName string) *googleBucket {
	return &googleBucket{
		Name: bucketName,
	}
}

// NewClient ...
func (g *googleBucket) NewClient(ctx context.Context) (*storage.Client, error) {
	client, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return nil, err
	}
	return client, nil
}

// GetFiles returns a list of file names within the Google bucket
func (g *googleBucket) GetFiles(ctx context.Context, client *storage.Client, prefix, delimiter string) ([]string, error) {
	it := client.Bucket(g.Name).Objects(ctx, &storage.Query{
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

		files = append(files, attrs.Name)
	}

	return files, nil
}

// DownloadFile downloads a file from the bucket to a desination folder
func (g *googleBucket) DownloadFile(ctx context.Context, client *storage.Client, destination, source string) error {

	// create dir of destination
	dir := filepath.Dir(destination)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("error creating destination directory: %w", err)
	}

	download, err := client.Bucket(g.Name).Object(source).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("error creating GCS object reader: %w", err)
	}
	defer download.Close()

	file, err := os.Create(destination)
	if err != nil {
		return fmt.Errorf("error creating download file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, download)
	if err != nil {
		return fmt.Errorf("error downloading file: %w", err)
	}
	return nil
}

// UploadFile uploads a file to the google bucket
func (g *googleBucket) UploadFile(ctx context.Context, client *storage.Client, destination, source string) error {

	upload := client.Bucket(g.Name).Object(destination).NewWriter(ctx)
	defer func() {
		err := upload.Close()
		if err != nil {
			log.Fatal().Err(err).Msg("failed to close writer stream")
		}
	}()

	file, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("Error opening upload file: %w", err)
	}
	defer file.Close()

	n, err := io.Copy(upload, file)
	if err != nil {
		return fmt.Errorf("Error uploading file: %w", err)
	}

	log.Info().Int64("byte_count", n).Msg("uploaded file")
	return nil
}
