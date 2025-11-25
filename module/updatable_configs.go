package module

// SealingConfigsGetter is an interface for the actual updatable configs module.
// but only exposes its getter methods to return the config values without exposing
// its setter methods.
// SealingConfigs contains three configs:
// - RequireApprovalsForSealingConstruction (updatable)
// - RequireApprovalsForSealingVerification (not-updatable)
// - ChunkAlpha (not-updatable)
// - EmergencySealingActive (not-updatable)
// - ApprovalRequestsThreshold (not-updatable)
type SealingConfigsGetter interface {
	// updatable fields
	RequireApprovalsForSealConstructionDynamicValue() uint

	// not-updatable fields
	ChunkAlphaConst() uint
	RequireApprovalsForSealVerificationConst() uint
	EmergencySealingActiveConst() bool
	ApprovalRequestsThresholdConst() uint64
}

// SealingConfigsSetter is an interface that allows the caller to update updatable configs
type SealingConfigsSetter interface {
	SealingConfigsGetter
	// SetRequiredApprovalsForSealingConstruction takes a new config value and updates the config
	// if the new value is valid.
	// Returns ValidationError if the new value results in an invalid sealing config.
	SetRequiredApprovalsForSealingConstruction(newVal uint) error
}

// ReadonlySealingLagRateLimiterConfig is an interface for the actual updatable configs module.
// but only exposes its getter methods to return the config values without exposing
// its setter methods.
// ReadonlySealingLagRateLimiterConfig contains several configs:
// - MinSealingLag (updatable)
// - MaxSealingLag (updatable)
// - HalvingInterval (updatable)
// - MinCollectionSize (updatable)
type ReadonlySealingLagRateLimiterConfig interface {
	// MinSealingLag is the minimum sealing lag that the rate limiter will allow.
	MinSealingLag() uint
	// MaxSealingLag is the maximum sealing lag that the rate limiter will allow.
	MaxSealingLag() uint
	// HalvingInterval is the interval in blocks in which the halving is applied.
	HalvingInterval() uint
	// MinCollectionSize is the minimum collection size that the rate limiter will allow.
	MinCollectionSize() uint
}

// SealingLagRateLimiterConfig is an interface that allows the caller to update updatable configs
type SealingLagRateLimiterConfig interface {
	ReadonlySealingLagRateLimiterConfig
	// SetMinSealingLag takes a new config value and updates the config
	// if the new value is valid.
	// Returns ValidationError if the new value results in an invalid config.
	SetMinSealingLag(value uint) error
	// SetMaxSealingLag takes a new config value and updates the config
	// if the new value is valid.
	// Returns ValidationError if the new value results in an invalid config.
	SetMaxSealingLag(value uint) error
	// SetHalvingInterval takes a new config value and updates the config
	// if the new value is valid.
	// Returns ValidationError if the new value results in an invalid config.
	SetHalvingInterval(value uint) error
	// SetMinCollectionSize takes a new config value and updates the config
	// if the new value is valid.
	// Returns ValidationError if the new value results in an invalid config.
	SetMinCollectionSize(value uint) error
}
