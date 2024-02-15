package model

// To give a summary with the default value:
//  1. The penalty of each misbehavior is 0.01 * DisallowListingThreshold = -864
//  2. The penalty of each misbehavior is decayed by a decay value at each decay interval. The default decay value is 1000.
//     This means that by default if a node misbehaves 100 times in a second, it gets disallow-listed, and takes 86.4 seconds to recover.
//     We emphasize on the default penalty value can be amplified by the engine that reports the misbehavior.
//  3. Each time a node is disallow-listed, its decay speed is decreased by 90%. This means that if a node is disallow-listed
//     for the first time, it takes 86.4 seconds to recover. If the node is disallow-listed for the second time, its decay
//     speed is decreased by 90% from 1000 to 100, and it takes around 15 minutes to recover. If the node is disallow-listed
//     for the third time, its decay speed is decreased by 90% from 100 to 10, and it takes around 2.5 hours to recover.
//     If the node is disallow-listed for the fourth time, its decay speed is decreased by 90% from 10 to 1, and it takes
//     around a day to recover. From this point on, the decay speed is 1, and it takes around a day to recover from each
//     disallow-listing.
const (
	// DisallowListingThreshold is the threshold for concluding a node behavior is malicious and disallow-listing the node.
	// If the overall penalty of this node drops below this threshold, the node is reported to be disallow-listed by
	// the networking layer, i.e., existing connections to the node are closed and the node is no longer allowed to connect till
	// its penalty is decayed back to zero.
	// maximum block-list period is 1 day
	DisallowListingThreshold = -24 * 60 * 60 // (Don't change this value)

	// DefaultPenaltyValue is the default penalty value for misbehaving nodes.
	// By default, each reported infringement will be penalized by this value. However, the penalty can be amplified
	// by the engine that reports the misbehavior. The penalty system is designed in a way that more than 100 misbehaviors/sec
	// at the default penalty value will result in disallow-listing the node. By amplifying the penalty, the engine can
	// decrease the number of misbehaviors/sec that will result in disallow-listing the node. For example, if the engine
	// amplifies the penalty by 10, the number of misbehaviors/sec that will result in disallow-listing the node will be
	// 10 times less than the default penalty value and the node will be disallow-listed after 10 misbehaviors/sec.
	DefaultPenaltyValue = 0.01 * DisallowListingThreshold // (Don't change this value)

	// InitialDecaySpeed is the initial decay speed of the penalty of a misbehaving node.
	// The decay speed is applied on an arithmetic progression. The penalty value of the node is the first term of the
	// progression and the decay speed is the common difference of the progression, i.e., p(n) = p(0) + n * d, where
	// p(n) is the penalty value of the node after n decay intervals, p(0) is the initial penalty value of the node, and
	// d is the decay speed. Decay intervals are set to 1 second (protocol invariant). Hence, with the initial decay speed
	// of 1000, the penalty value of the node will be decreased by 1000 every second. This means that if a node misbehaves
	// 100 times in a second, it gets disallow-listed, and takes 86.4 seconds to recover.
	// In mature implementation of the protocol, the decay speed of a node is decreased by 90% each time the node is
	// disallow-listed. This means that if a node is disallow-listed for the first time, it takes 86.4 seconds to recover.
	// If the node is disallow-listed for the second time, its decay speed is decreased by 90% from 1000 to 100, and it
	// takes around 15 minutes to recover. If the node is disallow-listed for the third time, its decay speed is decreased
	// by 90% from 100 to 10, and it takes around 2.5 hours to recover. If the node is disallow-listed for the fourth time,
	// its decay speed is decreased by 90% from 10 to 1, and it takes around a day to recover. From this point on, the decay
	// speed is 1, and it takes around a day to recover from each disallow-listing.
	InitialDecaySpeed = 1000 // (Don't change this value)
)
