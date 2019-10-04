package verify

import (
	"github.com/dapperlabs/flow-go/internal/roles/verify/config"
	"github.com/dapperlabs/flow-go/internal/roles/verify/processor"
	"github.com/dapperlabs/flow-go/pkg/crypto"
)

// NewReceiptProcessorConfig returns a new ReceiptProcessorConfig.
func NewReceiptProcessorConfig(c *config.Config) *processor.ReceiptProcessorConfig {
	return &processor.ReceiptProcessorConfig{
		QueueBuffer: c.ProcessorQueueBuffer,
		CacheBuffer: c.ProcessorCacheBuffer,
	}
}

// NewHasher return a new crypto.Hasher
// TODO: cast a config string to crypto.AlgoName
func NewHasher(c *config.Config) crypto.Hasher {
	hasher, _ := crypto.NewHasher(crypto.SHA3_256)
	return hasher
}
