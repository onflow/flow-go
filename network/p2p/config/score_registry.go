package p2pconfig

const (
	ScoringRegistryKey               = "scoring-registry"
	MinimumSpamPenaltyDecayFactorKey = "minimum-spam-penalty-decay-factor"
	MaximumSpamPenaltyDecayFactorKey = "maximum-spam-penalty-decay-factor"
)

type ScoringRegistryParameters struct {
	MinimumSpamPenaltyDecayFactor float64               `validate:"gt=0,lte=1" mapstructure:"minimum-spam-penalty-decay-factor"`
	MaximumSpamPenaltyDecayFactor float64               `validate:"gt=0,lte=1" mapstructure:"maximum-spam-penalty-decay-factor"`
	MisbehaviourPenalties         MisbehaviourPenalties `validate:"required" mapstructure:"misbehaviour-penalties"`
}

const (
	MisbehaviourPenaltiesKey          = "misbehaviour-penalties"
	SkipDecayThresholdKey             = "skip-decay-threshold"
	GraftMisbehaviourKey              = "graft"
	PruneMisbehaviourKey              = "prune"
	IHaveMisbehaviourKey              = "ihave"
	IWantMisbehaviourKey              = "iwant"
	PublishMisbehaviourKey            = "publish"
	ClusterPrefixedReductionFactorKey = "cluster-prefixed-reduction-factor"
)

type MisbehaviourPenalties struct {
	SkipDecayThreshold             float64 `validate:"gt=-1,lt=0" mapstructure:"skip-decay-threshold"`
	GraftMisbehaviour              float64 `validate:"lt=0" mapstructure:"graft"`
	PruneMisbehaviour              float64 `validate:"lt=0" mapstructure:"prune"`
	IHaveMisbehaviour              float64 `validate:"lt=0" mapstructure:"ihave"`
	IWantMisbehaviour              float64 `validate:"lt=0" mapstructure:"iwant"`
	PublishMisbehaviour            float64 `validate:"lt=0" mapstructure:"publish"`
	ClusterPrefixedReductionFactor float64 `validate:"gt=0,lte=1" mapstructure:"cluster-prefixed-reduction-factor"`
}
