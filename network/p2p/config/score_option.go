package p2pconfig

import "time"

const (
	ScoreOptionKey              = "internal-peer-scoring"
	AppScoreWeightKey           = "app-specific-score-weight"
	MaxDebugLogsKey             = "max-debug-logs"
	ScoreOptionDecayIntervalKey = "decay-interval"
	ScoreOptionDecayToZeroKey   = "decay-to-zero"
	ScoreOptionPenaltiesKey     = "penalties"
	ScoreOptionRewardsKey       = "rewards"
	ScoreOptionThresholdsKey    = "thresholds"
	ScoreOptionBehaviourKey     = "behaviour"
	ScoreOptionTopicKey         = "topic"
)

// InternalPeerScoring gossipsub scoring option configuration parameters.
type InternalPeerScoring struct {
	// AppSpecificScoreWeight is the  weight for app-specific scores. It is used to scale the app-specific
	// scores to the same range as the other scores. At the current version, we don't distinguish between the app-specific
	// scores and the other scores, so we set it to 1.
	AppSpecificScoreWeight float64 `validate:"gt=0,lte=1" mapstructure:"app-specific-score-weight"`
	// MaxDebugLogs sets the max number of debug/trace log events per second. Logs emitted above
	// this threshold are dropped.
	MaxDebugLogs uint32 `validate:"lte=50" mapstructure:"max-debug-logs"`
	// DecayInterval is the  decay interval for the overall score of a peer at the GossipSub scoring
	// system. We set it to 1 minute so that it is not too short so that a malicious node can recover from a penalty
	// and is not too long so that a well-behaved node can't recover from a penalty.
	DecayInterval time.Duration `validate:"gte=1m" mapstructure:"decay-interval"`
	// DecayToZero is the  decay to zero for the overall score of a peer at the GossipSub scoring system.
	// It defines the maximum value below which a peer scoring counter is reset to zero.
	// This is to prevent the counter from decaying to a very small value.
	// The  value is 0.01, which means that a counter will be reset to zero if it decays to 0.01.
	// When a counter hits the DecayToZero threshold, it means that the peer did not exhibit the behavior
	// for a long time, and we can reset the counter.
	DecayToZero     float64                    `validate:"required" mapstructure:"decay-to-zero"`
	Penalties       ScoreOptionPenalties       `validate:"required" mapstructure:"penalties"`
	Rewards         ScoreOptionRewards         `validate:"required" mapstructure:"rewards"`
	Thresholds      ScoreOptionThresholds      `validate:"required" mapstructure:"thresholds"`
	Behaviour       ScoreOptionBehaviour       `validate:"required" mapstructure:"behaviour"`
	TopicValidation ScoreOptionTopicValidation `validate:"required" mapstructure:"topic"`
}

const (
	MaxAppSpecificPenaltyKey      = "max-app-specific-penalty"
	MinAppSpecificPenaltyKey      = "min-app-specific-penalty"
	UnknownIdentityPenaltyKey     = "unknown-identity-penalty"
	InvalidSubscriptionPenaltyKey = "invalid-subscription-penalty"
)

// ScoreOptionPenalties score option penalty configuration parameters.
type ScoreOptionPenalties struct {
	// MaxAppSpecificPenalty the maximum penalty for sever offenses that we apply to a remote node score. The score
	// mechanism of GossipSub in Flow is designed in a way that all other infractions are penalized with a fraction of
	// this value. We have also set the other parameters such as DefaultGraylistThreshold, DefaultGossipThreshold and DefaultPublishThreshold to
	// be a bit higher than this, i.e., MaxAppSpecificPenalty + 1. This ensures that a node with a score of MaxAppSpecificPenalty
	// will be graylisted (i.e., all incoming and outgoing RPCs are rejected) and will not be able to publish or gossip any messages.
	MaxAppSpecificPenalty float64 `validate:"lt=0" mapstructure:"max-app-specific-penalty"`
	// MinAppSpecificPenalty the minimum penalty for sever offenses that we apply to a remote node score.
	MinAppSpecificPenalty float64 `validate:"lt=0" mapstructure:"min-app-specific-penalty"`
	// UnknownIdentityPenalty is the  penalty for unknown identity. It is applied to the peer's score when
	// the peer is not in the identity list.
	UnknownIdentityPenalty float64 `validate:"lt=0" mapstructure:"unknown-identity-penalty"`
	// InvalidSubscriptionPenalty is the  penalty for invalid subscription. It is applied to the peer's score when
	// the peer subscribes to a topic that it is not authorized to subscribe to.
	InvalidSubscriptionPenalty float64 `validate:"lt=0" mapstructure:"invalid-subscription-penalty"`
}

