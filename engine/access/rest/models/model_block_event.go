package models

import "time"

type BlockEvents struct {
	BlockId string `json:"block_id,omitempty"`

	BlockHeight string `json:"block_height,omitempty"`

	BlockTimestamp time.Time `json:"block_timestamp,omitempty"`

	Events []Event `json:"events,omitempty"`

	Links *Links `json:"_links,omitempty"`
}
