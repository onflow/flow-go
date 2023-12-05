# GossipSub App Specific Score 

This package provides a scoring mechanism for peers in a GossipSub network by computing their application-specific scores.
Application-specific score is part of the GossipSub scoring mechanism, which is used to determine the behavior of peers in the network from
the perspective of their behavior at the application level (i.e., Flow protocol).
The score is determined based on a combination of penalties and rewards related to various factors, such as spamming misbehaviors, staking status, and valid subscriptions.

## Key Components
1. `GossipSubAppSpecificScoreRegistry`: This struct maintains the necessary information for determining a peer's score.
2. `AppSpecificScoreFunc`: This function is exposed to GossipSub and calculates the application-specific score for a peer based on penalties and rewards.
3. `stakingScore`: This function computes the staking score (reward/penalty) for a peer based on their identity and role.
4. `subscriptionPenalty`: This function calculates the penalty for invalid subscriptions.
5. `OnInvalidControlMessageNotification`: This method updates a peer's penalty when an invalid control message misbehavior is detected, e.g., spamming on a control message.

## Score Calculation
The application-specific score for a peer is calculated as the sum of the following factors:

1. Spam Penalty: A penalty applied when a peer conducts a spamming misbehavior (e.g., GRAFT, PRUNE, iHave, or iWant misbehaviors).
2. Staking Penalty: A penalty applied for unknown peers with invalid Flow protocol identities. This ejects them from the GossipSub network. 
3. Subscription Penalty: A penalty applied when a peer subscribes to a topic they are not allowed to, based on their role in the Flow network.
4. Staking Reward: A reward applied to well-behaved staked peers (excluding access nodes at the moment) only if they have no penalties from spamming or invalid subscriptions.

The score is updated every time a peer misbehaves, and the spam penalties decay over time using the default decay function, which applies a geometric decay to the peer's score.

### Usage
To use the scoring mechanism, create a new `GossipSubAppSpecificScoreRegistry` with the desired configuration, and then obtain the `AppSpecificScoreFunc` to be passed to the GossipSub protocol.

Example:
```go
config := &GossipSubAppSpecificScoreRegistryConfig{
	// ... configure the required components
}
registry := NewGossipSubAppSpecificScoreRegistry(config)
appSpecificScoreFunc := registry.AppSpecificScoreFunc()

// Use appSpecificScoreFunc as the score function for GossipSub
```

The scoring mechanism can be easily integrated with the GossipSub protocol to ensure that well-behaved peers are prioritized, and misbehaving peers are penalized. See the `ScoreOption` below for more details.

**Note**: This package was designed specifically for the Flow network and might require adjustments if used in other contexts.


## Score Option
`ScoreOption` is a configuration object for the peer scoring system in the Flow network.
It defines several scoring parameters and thresholds that determine the behavior of the network towards its peers.
This includes rewarding well-behaving peers and penalizing misbehaving ones.

**Note**: `ScoreOption` is passed to the GossipSub as a configuration option at the time of initialization.

### Usage
To use the `ScoreOption`, you need to create a `ScoreOptionConfig` with the desired settings and then call `NewScoreOption` with that configuration.

```go
config := NewScoreOptionConfig(logger)
config.SetProvider(identityProvider)
config.SetCacheSize(1000)
config.SetCacheMetrics(metricsCollector)

// Optional: Set custom app-specific scoring function
config.SetAppSpecificScoreFunction(customAppSpecificScoreFunction)

scoreOption := NewScoreOption(config)
```

### Scoring Parameters
`ScoreOption` provides a set of default scoring parameters and thresholds that can be configured through the `ScoreOptionConfig`. These parameters include:

