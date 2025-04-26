package store

import (
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage"
	"github.com/onflow/flow-go/storage/operation"
)

type ComputationResultUploadStatus struct {
	db storage.DB
}

func NewComputationResultUploadStatus(db storage.DB) *ComputationResultUploadStatus {
	return &ComputationResultUploadStatus{
		db: db,
	}
}

func (c *ComputationResultUploadStatus) Upsert(blockID flow.Identifier,
	wasUploadCompleted bool) error {
	return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.UpsertComputationResultUploadStatus(rw.Writer(), blockID, wasUploadCompleted)
	})
}

func (c *ComputationResultUploadStatus) GetIDsByUploadStatus(targetUploadStatus bool) ([]flow.Identifier, error) {
	reader, err := c.db.Reader()
	if err != nil {
		return nil, err
	}

	ids := make([]flow.Identifier, 0)
	err = operation.GetBlockIDsByStatus(reader, &ids, targetUploadStatus)
	return ids, err
}

func (c *ComputationResultUploadStatus) ByID(computationResultID flow.Identifier) (bool, error) {
	reader, err := c.db.Reader()
	if err != nil {
		return false, err
	}

	var ret bool
	err = operation.GetComputationResultUploadStatus(reader, computationResultID, &ret)
	if err != nil {
		return false, err
	}

	return ret, nil
}

func (c *ComputationResultUploadStatus) Remove(computationResultID flow.Identifier) error {
	return c.db.WithReaderBatchWriter(func(rw storage.ReaderBatchWriter) error {
		return operation.RemoveComputationResultUploadStatus(rw.Writer(), computationResultID)
	})
}
