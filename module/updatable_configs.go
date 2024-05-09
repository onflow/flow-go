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