1. `AppSpecificScoreWeight`: The weight of the application-specific score in the overall peer score calculation at the GossipSub.
2. `GossipThreshold`: The threshold below which a peer's score will result in ignoring gossips to and from that peer.
3. `PublishThreshold`: The threshold below which a peer's score will result in not propagating self-published messages to that peer.
4. `GraylistThreshold`: The threshold below which a peer's score will result in ignoring incoming RPCs from that peer.
5. `AcceptPXThreshold`: The threshold above which a peer's score will result in accepting PX information with a prune from that peer. PX stands for "Peer Exchange" in the context of libp2p's gossipsub protocol. When a peer sends a PRUNE control message to another peer, it can include a list of other peers as PX information. The purpose of this is to help the pruned peer find new peers to replace the ones that have been pruned from its mesh. When a node receives a PRUNE message containing PX information, it can decide whether to connect to the suggested peers based on its own criteria. In this package, the `DefaultAcceptPXThreshold` is used to determine if the originating peer's penalty score is good enough to accept the PX information. If the originating peer's penalty score exceeds the threshold, the node will consider connecting to the suggested peers.
6. `OpportunisticGraftThreshold`: The threshold below which the median peer score in the mesh may result in selecting more peers with a higher score for opportunistic grafting.

### Flow Specific Scoring Parameters and Thresholds
# GossipSub Scoring Parameters Explained
1. `DefaultAppSpecificScoreWeight = 1`: This is the default weight for application-specific scoring. It basically tells us how important the application-specific score is in comparison to other scores.
2. `MaxAppSpecificPenalty = -100` and `MinAppSpecificPenalty = -1`: These values define the range for application-specific penalties. A peer can have a maximum penalty of -100 and a minimum penalty of -1.
3. `MaxAppSpecificReward = 100`: This is the maximum reward a peer can earn for good behavior.
4. `DefaultStakedIdentityReward = MaxAppSpecificReward`: This reward is given to peers that contribute positively to the network (i.e., no misbehavior). It’s to encourage them and prioritize them in neighbor selection.
5. `DefaultUnknownIdentityPenalty = MaxAppSpecificPenalty`: This penalty is given to a peer if it's not in the identity list. It's to discourage anonymity.
6. `DefaultInvalidSubscriptionPenalty = MaxAppSpecificPenalty`: This penalty is for peers that subscribe to topics they are not authorized to subscribe to.
7. `DefaultGossipThreshold = -99`: If a peer's penalty goes below this threshold, the peer is ignored for gossip. It means no gossip is sent to or received from that peer.
8. `DefaultPublishThreshold = -99`: If a peer's penalty goes below this threshold, self-published messages will not be sent to this peer.
9. `DefaultGraylistThreshold = -99`: If a peer's penalty goes below this threshold, it is graylisted. This means all incoming messages from this peer are ignored.
10. `DefaultAcceptPXThreshold = 99`: This is a threshold for accepting peers. If a peer sends information and its score is above this threshold, the information is accepted.
11. `DefaultOpportunisticGraftThreshold = MaxAppSpecificReward + 1`: This value is used to selectively connect to new peers if the median score of the current peers drops below this threshold.
12. `defaultScoreCacheSize = 1000`: Sets the default size of the cache used to store the application-specific penalty of peers.
13. `defaultDecayInterval = 1 * time.Minute`: Sets the default interval at which the score of a peer will be decayed.
14. `defaultDecayToZero = 0.01`: This is a threshold below which a decayed score is reset to zero. It prevents the score from decaying to a very small value.
15. `defaultTopicTimeInMeshQuantum` is a parameter in the GossipSub scoring system that represents a fixed time interval used to count the amount of time a peer stays in a topic mesh. It is set to 1 hour, meaning that for each hour a peer remains in a topic mesh, its time-in-mesh counter increases by 1, contributing to its availability score. This is to reward peers that stay in the mesh for longer durations and discourage those that frequently join and leave.
16. `defaultTopicInvalidMessageDeliveriesWeight` is set to -1.0 and is used to penalize peers that send invalid messages by applying it to the square of the number of such messages. A message is considered invalid if it is not properly signed. A peer will be disconnected if it sends around 14 invalid messages within a gossipsub heartbeat interval.
17. `defaultTopicInvalidMessageDeliveriesDecay` is a decay factor set to 0.99. It is used to reduce the number of invalid message deliveries counted against a peer by 1% at each heartbeat interval. This prevents the peer from being disconnected if it stops sending invalid messages. The heartbeat interval in the gossipsub scoring system is set to 1 minute by default.

