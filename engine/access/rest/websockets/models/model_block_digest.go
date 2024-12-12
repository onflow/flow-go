package models

import (
	"time"
)

// BlockDigest is a lightweight block information model.
type BlockDigest struct {
	BlockId   string    `json:"block_id"`
	Height    string    `json:"height"`
	Timestamp time.Time `json:"timestamp"`
}