const (
	MaxAppSpecificRewardKey = "max-app-specific-reward"
	StakedIdentityRewardKey = "staked-identity-reward"
)

// ScoreOptionRewards score option rewards configuration parameters.
type ScoreOptionRewards struct {
	// MaxAppSpecificReward is the  reward for well-behaving staked peers. If a peer does not have
	// any misbehavior record, e.g., invalid subscription, invalid message, etc., it will be rewarded with this score.
	MaxAppSpecificReward float64 `validate:"gt=0" mapstructure:"max-app-specific-reward"`
	// StakedIdentityReward is the  reward for staking peers. It is applied to the peer's score when
	// the peer does not have any misbehavior record, e.g., invalid subscription, invalid message, etc.
	// The purpose is to reward the staking peers for their contribution to the network and prioritize them in neighbor selection.
	StakedIdentityReward float64 `validate:"gt=0" mapstructure:"staked-identity-reward"`
}

const (
	GossipThresholdKey             = "gossip"
	PublishThresholdKey            = "publish"
	GraylistThresholdKey           = "graylist"
	AcceptPXThresholdKey           = "accept-px"
	OpportunisticGraftThresholdKey = "opportunistic-graft"
)

// ScoreOptionThresholds score option threshold configuration parameters.
type ScoreOptionThresholds struct {
	// Gossip when a peer's penalty drops below this threshold,
	// no gossip is emitted towards that peer and gossip from that peer is ignored.
	Gossip float64 `validate:"lt=0" mapstructure:"gossip"`
	// Publish when a peer's penalty drops below this threshold,
	// self-published messages are not propagated towards this peer.
	Publish float64 `validate:"lt=0" mapstructure:"publish"`
	// Graylist when a peer's penalty drops below this threshold, the peer is graylisted, i.e.,
	// incoming RPCs from the peer are ignored.
	Graylist float64 `validate:"lt=0" mapstructure:"graylist"`
	// AcceptPX when a peer sends us PX information with a prune, we only accept it and connect to the supplied
	// peers if the originating peer's penalty exceeds this threshold.
	AcceptPX float64 `validate:"gt=0" mapstructure:"accept-px"`
	// OpportunisticGraft when the median peer penalty in the mesh drops below this value,
	// the peer may select more peers with penalty above the median to opportunistically graft on the mesh.
	OpportunisticGraft float64 `validate:"gt=0" mapstructure:"opportunistic-graft"`
}

const (
	BehaviourPenaltyThresholdKey = "penalty-threshold"
	BehaviourPenaltyWeightKey    = "penalty-weight"
	BehaviourPenaltyDecayKey     = "penalty-decay"
)