## GossipSub Message Delivery Scoring
This section provides an overview of the GossipSub message delivery scoring mechanism used in the Flow network. 
It's designed to maintain an efficient, secure and stable peer-to-peer network by scoring each peer based on their message delivery performance. 
The system ensures the reliability of message propagation by scoring peers, which discourages malicious behaviors and enhances overall network performance.

### Comprehensive System Overview
The GossipSub message delivery scoring mechanism used in the Flow network is an integral component of its P2P communication model. 
It is designed to monitor and incentivize appropriate network behaviors by attributing scores to peers based on their message delivery performance. 
This scoring system is fundamental to ensure that messages are reliably propagated across the network, creating a robust P2P communication infrastructure.

The scoring system is per topic, which means it tracks the efficiency of peers in delivering messages in each specific topic they are participating in. 
These per-topic scores then contribute to an overall score for each peer, providing a comprehensive view of a peer's effectiveness within the network.
In GossipSub, a crucial aspect of a peer's responsibility is to relay messages effectively to other nodes in the network. 
The role of the scoring mechanism is to objectively assess a peer's efficiency in delivering these messages. 
It takes into account several factors to determine the effectiveness of the peers.

1. **Message Delivery Rate** - A peer's ability to deliver messages quickly is a vital metric. Slow delivery could lead to network lags and inefficiency.
2. **Message Delivery Volume** - A peer's capacity to deliver a large number of messages accurately and consistently.
3. **Continuity of Performance** - The scoring mechanism tracks not only the rate and volume of the messages but also the consistency in a peer's performance over time.
4. **Prevention of Malicious Behaviors** - The scoring system also helps in mitigating potential network attacks such as spamming and message replay attacks.

The system utilizes several parameters to maintain and adjust the scores of the peers:
- `defaultTopicMeshMessageDeliveriesDecay`(value: 0.5): This parameter dictates how rapidly a peer's message delivery count decays with time. With a value of 0.5, it indicates a 50% decay at each decay interval. This mechanism ensures that past performances do not disproportionately impact the current score of the peer.
- `defaultTopicMeshMessageDeliveriesCap` (value: 1000): This parameter sets an upper limit on the number of message deliveries that can contribute to the score of a peer in a topic. With a cap set at 1000, it prevents the score from being overly influenced by large volumes of message deliveries, providing a balanced assessment of peer performance.
- `defaultTopicMeshMessageDeliveryThreshold` (value: 0.1 * `defaultTopicMeshMessageDeliveriesCap`): This threshold serves to identify under-performing peers. If a peer's message delivery count is below this threshold in a topic, the peer's score is penalized. This encourages peers to maintain a minimum level of performance.
- `defaultTopicMeshMessageDeliveriesWeight`  (value: -0.05 * `MaxAppSpecificReward` / (`defaultTopicMeshMessageDeliveryThreshold` ^ 2) = 5^-4): This weight is applied when penalizing under-performing peers. The penalty is proportional to the square of the difference between the actual message deliveries and the threshold, multiplied by this weight.
- `defaultMeshMessageDeliveriesWindow` (value: `defaultDecayInterval` = 1 minute): This parameter defines the time window within which a message delivery is counted towards the score. This window is set to the decay interval, preventing replay attacks and counting only unique message deliveries.
- `defaultMeshMessageDeliveriesActivation` (value: 2 * `defaultDecayInterval` = 2 minutes): This time interval is the grace period before the scoring system starts tracking a new peer's performance. It accounts for the time it takes for a new peer to fully integrate into the network.

By continually updating and adjusting the scores of peers based on these parameters, the GossipSub message delivery scoring mechanism ensures a robust, efficient, and secure P2P network.

### Examples

#### Scenario 1: Peer A Delivers Messages Within Cap and Above Threshold
Let's assume a Peer A that consistently delivers 500 messages per decay interval. This is within the `defaultTopicMeshMessageDeliveriesCap` (1000) and above the `defaultTopicMeshMessageDeliveryThreshold` (100).
As Peer A's deliveries are above the threshold and within the cap, its score will not be penalized. Instead, it will be maintained, promoting healthy network participation.

