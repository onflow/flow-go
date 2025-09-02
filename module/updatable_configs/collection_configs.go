package updatable_configs

import (
	"go.uber.org/atomic"

	"github.com/onflow/flow-go/module"
	"github.com/onflow/flow-go/module/updatable_configs/validation"
)

type bySealingLagRateLimiterConfigs struct {
	minSealingLag     *atomic.Uint32
	maxSealingLag     *atomic.Uint32
	halvingInterval   *atomic.Uint32
	minCollectionSize *atomic.Uint32
}

var _ module.SealingLagRateLimiterConfig = (*bySealingLagRateLimiterConfigs)(nil)

// DefaultBySealingLagRateLimiterConfigs returns a default config for collection throttling.
// It performs binary throttling once the sealing lag reaches max sealing lag.
func DefaultBySealingLagRateLimiterConfigs() module.SealingLagRateLimiterConfig {
	// default config results in binary throttling once the sealing lag reaches max sealing lag, before that no throttling
	// is being applied. The 600 blocks is chosen as it is roughly 5 minutes.
	// 2 blocks / second * 60 seconds * 5 minutes = 600 blocks
	return &bySealingLagRateLimiterConfigs{
		minSealingLag:     atomic.NewUint32(300),
		maxSealingLag:     atomic.NewUint32(600),
		halvingInterval:   atomic.NewUint32(300),
		minCollectionSize: atomic.NewUint32(0),
	}
}

func (c *bySealingLagRateLimiterConfigs) MinSealingLag() uint {
	return uint(c.minSealingLag.Load())
}

func (c *bySealingLagRateLimiterConfigs) MaxSealingLag() uint {
	return uint(c.maxSealingLag.Load())
}

func (c *bySealingLagRateLimiterConfigs) HalvingInterval() uint {
	return uint(c.halvingInterval.Load())
}

func (c *bySealingLagRateLimiterConfigs) MinCollectionSize() uint {
	return uint(c.minCollectionSize.Load())
}

func (c *bySealingLagRateLimiterConfigs) SetMinSealingLag(value uint) error {
	if err := validation.ValidateMinMaxSealingLag(value, c.MaxSealingLag()); err != nil {
		return err
	}
	c.minSealingLag.Store(uint32(value))
	return nil
}

func (c *bySealingLagRateLimiterConfigs) SetMaxSealingLag(value uint) error {
	if err := validation.ValidateMinMaxSealingLag(c.MinSealingLag(), value); err != nil {
		return err
	}
	c.maxSealingLag.Store(uint32(value))
	return nil
}

func (c *bySealingLagRateLimiterConfigs) SetHalvingInterval(value uint) error {
	if err := validation.ValidateHalvingInterval(value); err != nil {
		return err
	}
	c.halvingInterval.Store(uint32(value))
	return nil
}

func (c *bySealingLagRateLimiterConfigs) SetMinCollectionSize(value uint) error {
	c.minCollectionSize.Store(uint32(value))
	return nil
}
