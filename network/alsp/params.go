package alsp

import "time"

// To give a summary with the default value:
//  1. The penalty of each misbehavior is 0.01 * misbehaviorDisallowListingThreshold = -86.4
//  2. The penalty of each misbehavior is decayed by a decay value at each decay interval. The default decay value is 1000.
//     This means that by default if a node misbehaves 1000 times in a second, it gets disallow-listed, and takes 86.4 seconds to recover.
//     We emphasize on the default penalty value can be amplified by the engine that reports the misbehavior.
//  3. Each time a node is disallow-listed, its decay speed is decreased by 90%. This means that if a node is disallow-listed
//     for the first time, it takes 86.4 seconds to recover. If the node is disallow-listed for the second time, its decay
//     speed is decreased by 90% from 1000 to 100, and it takes around 15 minutes to recover. If the node is disallow-listed
//     for the third time, its decay speed is decreased by 90% from 100 to 10, and it takes around 2.5 hours to recover.
//     If the node is disallow-listed for the fourth time, its decay speed is decreased by 90% from 10 to 1, and it takes
//     around a day to recover. From this point on, the decay speed is 1, and it takes around a day to recover from each
//     disallow-listing.
const (
	// misbehaviorDisallowListingThreshold is the threshold for concluding a node behavior is malicious and disallow-listing the node.
	// If the overall penalty of this node drops below this threshold, the node is reported to be disallow-listed by
	// the networking layer, i.e., existing connections to the node are closed and the node is no longer allowed to connect till
	// its penalty is decayed back to zero.
	misbehaviorDisallowListingThreshold = -86400

	// defaultPenaltyValue is the default penalty value for misbehaving nodes.
	// By default, each reported infringement will be penalized by this value. However, the penalty can be amplified
	// by the engine that reports the misbehavior. The penalty system is designed in a way that more than 100 misbehavior/sec
	// at the default penalty value will result in disallow-listing the node. By amplifying the penalty, the engine can
	// decrease the number of misbehavior/sec that will result in disallow-listing the node. For example, if the engine
	// amplifies the penalty by 10, the number of misbehavior/sec that will result in disallow-listing the node will be
	// 10 times less than the default penalty value and the node will be disallow-listed after 10 times more misbehavior/sec.
	defaultPenaltyValue = 0.01 * misbehaviorDisallowListingThreshold

	// misbehaviorDecayHeartbeatInterval is the interval at which the penalty of the misbehaving nodes is decayed.
	// At each interval, the penalty of the misbehaving nodes is decayed by the deducting the defaultDecayValue from the penalty.
	misbehaviorDecayHeartBeatInterval = 1 * time.Second

	// defaultDecayValue is the default value that is deducted from the penalty of the misbehaving nodes at each decay interval.
	// This is called the default decay value because the decay value will be changed per node each time the node goes
	// below the disallow-listing threshold. Hence, the nodes that are misbehaving more than the others will be decayed
	// slower than the others.
	defaultDecayValue = 1000

	// decayValueSpeedPenalty is the penalty for the decay speed. The decay speed is multiplied by this value each time
	// the node goes below the disallow-listing threshold.
	decayValueSpeedPenalty = 0.1

	// minimumDecayValue is the minimum decay value that is used to decay the penalty of the misbehaving nodes.
	// The decay value is multiplied by the decay value speed penalty each time the node goes below the disallow-listing
	// threshold. The minimum decay value is used to prevent the decay value from becoming too small. If the decay value
	// becomes too small, the penalty of the misbehaving forever. Hence, the decay value is capped at this value.
	minimumDecayValue = 1
)
