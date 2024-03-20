package p2pconfig

import "time"

const (
	SpamRecordCacheKey          = "spam-record-cache"
	ScoringRegistryKey          = "scoring-registry"
	AppSpecificScoreRegistryKey = "app-specific-score"
	StartupSilenceDurationKey   = "startup-silence-duration"
)

type ScoringRegistryParameters struct {
	// StartupSilenceDuration defines the duration of time, after the node startup,
	// during which the scoring registry remains inactive before penalizing nodes.
	// Throughout this startup silence period, the application-specific penalty
	// for all nodes will be set to 0, and any invalid control message notifications
	// will be ignored.
	//
	// This configuration allows nodes to stabilize and initialize before
	// applying penalties or responding processing invalid control message notifications.
	StartupSilenceDuration time.Duration              `validate:"gt=10m" mapstructure:"startup-silence-duration"`
	AppSpecificScore       AppSpecificScoreParameters `validate:"required" mapstructure:"app-specific-score"`
	SpamRecordCache        SpamRecordCacheParameters  `validate:"required" mapstructure:"spam-record-cache"`
	MisbehaviourPenalties  MisbehaviourPenalties      `validate:"required" mapstructure:"misbehaviour-penalties"`
}

const (
	ScoreUpdateWorkerNumKey                       = "score-update-worker-num"
	ScoreUpdateRequestQueueSizeKey                = "score-update-request-queue-size"
	ScoreTTLKey                                   = "score-ttl"
	InvalidControlMessageNotificationQueueSizeKey = "invalid-control-message-notification-queue-size"
)

// AppSpecificScoreParameters is the parameters for the GossipSubAppSpecificScoreRegistry.
// Parameters are "numerical values" that are used to compute or build components that compute or maintain the application specific score of peers.
type AppSpecificScoreParameters struct {
	// ScoreUpdateWorkerNum is the number of workers in the worker pool for handling the application specific score update of peers in a non-blocking way.
	ScoreUpdateWorkerNum int `validate:"gt=0" mapstructure:"score-update-worker-num"`

	// ScoreUpdateRequestQueueSize is the size of the worker pool for handling the application specific score update of peers in a non-blocking way.
	ScoreUpdateRequestQueueSize uint32 `validate:"gt=0" mapstructure:"score-update-request-queue-size"`

	// InvalidControlMessageNotificationQueueSize is the size of the queue for handling invalid control message notifications in a non-blocking way.
	InvalidControlMessageNotificationQueueSize uint32 `validate:"gt=0" mapstructure:"invalid-control-message-notification-queue-size"`

	// ScoreTTL is the time to live of the application specific score of a peer; the registry keeps a cached copy of the
	// application specific score of a peer for this duration. When the duration expires, the application specific score
	// of the peer is updated asynchronously. As long as the update is in progress, the cached copy of the application
	// specific score of the peer is used even if it is expired.
	ScoreTTL time.Duration `validate:"required" mapstructure:"score-ttl"`
}

const (
	DecayKey = "decay"
)

type SpamRecordCacheParameters struct {
	// CacheSize is size of the cache used to store the spam records of peers.
	// The spam records are used to penalize peers that send invalid messages.
	CacheSize uint32               `validate:"gt=0" mapstructure:"cache-size"`
	Decay     SpamRecordCacheDecay `validate:"required" mapstructure:"decay"`
}

const (
	PenaltyDecaySlowdownThresholdKey = "penalty-decay-slowdown-threshold"
	DecayRateReductionFactorKey      = "penalty-decay-rate-reduction-factor"
	PenaltyDecayEvaluationPeriodKey  = "penalty-decay-evaluation-period"
	MinimumSpamPenaltyDecayFactorKey = "minimum-spam-penalty-decay-factor"
	MaximumSpamPenaltyDecayFactorKey = "maximum-spam-penalty-decay-factor"
	SkipDecayThresholdKey            = "skip-decay-threshold"
)

type SpamRecordCacheDecay struct {
	// PenaltyDecaySlowdownThreshold defines the penalty level which the decay rate is reduced by `DecayRateReductionFactor` every time the penalty of a node falls below the threshold, thereby slowing down the decay process.
	// This mechanism ensures that malicious nodes experience longer decay periods, while honest nodes benefit from quicker decay.
	PenaltyDecaySlowdownThreshold float64 `validate:"lt=0" mapstructure:"penalty-decay-slowdown-threshold"`

	// DecayRateReductionFactor defines the value by which the decay rate is decreased every time the penalty is below the PenaltyDecaySlowdownThreshold. A reduced decay rate extends the time it takes for penalties to diminish.
	DecayRateReductionFactor float64 `validate:"gt=0,lt=1" mapstructure:"penalty-decay-rate-reduction-factor"`

	// PenaltyDecayEvaluationPeriod defines the interval at which the decay for a spam record is okay to be adjusted.
	PenaltyDecayEvaluationPeriod time.Duration `validate:"gt=0" mapstructure:"penalty-decay-evaluation-period"`

	SkipDecayThreshold float64 `validate:"gt=-1,lt=0" mapstructure:"skip-decay-threshold"`

	MinimumSpamPenaltyDecayFactor float64 `validate:"gt=0,lte=1" mapstructure:"minimum-spam-penalty-decay-factor"`
	MaximumSpamPenaltyDecayFactor float64 `validate:"gt=0,lte=1" mapstructure:"maximum-spam-penalty-decay-factor"`
}

const (
	MisbehaviourPenaltiesKey          = "misbehaviour-penalties"
	GraftKey                          = "graft"
	PruneKey                          = "prune"
	IWantKey                          = "iwant"
	IHaveKey                          = "ihave"
	PublishKey                        = "publish"
	ClusterPrefixedReductionFactorKey = "cluster-prefixed-reduction-factor"
)

type MisbehaviourPenalties struct {
	GraftMisbehaviour              float64 `validate:"lt=0" mapstructure:"graft"`
	PruneMisbehaviour              float64 `validate:"lt=0" mapstructure:"prune"`
	IHaveMisbehaviour              float64 `validate:"lt=0" mapstructure:"ihave"`
	IWantMisbehaviour              float64 `validate:"lt=0" mapstructure:"iwant"`
	PublishMisbehaviour            float64 `validate:"lt=0" mapstructure:"publish"`
	ClusterPrefixedReductionFactor float64 `validate:"gt=0,lte=1" mapstructure:"cluster-prefixed-reduction-factor"`
}
