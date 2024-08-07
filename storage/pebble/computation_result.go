package pebble

import (
	"github.com/cockroachdb/pebble"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/storage/pebble/operation"
)

type ComputationResultUploadStatus struct {
	db *pebble.DB
}

func NewComputationResultUploadStatus(db *pebble.DB) *ComputationResultUploadStatus {
	return &ComputationResultUploadStatus{
		db: db,
	}
}

func (c *ComputationResultUploadStatus) Upsert(blockID flow.Identifier,
	wasUploadCompleted bool) error {
	return operation.UpsertComputationResultUploadStatus(blockID, wasUploadCompleted)(c.db)
}

func (c *ComputationResultUploadStatus) GetIDsByUploadStatus(targetUploadStatus bool) ([]flow.Identifier, error) {
	ids := make([]flow.Identifier, 0)
	err := operation.GetBlockIDsByStatus(&ids, targetUploadStatus)(c.db)
	return ids, err
}

func (c *ComputationResultUploadStatus) ByID(computationResultID flow.Identifier) (bool, error) {
	var ret bool
	err := operation.GetComputationResultUploadStatus(computationResultID, &ret)(c.db)
	if err != nil {
		return false, err
	}

	return ret, nil
}

func (c *ComputationResultUploadStatus) Remove(computationResultID flow.Identifier) error {
	return operation.RemoveComputationResultUploadStatus(computationResultID)(c.db)
}