// ScoreOptionBehaviour score option behaviour configuration parameters.
type ScoreOptionBehaviour struct {
	// PenaltyThreshold is the threshold when the behavior of a peer is considered as bad by GossipSub.
	// Currently, the misbehavior is defined as advertising an iHave without responding to the iWants (iHave broken promises), as well as attempting
	// on GRAFT when the peer is considered for a PRUNE backoff, i.e., the local peer does not allow the peer to join the local topic mesh
	// for a while, and the remote peer keep attempting on GRAFT (aka GRAFT flood).
	// When the misbehavior counter of a peer goes beyond this threshold, the peer is penalized by BehaviorPenaltyWeight (see below) for the excess misbehavior.
	//
	// An iHave broken promise means that a peer advertises an iHave for a message, but does not respond to the iWant requests for that message.
	// For iHave broken promises, the gossipsub scoring works as follows:
	// It samples ONLY A SINGLE iHave out of the entire RPC.
	// If that iHave is not followed by an actual message within the next 3 seconds, the peer misbehavior counter is incremented by 1.
	//
	// We set it to 10, meaning that we at most tolerate 10 of such RPCs containing iHave broken promises. After that, the peer is penalized for every
	// excess RPC containing iHave broken promises.
	// The counter is also decayed by (0.99) every decay interval (DecayInterval) i.e., every minute.
	// Note that misbehaviors are counted by GossipSub across all topics (and is different from the Application Layer Misbehaviors that we count through
	// the ALSP system).
	PenaltyThreshold float64 `validate:"gt=0" mapstructure:"penalty-threshold"`
	// PenaltyWeight is the weight for applying penalty when a peer misbehavior goes beyond the threshold.
	// Misbehavior of a peer at gossipsub layer is defined as advertising an iHave without responding to the iWants (broken promises), as well as attempting
	// on GRAFT when the peer is considered for a PRUNE backoff, i.e., the local peer does not allow the peer to join the local topic mesh
	// This is detected by the GossipSub scoring system, and the peer is penalized by BehaviorPenaltyWeight.
	//
	// An iHave broken promise means that a peer advertises an iHave for a message, but does not respond to the iWant requests for that message.
	// For iHave broken promises, the gossipsub scoring works as follows:
	// It samples ONLY A SINGLE iHave out of the entire RPC.
	// If that iHave is not followed by an actual message within the next 3 seconds, the peer misbehavior counter is incremented by 1.
	//
	// The penalty is applied to the square of the difference between the misbehavior counter and the threshold, i.e., -|w| * (misbehavior counter - threshold)^2.
	// We set it to 0.01 * MaxAppSpecificPenalty, which means that misbehaving 10 times more than the threshold (i.e., 10 + 10) will cause the peer to lose
	// its entire AppSpecificReward that is awarded by our app-specific scoring function to all staked (i.e., authorized) nodes by .
	// Moreover, as the MaxAppSpecificPenalty is -MaxAppSpecificReward, misbehaving sqrt(2) * 10 times more than the threshold will cause the peer score
	// to be dropped below the MaxAppSpecificPenalty, which is also below the Graylist, and the peer will be graylisted (i.e., disconnected).
	//
	// The math is as follows: -|w| * (misbehavior - threshold)^2 = 0.01 * MaxAppSpecificPenalty * (misbehavior - threshold)^2 < 2 * MaxAppSpecificPenalty
	// if misbehavior > threshold + sqrt(2) * 10.
	// As shown above, with this choice of BehaviorPenaltyWeight, misbehaving sqrt(2) * 10 times more than the threshold will cause the peer score
	// to be dropped below the MaxAppSpecificPenalty, which is also below the Graylist, and the peer will be graylisted (i.e., disconnected). This weight
	// is chosen in a way that with almost a few misbehaviors more than the threshold, the peer will be graylisted. The rationale relies on the fact that
	// the misbehavior counter is incremented by 1 for each RPC containing one or more broken promises. Hence, it is per RPC, and not per broken promise.
	// Having sqrt(2) * 10 broken promises RPC is a blatant misbehavior, and the peer should be graylisted. With decay interval of 1 minute, and decay value of
	// 0.99 we expect a graylisted node due to borken promises to get back in about 527 minutes, i.e., (0.99)^x * (sqrt(2) * 10)^2 * MaxAppSpecificPenalty > Graylist
	// where x is the number of decay intervals that the peer is graylisted. As MaxAppSpecificPenalty and GraylistThresholds are close, we can simplify the inequality
	// to (0.99)^x * (sqrt(2) * 10)^2 > 1 --> (0.99)^x * 200 > 1 --> (0.99)^x > 1/200 --> x > log(1/200) / log(0.99) --> x > 527.17 decay intervals, i.e., 527 minutes.
	// Note that misbehaviors are counted by GossipSub across all topics (and is different from the Application Layer Misbehaviors that we count through
	// the ALSP system that are reported by the engines).
	PenaltyWeight float64 `validate:"lt=0" mapstructure:"penalty-weight"`
	// PenaltyDecay is the decay interval for the misbehavior counter of a peer. The misbehavior counter is
	// incremented by GossipSub for iHave broken promises or the GRAFT flooding attacks (i.e., each GRAFT received from a remote peer while that peer is on a PRUNE backoff).
	//
	// An iHave broken promise means that a peer advertises an iHave for a message, but does not respond to the iWant requests for that message.
	// For iHave broken promises, the gossipsub scoring works as follows:
	// It samples ONLY A SINGLE iHave out of the entire RPC.
	// If that iHave is not followed by an actual message within the next 3 seconds, the peer misbehavior counter is incremented by 1.
	// This means that regardless of how many iHave broken promises an RPC contains, the misbehavior counter is incremented by 1.
	// That is why we decay the misbehavior counter very slow, as this counter indicates a severe misbehavior.
	//
	// The misbehavior counter is decayed per decay interval (i.e., DecayInterval = 1 minute) by GossipSub.
	// We set it to 0.99, which means that the misbehavior counter is decayed by 1% per decay interval.
	// With the generous threshold that we set (i.e., PenaltyThreshold = 10), we take the peers going beyond the threshold as persistent misbehaviors,
	// We expect honest peers never to go beyond the threshold, and if they do, we expect them to go back below the threshold quickly.
	//
	// Note that misbehaviors are counted by GossipSub across all topics (and is different from the Application Layer Misbehaviors that we count through
	// the ALSP system that is based on the engines report).
	PenaltyDecay float64 `validate:"gt=0,lt=1" mapstructure:"penalty-decay"`
}

