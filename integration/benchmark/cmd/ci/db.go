package main

import (
	"context"
	"fmt"

	"cloud.google.com/go/bigquery"
	"github.com/rs/zerolog"
)

type RawRecord struct {
	RepoInfo
	BenchmarkInfo
	BenchmarkResults
	Environment
}

type DB struct {
	client    *bigquery.Client
	projectID string

	log zerolog.Logger
}

func NewDB(ctx context.Context, log zerolog.Logger, projectID string) (*DB, error) {
	client, err := bigquery.NewClient(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to create bigquery client: %w", err)
	}
	return &DB{
		log:       log,
		client:    client,
		projectID: projectID,
	}, nil
}

func (db *DB) Close() error {
	return db.client.Close()
}

// createTable creates a table for raw TPS data if it does not exist.
func (db *DB) createTable(ctx context.Context, datasetID string, tableID string, st any) error {
	schema, err := bigquery.InferSchema(st)
	if err != nil {
		return fmt.Errorf("failed to infer schema: %w", err)
	}

	metaData := &bigquery.TableMetadata{Schema: schema}
	tableRef := db.client.Dataset(datasetID).Table(tableID)

	log := db.log.With().Str("table", tableRef.FullyQualifiedName()).Logger()

	// Check that table exists or create it.
	log.Info().Msg("Checking if table")
	if _, err := tableRef.Metadata(ctx); err != nil {
		log.Info().Msg("Creating table")
		if err := tableRef.Create(ctx, metaData); err != nil {
			return fmt.Errorf("table creation failed: %w", err)
		}
		log.Info().Msg("Created table")
	} else {
		log.Info().Msg("Table already exists")
	}
	return nil
}

func (db *DB) saveRawResults(
	ctx context.Context,
	datasetID, tableID string,
	results BenchmarkResults,
	repoInfo RepoInfo,
	benchmarkInfo BenchmarkInfo,
	env Environment,
) error {
	dataset := db.client.Dataset(datasetID)
	table := dataset.Table(tableID)

	if err := table.Inserter().Put(ctx, RawRecord{
		RepoInfo:         repoInfo,
		BenchmarkInfo:    benchmarkInfo,
		BenchmarkResults: results,
		Environment:      env,
	}); err != nil {
		return fmt.Errorf("failed to insert data: %w", err)
	}
	return nil
}