#### Scenario 2: Peer B Delivers Messages Below Threshold
Now, assume Peer B delivers 50 messages per decay interval, below the `defaultTopicMeshMessageDeliveryThreshold` (100).
In this case, the score of Peer B will be penalized because its delivery rate is below the threshold. The penalty is calculated as `-|w| * (actual - threshold)^2`, where `w` is the weight (`defaultTopicMeshMessageDeliveriesWeight`), `actual` is the actual messages delivered (50), and `threshold` is the delivery threshold (100).

#### Scenario 3: Peer C Delivers Messages Exceeding the Cap
Consider Peer C, which delivers 1500 messages per decay interval, exceeding the `defaultTopicMeshMessageDeliveriesCap` (1000).
In this case, even though Peer C is highly active, its score will not increase further once it hits the cap (1000). This is to avoid overemphasis on high delivery counts, which could skew the scoring system.

#### Scenario 4: Peer D Joins a Topic Mesh
When a new Peer D joins a topic mesh, it will be given a grace period of `defaultMeshMessageDeliveriesActivation` (2 decay intervals) before its message delivery performance is tracked. This grace period allows the peer to set up and begin receiving messages from the network.
Remember, the parameters and scenarios described here aim to maintain a stable, efficient, and secure peer-to-peer network by carefully tracking and scoring each peer's message delivery performance.

#### Scenario 5: Message Delivery Decay
To better understand how the message delivery decay (`defaultTopicMeshMessageDeliveriesDecay`) works in the GossipSub protocol, let's examine a hypothetical scenario.
Let's say we have a peer named `Peer A` who is actively participating in `Topic X`. `Peer A` has successfully delivered 800 messages in `Topic X` over a given time period.
**Initial State**: At this point, `Peer A`'s message delivery count for `Topic X` is 800. Now, the decay interval elapses without `Peer A` delivering any new messages in `Topic X`.
**After One Decay Interval**: Given that our `defaultTopicMeshMessageDeliveriesDecay` value is 0.5, after one decay interval, `Peer A`'s message delivery count for `Topic X` will decay by 50%. Therefore, `Peer A`'s count is now:

    800 (previous message count) * 0.5 (decay factor) = 400

**After Two Decay Intervals**
If `Peer A` still hasn't delivered any new messages in `Topic X` during the next decay interval, the decay is applied again, further reducing the message delivery count:

    400 (current message count) * 0.5 (decay factor) = 200
And this process will continue at every decay interval, halving `Peer A`'s message delivery count for `Topic X` until `Peer A` delivers new messages in `Topic X` or the count reaches zero.
This decay process ensures that a peer cannot rest on its past deliveries; it must continually contribute to the network to maintain its score. 
It helps maintain a lively and dynamic network environment, incentivizing constant active participation from all peers.