const (
	SkipAtomicValidationKey           = "skip-atomic-validation"
	InvalidMessageDeliveriesWeightKey = "invalid-message-deliveries-weight"
	InvalidMessageDeliveriesDecayKey  = "invalid-message-deliveries-decay"
	TimeInMeshQuantumKey              = "time-in-mesh-quantum"
	TopicWeightKey                    = "topic-weight"
	MeshMessageDeliveriesDecayKey     = "mesh-message-deliveries-decay"
	MeshMessageDeliveriesCapKey       = "mesh-message-deliveries-cap"
	MeshMessageDeliveryThresholdKey   = "mesh-message-deliveries-threshold"
	MeshDeliveriesWeightKey           = "mesh-deliveries-weight"
	MeshMessageDeliveriesWindowKey    = "mesh-message-deliveries-window"
	MeshMessageDeliveryActivationKey  = "mesh-message-delivery-activation"
)

// ScoreOptionTopicValidation score option topic validation configuration parameters.
type ScoreOptionTopicValidation struct {
	// SkipAtomicValidation is the  value for the skip atomic validation flag for topics.
	// We set it to true, which means gossipsub parameter validation will not fail if we leave some of the
	// topic parameters at their  values, i.e., zero. This is because we are not setting all
	// topic parameters at the current implementation.
	SkipAtomicValidation bool `validate:"required" mapstructure:"skip-atomic-validation"`
	// InvalidMessageDeliveriesWeight this value is applied to the square of the number of invalid message deliveries on a topic.
	// It is used to penalize peers that send invalid messages. By an invalid message, we mean a message that is not signed by the
	// publisher, or a message that is not signed by the peer that sent it. We set it to -1.0, which means that with around 14 invalid
	// message deliveries within a gossipsub heartbeat interval, the peer will be disconnected.
	// The supporting math is as follows:
	// - each staked (i.e., authorized) peer is rewarded by the fixed reward of 100 (i.e., StakedIdentityReward).
	// - x invalid message deliveries will result in a penalty of x^2 * InvalidMessageDeliveriesWeight, i.e., -x^2.
	// - the peer will be disconnected when its penalty reaches -100 (i.e., MaxAppSpecificPenalty).
	// - so, the maximum number of invalid message deliveries that a peer can have before being disconnected is sqrt(200/InvalidMessageDeliveriesWeight) ~ 14.
	InvalidMessageDeliveriesWeight float64 `validate:"lt=0" mapstructure:"invalid-message-deliveries-weight"`
	// InvalidMessageDeliveriesDecay decay factor used to decay the number of invalid message deliveries.
	// The total number of invalid message deliveries is multiplied by this factor at each heartbeat interval to
	// decay the number of invalid message deliveries, and prevent the peer from being disconnected if it stops
	// sending invalid messages. We set it to 0.99, which means that the number of invalid message deliveries will
	// decay by 1% at each heartbeat interval.
	// The decay heartbeats are defined by the heartbeat interval of the gossipsub scoring system, which is 1 Minute (DecayInterval).
	InvalidMessageDeliveriesDecay float64 `validate:"gt=0,lt=1" mapstructure:"invalid-message-deliveries-decay"`
	// TimeInMeshQuantum is the  time in mesh quantum for the GossipSub scoring system. It is used to gauge
	// a discrete time interval for the time in mesh counter. We set it to 1 hour, which means that every one complete hour a peer is
	// in a topic mesh, the time in mesh counter will be incremented by 1 and is counted towards the availability score of the peer in that topic mesh.
	// The reason of setting it to 1 hour is that we want to reward peers that are in a topic mesh for a long time, and we want to avoid rewarding peers that
	// are churners, i.e., peers that join and leave a topic mesh frequently.
	TimeInMeshQuantum time.Duration `validate:"gte=1h" mapstructure:"time-in-mesh-quantum"`
	// Weight is the  weight of a topic in the GossipSub scoring system.
	// The overall score of a peer in a topic mesh is multiplied by the weight of the topic when calculating the overall score of the peer.
	// We set it to 1.0, which means that the overall score of a peer in a topic mesh is not affected by the weight of the topic.
	TopicWeight float64 `validate:"gt=0" mapstructure:"topic-weight"`
	// MeshMessageDeliveriesDecay is applied to the number of actual message deliveries in a topic mesh
	// at each decay interval (i.e., DecayInterval).
	// It is used to decay the number of actual message deliveries, and prevents past message
	// deliveries from affecting the current score of the peer.
	// As the decay interval is 1 minute, we set it to 0.5, which means that the number of actual message
	// deliveries will decay by 50% at each decay interval.
	MeshMessageDeliveriesDecay float64 `validate:"gt=0" mapstructure:"mesh-message-deliveries-decay"`
	// MeshMessageDeliveriesCap is the maximum number of actual message deliveries in a topic
	// mesh that is used to calculate the score of a peer in that topic mesh.
	// We set it to 1000, which means that the maximum number of actual message deliveries in a
	// topic mesh that is used to calculate the score of a peer in that topic mesh is 1000.
	// This is to prevent the score of a peer in a topic mesh from being affected by a large number of actual
	// message deliveries and also affect the score of the peer in other topic meshes.
	// When the total delivered messages in a topic mesh exceeds this value, the score of the peer in that topic
	// mesh will not be affected by the actual message deliveries in that topic mesh.
	// Moreover, this does not allow the peer to accumulate a large number of actual message deliveries in a topic mesh
	// and then start under-performing in that topic mesh without being penalized.
	MeshMessageDeliveriesCap float64 `validate:"gt=0" mapstructure:"mesh-message-deliveries-cap"`
	// MeshMessageDeliveryThreshold is the threshold for the number of actual message deliveries in a
	// topic mesh that is used to calculate the score of a peer in that topic mesh.
	// If the number of actual message deliveries in a topic mesh is less than this value,
	// the peer will be penalized by square of the difference between the actual message deliveries and the threshold,
	// i.e., -w * (actual - threshold)^2 where `actual` and `threshold` are the actual message deliveries and the
	// threshold, respectively, and `w` is the weight (i.e., MeshMessageDeliveriesWeight).
	// We set it to 0.1 * MeshMessageDeliveriesCap, which means that if a peer delivers less tha 10% of the
	// maximum number of actual message deliveries in a topic mesh, it will be considered as an under-performing peer
	// in that topic mesh.
	MeshMessageDeliveryThreshold float64 `validate:"gt=0" mapstructure:"mesh-message-deliveries-threshold"`
	// MeshDeliveriesWeight is the weight for applying penalty when a peer is under-performing in a topic mesh.
	// Upon every decay interval, if the number of actual message deliveries is less than the topic mesh message deliveries threshold
	// (i.e., MeshMessageDeliveriesThreshold), the peer will be penalized by square of the difference between the actual
	// message deliveries and the threshold, multiplied by this weight, i.e., -w * (actual - threshold)^2 where w is the weight, and
	// `actual` and `threshold` are the actual message deliveries and the threshold, respectively.
	// We set this value to be - 0.05 MaxAppSpecificReward / (MeshMessageDeliveriesThreshold^2). This guarantees that even if a peer
	// is not delivering any message in a topic mesh, it will not be disconnected.
	// Rather, looses part of the MaxAppSpecificReward that is awarded by our app-specific scoring function to all staked
	// nodes by will be withdrawn, and the peer will be slightly penalized. In other words, under-performing in a topic mesh
	// will drop the overall score of a peer by 5% of the MaxAppSpecificReward that is awarded by our app-specific scoring function.
	// It means that under-performing in a topic mesh will not cause a peer to be disconnected, but it will cause the peer to lose
	// its MaxAppSpecificReward that is awarded by our app-specific scoring function.
	// At this point, we do not want to disconnect a peer only because it is under-performing in a topic mesh as it might be
	// causing a false positive network partition.
	// TODO: we must increase the penalty for under-performing in a topic mesh in the future, and disconnect the peer if it is under-performing.
	MeshDeliveriesWeight float64 `validate:"lt=0" mapstructure:"mesh-deliveries-weight"`
	// MeshMessageDeliveriesWindow is the window size is time interval that we count a delivery of an already
	// seen message towards the score of a peer in a topic mesh. The delivery is counted
	// by GossipSub only if the previous sender of the message is different from the current sender.
	// We set it to the decay interval of the GossipSub scoring system, which is 1 minute.
	// It means that if a peer delivers a message that it has already seen less than one minute ago,
	// the delivery will be counted towards the score of the peer in a topic mesh only if the previous sender of the message.
	// This also prevents replay attacks of messages that are older than one minute. As replayed messages will not
	// be counted towards the actual message deliveries of a peer in a topic mesh.
	MeshMessageDeliveriesWindow time.Duration `validate:"gte=1m" mapstructure:"mesh-message-deliveries-window"`
	// MeshMessageDeliveryActivation is the time interval that we wait for a new peer that joins a topic mesh
	// till start counting the number of actual message deliveries of that peer in that topic mesh.
	// We set it to 2 * DecayInterval, which means that we wait for 2 decay intervals before start counting
	// the number of actual message deliveries of a peer in a topic mesh.
	// With a  decay interval of 1 minute, it means that we wait for 2 minutes before start counting the
	// number of actual message deliveries of a peer in a topic mesh. This is to account for
	// the time that it takes for a peer to start up and receive messages from other peers in the topic mesh.
	MeshMessageDeliveryActivation time.Duration `validate:"gte=2m" mapstructure:"mesh-message-delivery-activation"`
}
