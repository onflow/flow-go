package models

import (
	"time"
)

type BlockDigest struct {
	BlockId   string    `json:"block_id"`
	Height    string    `json:"height"`
	Timestamp time.Time `json:"timestamp"`
}