### Scenario 6: Replay Attack
The `defaultMeshMessageDeliveriesWindow` and `defaultMeshMessageDeliveriesActivation` parameters play a crucial role in preventing replay attacks in the GossipSub protocol. Let's illustrate this with an example.
Consider a scenario where we have three peers: `Peer A`, `Peer B`, and `Peer C`. All three peers are active participants in `Topic X`.
**Initial State**: At Time = 0: `Peer A` generates and broadcasts a new message `M` in `Topic X`. `Peer B` and `Peer C` receive this message from `Peer A` and update their message caches accordingly.
**After Few Seconds**: At Time = 30 seconds: `Peer B`, with malicious intent, tries to rebroadcast the same message `M` back into `Topic X`. 
Given that our `defaultMeshMessageDeliveriesWindow` value is equal to the decay interval (let's assume 1 minute), `Peer C` would have seen the original message `M` from `Peer A` less than one minute ago.
This is within the `defaultMeshMessageDeliveriesWindow`. Because `Peer A` (the original sender) is different from `Peer B` (the current sender), this delivery will be counted towards `Peer B`'s message delivery score in `Topic X`.
**After One Minute**: At Time = 61 seconds: `Peer B` tries to rebroadcast the same message `M` again. 
Now, more than a minute has passed since `Peer C` first saw the message `M` from `Peer A`. This is outside the `defaultMeshMessageDeliveriesWindow`. 
Therefore, the message `M` from `Peer B` will not count towards `Peer B`'s message delivery score in `Topic X` and `Peer B` still needs to fill up its threshold of message delivery in order not to be penalized for under-performing. 
This effectively discouraging replay attacks of messages older than the `defaultMeshMessageDeliveriesWindow`.
This mechanism, combined with other parameters, helps maintain the security and efficiency of the network by discouraging harmful behaviors such as message replay attacks.

## Mitigating iHave Broken Promises Attacks in GossipSub Protocol
### What is an iHave Broken Promise Attack?
In the GossipSub protocol, peers gossip information about new messages to a subset of random peers (out of their local mesh) in the form of an "iHave" message which basically tells the receiving peer what messages the sender has. 
The receiving peer then replies with an "iWant" message, requesting for the messages it doesn't have. Note that for the peers in local mesh the actual new messages are sent instead of an "iHave" message (i.e., eager push). However,
iHave-iWant protocol is part of a complementary mechanism to ensure that the information is disseminated to the entire network in a timely manner (i.e., lazy pull).

An "iHave Broken Promise" attack occurs when a peer advertises many "iHave" for a message but doesn't respond to the "iWant" requests for those messages.
This not only hinders the effective dissemination of information but can also strain the network with redundant requests. Hence, we stratify it as a spam behavior mounting a DoS attack on the network.

### Detecting iHave Broken Promise Attacks
Detecting iHave Broken Promise Attacks is done by the GossipSub itself. On each incoming RPC from a remote node, the local GossipSub node checks if the RPC contains an iHave message. It then samples one (and only one) iHave message
randomly out of the entire set of iHave messages piggybacked on the incoming RPC. If the sampled iHave message is not literally addressed with the actual message, the local GossipSub node considers this as an iHave broken promise and
increases the behavior penalty counter for that remote node. Hence, incrementing the behavior penalty counter for a remote peer is done per RPC containing at least one iHave broken promise and not per iHave message.
Note that the behavior penalty counter also keeps track of GRAFT flood attacks that are done by a remote peer when it advertises many GRAFTs while it is on a PRUNE backoff by the local node. Mitigating iHave broken promise attacks also
mitigates GRAFT flood attacks.

### Configuring GossipSub Parameters
In order to mitigate the iHave broken promises attacks, GossipSub expects the application layer (i.e., Flow protocol) to properly configure the relevant scoring parameters, notably: 

- `BehaviourPenaltyThreshold` is set to `defaultBehaviourPenaltyThreshold`, i.e., `10`.
- `BehaviourPenaltyWeight` is set to `defaultBehaviourPenaltyWeight`, i.e., `0.01` * `MaxAppSpecificPenalty`
- `BehaviourPenaltyDecay` is set to `defaultBehaviourPenaltyDecay`, i.e., `0.99`.

#### 1. `defaultBehaviourPenaltyThreshold`
This parameter sets the threshold for when the behavior of a peer is considered bad. Misbehavior is defined as advertising an iHave without responding to the iWants (iHave broken promises), and attempting on GRAFT when the peer is considered for a PRUNE backoff.
If a remote peer sends an RPC that advertises at least one iHave for a message but doesn't respond to the iWant requests for that message within the next `3 seconds`, the peer misbehavior counter is incremented by `1`. This threshold is set to `10`, meaning that we at most tolerate 10 such RPCs containing iHave broken promises. After this, the peer is penalized for every excess RPC containing iHave broken promises. The counter decays by (`0.99`) every decay interval (defaultDecayInterval) i.e., every minute.

#### 2. `defaultBehaviourPenaltyWeight`
This is the weight applied as a penalty when a peer's misbehavior goes beyond the `defaultBehaviourPenaltyThreshold`. 
The penalty is applied to the square of the difference between the misbehavior counter and the threshold, i.e., -|w| * (misbehavior counter - threshold)^2, where `|w|` is the absolute value of the `defaultBehaviourPenaltyWeight`. 
Note that `defaultBehaviourPenaltyWeight` is a negative value, meaning that the penalty is applied in the opposite direction of the misbehavior counter. For sake of illustration, we use the notion of `-|w|` to denote that a negative penalty is applied.
We set  `defaultBehaviourPenaltyWeight` to `0.01 * MaxAppSpecificPenalty`, meaning a peer misbehaving `10` times more than the threshold (i.e., `10 + 10`) will lose its entire `MaxAppSpecificReward`, which is a reward given to all staked nodes in Flow blockchain.
This also means that a peer misbehaving `sqrt(2) * 10` times more than the threshold will cause the peer score to be dropped below the `MaxAppSpecificPenalty`, which is also below the `GraylistThreshold`, and the peer will be graylisted (i.e., all incoming and outgoing GossipSub RPCs from and to that peer will be rejected). 
This means the peer is temporarily disconnected from the network, preventing it from causing further harm.

#### 3. defaultBehaviourPenaltyDecay
This is the decay interval for the misbehavior counter of a peer. This counter is decayed by the `defaultBehaviourPenaltyDecay` parameter (e.g., `0.99`) per decay interval, which is currently every 1 minute.
This parameter helps to gradually reduce the effect of past misbehaviors and provides a chance for penalized nodes to rejoin the network. A very slow decay rate can help identify and isolate persistent offenders, while also allowing potentially honest nodes that had transient issues to regain their standing in the network.
The duration a peer remains graylisted is governed by the choice of `defaultBehaviourPenaltyWeight` and the decay parameters. 
Based on the given configuration, a peer which has misbehaved on `sqrt(2) * 10` RPCs more than the threshold will get graylisted (disconnected at GossipSub level).
With the decay interval set to 1 minute and decay value of 0.99, a graylisted peer due to broken promises would be expected to be reconnected in about 50 minutes. 
This is calculated by solving for `x` in the equation `(0.99)^x * (sqrt(2) * 10)^2 * MaxAppSpecificPenalty > GraylistThreshold`. 
Simplifying, we find `x` to be approximately `527` decay intervals, or roughly `527` minutes. 
This is the estimated time it would take for a severely misbehaving peer to have its penalty decayed enough to exceed the `GraylistThreshold` and thus be reconnected to the network.

### Example Scenarios
**Scenario 1: Misbehaving Below Threshold**
In this scenario, consider peer `B` that has recently joined the network and is taking part in GossipSub. 
This peer advertises to peer `A` many `iHave` messages over an RPC. But when other peer `A` requests these message with `iWant`s it fails to deliver the message within 3 seconds. 
This action constitutes an _iHave broken promise_ for a single RPC and peer `A` increases the local behavior penalty counter of peer `B` by 1.
If the peer `B` commits this misbehavior infrequently, such that the total number of these RPCs does not exceed the `defaultBehaviourPenaltyThreshold` (set to 10 in our configuration), 
the misbehavior counter for this peer will increment by 1 for each RPC and decays by `1%` evey decay interval (1 minute), but no additional penalty will be applied. 
The misbehavior counter decays by a factor of `defaultBehaviourPenaltyDecay` (0.99) every minute, allowing the peer to recover from these minor infractions without significant disruption.

**Scenario 2: Misbehaving Above Threshold But Below Graylisting**
Now consider that peer `B` frequently sends RPCs advertising many `iHaves`  to peer `A` but fails to deliver the promised messages. 
If the number of these misbehaviors exceeds our threshold (10 in our configuration), the peer `B` is now penalized by the local GossipSub mechanism of peer `A`. 
The amount of the penalty is determined by the `defaultBehaviourPenaltyWeight` (set to 0.01 * MaxAppSpecificPenalty) applied to the square of the difference between the misbehavior counter and the threshold.
This penalty will progressively affect the peer's score, deteriorating its reputation in the local GossipSub scoring system of node `A`, but does not yet result in disconnection or graylisting. 
The peer has a chance to amend its behavior before crossing into graylisting territory through stop misbehaving and letting the score to decay.
When peer `B` has a deteriorated score at node `A`, it will be less likely to be selected by node `A` as its local mesh peer (i.e., to directly receive new messages from node `A`), and is deprived of the opportunity to receive new messages earlier through node `A`.

**Scenario 3: Graylisting**
Now assume that peer `B` peer has been continually misbehaving, with RPCs including iHave broken promises exceeding `sqrt(2) * 10` the threshold. 
At this point, the peer's score drops below the `GraylistThreshold` due to the `defaultBehaviorPenaltyWeight` applied to the excess misbehavior. 
The peer is then graylisted by peer `A`, i.e., peer `A` rejects all incoming RPCs to and from peer `B` at GossipSub level. 
In our configuration, peer `B` will stay disconnected for at least `527` decay intervals or approximately `527` minutes. 
This gives a strong disincentive for the peer to continue this behavior and also gives it time to recover and eventually be reconnected to the network.

## Customization
The scoring mechanism can be easily customized to suit the needs of the Flow network. This includes changing the scoring parameters, thresholds, and the scoring function itself.
You can customize the scoring parameters and thresholds by using the various setter methods provided in the `ScoreOptionConfig` object. Additionally, you can provide a custom app-specific scoring function through the `SetAppSpecificScoreFunction` method.

**Note**: Usage of non-default app-specific scoring function is not recommended unless you are familiar with the scoring mechanism and the Flow network. It may result in _routing attack vulnerabilities_. It is **always safer** to use the default scoring function unless you know what you are doing.

Example of setting custom app-specific scoring function:
```go
config.SetAppSpecificScoreFunction(customAppSpecificScoreFunction)
```

## Peer Scoring System Integration
The peer scoring system is integrated with the GossipSub protocol through the `ScoreOption` configuration option. 
This option is passed to the GossipSub at the time of initialization.
`ScoreOption` can be used to build scoring options for GossipSub protocol with the desired scoring parameters and thresholds.
```go
flowPubSubOption := scoreOption.BuildFlowPubSubScoreOption()
gossipSubOption := scoreOption.BuildGossipSubScoreOption()
```

# Caching Application Specific Score
![app-specific-score-cache.png](app-specific-score-cache.png)
As the figure above illustrates, GossipSub's peer scoring mechanism invokes the application-specific scoring function on a peer id upon receiving a gossip message from that peer.
This means that the application-specific score of a peer is computed every time a gossip message is received from that peer. 
This can be computationally expensive, especially when the network is large and the number of gossip messages is high. 
As shown by the figure above, each time the application-specific score of a peer is computed, the score is computed from scratch by
computing the spam penalty, staking score and subscription penalty. Each of these computations involves a cache lookup and a computation.
Hence, a single computation of the application-specific score of a peer involves 3 cache lookups and 3 computations.
As the application-specific score of a peer is not expected to change frequently, we can cache the score of a peer and reuse it for a certain period of time. 
This can significantly reduce the computational overhead of the scoring mechanism. 
By caching the application-specific score of a peer, we can reduce the number of cache lookups and computations from 3 to 1 per computation of the application-specific score of a peer, which
results in a 66% reduction in the computational overhead of the scoring mechanism.
The caching mechanism is implemented in the `GossipSubAppSpecificScoreRegistry` struct. Each time the application-specific score of a peer is requested by the GossipSub protocol, the registry
checks if the score of the peer is cached. If the score is cached, the cached score is returned. Otherwise, a score of zero is returned, and a request for the application-specific score of the peer is 
queued to the `appScoreUpdateWorkerPool` to be computed asynchronously. Once the score is computed, it is cached and the score is updated in the `appScoreCache`. 
Each score record in the cache is associated with a TTL (time-to-live) value, which is the duration for which the score is valid. 
When the retrieved score is expired, the expired score is still returned to the GossipSub protocol, but the score is updated asynchronously in the background by submitting a request to the `appScoreUpdateWorkerPool`.
The app-specific score configuration values are configurable through the following parameters in the `default-config.yaml` file:
```yaml
    scoring-parameters:
      app-specific-score:
        # number of workers that asynchronously update the app specific score requests when they are expired.
        score-update-worker-num: 5
        # size of the queue used by the worker pool for the app specific score update requests. The queue is used to buffer the app specific score update requests
        # before they are processed by the worker pool. The queue size must be larger than total number of peers in the network.
        # The queue is deduplicated based on the peer ids ensuring that there is only one app specific score update request per peer in the queue.
        score-update-request-queue-size: 10_000
        # score ttl is the time to live for the app specific score. Once the score is expired; a new request will be sent to the app specific score provider to update the score.
        # until the score is updated, the previous score will be used.
        score-ttl: 1m
```