# Cruise Control: Automated Block Time Adjustment for Precise Epoch Switchover Timing

# Overview

## Context

Epochs have a fixed length, measured in views.
The actual view rate of the network varies depending on network conditions, e.g. load, number of offline replicas, etc.
We would like for consensus nodes to observe the actual view rate of the committee, and adjust how quickly they proceed
through views accordingly, to target a desired weekly epoch switchover time.

## High-Level Design

The `BlockTimeController` observes the current view rate and adjusts the timing when the proposal should be released.
It is a [PID controller](https://en.wikipedia.org/wiki/PID_controller). The essential idea is to take into account the
current error, the rate of change of the error, and the cumulative error, when determining how much compensation to apply.
The compensation function $u[v]$ has three terms:

- $P[v]$ compensates proportionally to the magnitude of the instantaneous error
- $I[v]$ compensates proportionally to the magnitude of the error and how long it has persisted
- $D[v]$ compensates proportionally to the rate of change of the error


📚 This document uses ideas from:

- the paper [Fast self-tuning PID controller specially suited for mini robots](https://www.frba.utn.edu.ar/wp-content/uploads/2021/02/EWMA_PID_7-1.pdf)
- the ‘Leaky Integrator’ [[forum discussion](https://engineering.stackexchange.com/questions/29833/limiting-the-integral-to-a-time-window-in-pid-controller), [technical background](https://www.music.mcgill.ca/~gary/307/week2/node4.html)]


### Choice of Process Variable: Targeted Epoch Switchover Time

The process variable is the variable which:

- has a target desired value, or setpoint ($SP$)
- is successively measured by the controller to compute the error $e$

---
👉 The `BlockTimeController` controls the progression through views, such that the epoch switchover happens at the intended point in time. We define:

- $\gamma = k\cdot \tau_0$ is the remaining epoch duration of a hypothetical ideal system, where *all* remaining $k$ views of the epoch progress with the ideal view time  $\tau_0$.
- The parameter $\tau_0$ is computed solely based on the Epoch configuration as
  $\tau_0 := \frac{<{\rm total\ epoch\ time}>}{<{\rm total\ views\ in\ epoch}>}$ (for mainnet 22, Epoch 75, we have $\tau_0 \simeq$  1250ms).
- $\Gamma$ is the *actual* time remaining until the desired epoch switchover.

The error, which the controller should drive towards zero, is defined as:

```math
e := \gamma - \Gamma
```
---


From our definition it follows that:

- $e > 0$  implies that the estimated epoch switchover (assuming ideal system behaviour) happens too late. Therefore, to hit the desired epoch switchover time, the time we spend in views has to be *smaller* than $\tau_0$.
- For $e < 0$  means that we estimate the epoch switchover to be too early. Therefore, we should be slowing down and spend more than $\tau_0$ in the following views.

**Reasoning:**

The desired idealized system behaviour would a constant view duration $\tau_0$ throughout the entire epoch.

However, in the real-world system we have disturbances (varying message relay times, slow or offline nodes, etc) and measurement uncertainty (node can only observe its local view times, but not the committee’s collective swarm behaviour).

<img src='https://github.com/onflow/flow-go/blob/master/docs/CruiseControl_BlockTimeController/PID_controller_for_block-rate-delay.png' width='680'>


After a disturbance, we want the controller to drive the system back to a state, where it can closely follow the ideal behaviour from there on.

- Simulations have shown that this approach produces *very* stable controller with the intended behaviour.

  **Controller driving  $e := \gamma - \Gamma \rightarrow 0$**
    - setting the differential term $K_d=0$, the controller responds as expected with damped oscillatory behaviour
      to a singular strong disturbance. Setting $K_d=3$ suppresses oscillations and the controller's performance improves as it responds more effectively.

      <img src='https://github.com/onflow/flow-go/blob/master/docs/CruiseControl_BlockTimeController/EpochSimulation_029.png' width='900'>

      <img src='https://github.com/onflow/flow-go/blob/master/docs/CruiseControl_BlockTimeController/EpochSimulation_030.png' width='900'>

    - controller very quickly compensates for moderate disturbances and observational noise in a well-behaved system:

      <img src='https://github.com/onflow/flow-go/blob/master/docs/CruiseControl_BlockTimeController/EpochSimulation_028.png' width='900'>

    - controller compensates massive anomaly (100s network partition) effectively:

      <img src='https://github.com/onflow/flow-go/blob/master/docs/CruiseControl_BlockTimeController/EpochSimulation_000.png' width='900'>

    - controller effectively stabilizes system with continued larger disturbances (20% of offline consensus participants) and notable observational noise:

      <img src='https://github.com/onflow/flow-go/blob/master/docs/CruiseControl_BlockTimeController/EpochSimulation_005-0.png' width='900'>

  **References:**

    - statistical model for happy-path view durations: [ID controller for ``block-rate-delay``](https://www.notion.so/ID-controller-for-block-rate-delay-cc9c2d9785ac4708a37bb952557b5ef4?pvs=21)
    - For Python implementation with additional disturbances (offline nodes) and observational noise, see GitHub repo: [flow-internal/analyses/pacemaker_timing/2023-05_Blocktime_PID-controller](https://github.com/dapperlabs/flow-internal/tree/master/analyses/pacemaker_timing/2023-05_Blocktime_PID-controller) → [controller_tuning_v01.py](https://github.com/dapperlabs/flow-internal/blob/master/analyses/pacemaker_timing/2023-05_Blocktime_PID-controller/controller_tuning_v01.py)

# Detailed PID controller specification

Each consensus participant runs a local instance of the controller described below. Hence, all the quantities are based on the node’s local observation.

## Definitions

**Observables** (quantities provided to the node or directly measurable by the node):

- $v$ is the node’s current view
- ideal view time $\tau_0$ is computed solely based on the Epoch configuration:
  $\tau_0 := \frac{<{\rm total\ epoch\ time}>}{<{\rm total\ views\ in\ epoch}>}$  (for mainnet 22, Epoch 75, we have $\tau_0 \simeq$  1250ms).
- $t[v]$ is the time the node entered view $v$
- $F[v]$  is the final view of the current epoch
- $T[v]$ is the target end time of the current epoch

**Derived quantities**

- remaining views of the epoch $k[v] := F[v] +1 - v$
- time remaining until the desired epoch switchover $\Gamma[v] := T[v]-t[v]$
- error $e[v] := \underbrace{k\cdot\tau_0}_{\gamma[v]} - \Gamma[v] = t[v] + k[v] \cdot\tau_0 - T[v]$

### Precise convention of View Timing

Upon observing block `B` with view $v$, the controller updates its internal state.

Note the '+1' term in the computation of the remaining views $k[v] := F[v] +1 - v$  . This is related to our convention that the epoch begins (happy path) when observing the first block of the epoch. Only by observing this block, the nodes transition to the first view of the epoch. Up to that point, the consensus replicas remain in the last view of the previous epoch, in the state of `having processed the last block of the old epoch and voted for it` (happy path). Replicas remain in this state until they see a confirmation of the view (either QC or TC for the last view of the previous epoch).

<img src='https://github.com/onflow/flow-go/blob/master/docs/CruiseControl_BlockTimeController/ViewDurationConvention.png' width='600'>

In accordance with this convention, observing the proposal for the last view of an epoch, marks the start of the last view. By observing the proposal, nodes enter the last view, verify the block, vote for it, the primary aggregates the votes, constructs the child (for first view of new epoch). The last view of the epoch ends, when the child proposal is published.

### Controller

The goal of the controller is to drive the system towards an error of zero, i.e. $e[v] \rightarrow 0$. For a [PID controller](https://en.wikipedia.org/wiki/PID_controller), the output $u$ for view $v$ has the form:

```math
u[v] = K_p \cdot e[v]+K_i \cdot \mathcal{I}[v] + K_d \cdot \Delta[v]
```

With error terms (computed from observations)

- $e[v]$ representing the *instantaneous* error as of view $v$
  (commonly referred to as ‘proportional term’)
- $\mathcal{I} [v] = \sum_v e[v]$ the sum of the errors
  (commonly referred to as ‘integral term’)
- $\Delta[v]=e[v]-e[v-1]$ the rate of change of the error
  (commonly referred to as ‘derivative term’)

and controller parameters (values derived from controller tuning):

- $K_p$ be the proportional coefficient
- $K_i$ be the integral coefficient
- $K_d$ be the derivative coefficient

## Measuring view duration

Each consensus participant observes the error $e[v]$ based on its local view evolution. As the following figure illustrates, the view duration is highly variable on small time scales.

![](/docs/CruiseControl_BlockTimeController/ViewRate.png)

Therefore, we expect $e[v]$ to be very variable. Furthermore, note that a node uses its local view transition times as an estimator for the collective behaviour of the entire committee. Therefore, there is also observational noise obfuscating the underlying collective behaviour. Hence, we expect notable noise.

## Managing noise

Noisy values for $e[v]$ also impact the derivative term $\Delta[v]$ and integral term $\mathcal{I}[v]$. This can impact the controller’s performance.

### **Managing noise in the proportional term**

An established approach for managing noise in observables is to use [exponentially weighted moving average [EWMA]](https://en.wikipedia.org/wiki/Moving_average) instead of the instantaneous values.  Specifically, let $\bar{e}[v]$ denote the EWMA of the instantaneous error, which is computed as follows:

```math
\eqalign{
\textnormal{initialization: }\quad \bar{e} :&= 0 \\
\textnormal{update with instantaneous error\ } e[v]:\quad \bar{e}[v] &= \alpha \cdot e[v] + (1-\alpha)\cdot \bar{e}[v-1]
}
```

The parameter $\alpha$ relates to the averaging time window. Let $\alpha \equiv \frac{1}{N_\textnormal{ewma}}$ and consider that the input changes from $x_\textnormal{old}$ to $x_\textnormal{new}$ as a step function. Then $N_\textnormal{ewma}$ is the number of samples required to move the output average about 2/3 of the way from  $x_\textnormal{old}$ to $x_\textnormal{new}$.

see also [Python `Ewma` implementation](https://github.com/dapperlabs/flow-internal/blob/423d927421c073e4c3f66165d8f51b829925278f/analyses/pacemaker_timing/2023-05_Blocktime_PID-controller/controller_tuning_v01.py#L405-L431)

### **Managing noise in the integral term**

In particular systematic observation bias are a problem, as it leads to a diverging integral term. The commonly adopted approach is to use a ‘leaky integrator’ [[1](https://www.music.mcgill.ca/~gary/307/week2/node4.html), [2](https://engineering.stackexchange.com/questions/29833/limiting-the-integral-to-a-time-window-in-pid-controller)], which we denote as $\bar{\mathcal{I}}[v]$.

```math
\eqalign{
\textnormal{initialization: }\quad \bar{\mathcal{I}} :&= 0 \\
\textnormal{update with instantaneous error\ } e[v]:\quad \bar{\mathcal{I}}[v] &= e[v] + (1-\lambda)\cdot\bar{\mathcal{I}}[v-1]
}
```

Intuitively, the loss factor $\lambda$ relates to the time window of the integrator. A factor of 0 means an infinite time horizon, while $\lambda =1$  makes the integrator only memorize the last input. Let  $\lambda \equiv \frac{1}{N_\textnormal{itg}}$ and consider a constant input value $x$. Then $N_\textnormal{itg}$ relates to the number of past samples that the integrator remembers:

- the integrators output will saturate at $x\cdot N_\textnormal{itg}$
- an integrator initialized with 0, reaches 2/3 of the saturation value $x\cdot N_\textnormal{itg}$ after consuming $N_\textnormal{itg}$ inputs

see also [Python `LeakyIntegrator` implementation](https://github.com/dapperlabs/flow-internal/blob/423d927421c073e4c3f66165d8f51b829925278f/analyses/pacemaker_timing/2023-05_Blocktime_PID-controller/controller_tuning_v01.py#L444-L468)

### **Managing noise in the derivative term**

Similarly to the proportional term, we apply an EWMA to the differential term and denote the averaged value as $\bar{\Delta}[v]$:

```math
\eqalign{
\textnormal{initialization: }\quad \bar{\Delta} :&= 0 \\
\textnormal{update with instantaneous error\ } e[v]:\quad \bar{\Delta}[v] &= \bar{e}[v] - \bar{e}[v-1]
}
```

## Final formula for PID controller

We have used a statistical model of the view duration extracted from mainnet 22 (Epoch 75) and manually added disturbances and observational noise and systemic observational bias.

The following parameters have proven to generate stable controller behaviour over a large variety of network conditions:

---
👉 The controller is given by

```math
u[v] = K_p \cdot \bar{e}[v]+K_i \cdot \bar{\mathcal{I}}[v] + K_d \cdot \bar{\Delta}[v]
```

with parameters:

- $K_p = 2.0$
- $K_i = 0.6$
- $K_d = 3.0$
- $N_\textnormal{ewma} = 5$, i.e. $\alpha = \frac{1}{N_\textnormal{ewma}} = 0.2$
- $N_\textnormal{itg} = 50$, i.e.  $\lambda = \frac{1}{N_\textnormal{itg}} = 0.02$

The controller output $u[v]$ represents the amount of time by which the controller wishes to deviate from the ideal view duration $\tau_0$. In other words, the duration of view $v$ that the controller wants to set is
```math
\widehat{\tau}[v] = \tau_0 - u[v]
```
---    

### Limits of authority

[Latest update: Crescendo Upgrade, June 2024]

In general, there is no bound on the output of the controller output $u$. However, it is important to limit the controller’s influence to keep $u$ within a sensible range.

- upper bound on view duration $\widehat{\tau}[v]$ that we allow the controller to set:

  The current timeout threshold is set to 1045ms and the largest view duration we want to allow the controller to set is $\tau_\textrm{max}$ = 910ms.
  Thereby, we have a buffer $\beta$ = 135ms remaining for message propagation and the replicas validating the proposal for view $v$.

  Note the subtle but important aspect: Primary for view $v$ controls duration of view $v-1$. This is because its proposal for view $v$
  contains the proof (Quorum Certificate [QC]) that view $v-1$ concluded on the happy path. By observing the QC for view $v-1$, nodes enter the
  subsequent view $v$.


- lower bound on the view duration:

  Let $t_\textnormal{p}[v]$ denote the time when the primary for view $v$ has constructed its block proposal.
  On the happy path, a replica concludes view $v-1$ and transitions to view $v$, when it observes the proposal for view $v$.
  The duration $t_\textnormal{p}[v] - t[v-1]$ is the time between the primary observing the parent block (view $v-1$), collecting votes,
  constructing a QC for view $v-1$, and subsequently its own proposal for view $v$. This duration is the minimally required time to execute the protocol.
  The controller can only *delay* broadcasting the block,
  but it cannot release the block before  $t_\textnormal{p}[v]$ simply because the proposal isn’t ready any earlier.



👉 Let $\hat{t}[v]$ denote the time when the primary for view $v$ *broadcasts* its proposal. We assign:

```math
\hat{t}[v] := \max\Big(t[v-1] +\min(\widehat{\tau}[v-1],\ \tau_\textrm{max}),\  t_\textnormal{p}[v]\Big) 
```
This equation guarantees that the controller does not drive consensus into a timeout, as long as broadcasting the block and its validation
together require less than time $\beta$. Currently, we have $\tau_\textrm{max}$ = 910ms as the upper bound for view durations that the controller can set.
In comparison, for HotStuff's timeout threshold we set $\texttt{hotstuff-min-timeout} = \tau_\textrm{max} + \beta$, with $\beta$ = 135ms.



### Further reading

- the statistical model of the view duration, see [PID controller for ``block-rate-delay``](https://www.notion.so/ID-controller-for-block-rate-delay-cc9c2d9785ac4708a37bb952557b5ef4?pvs=21)
- the simulation and controller tuning, see  [flow-internal/analyses/pacemaker_timing/2023-05_Blocktime_PID-controller](https://github.com/dapperlabs/flow-internal/tree/master/analyses/pacemaker_timing/2023-05_Blocktime_PID-controller) → [controller_tuning_v01.py](https://github.com/dapperlabs/flow-internal/blob/master/analyses/pacemaker_timing/2023-05_Blocktime_PID-controller/controller_tuning_v01.py)
- The most recent parameter setting was derived here:
    - [Cruise-Control headroom for speedups](https://www.notion.so/flowfoundation/Cruise-Control-headroom-for-speedups-46dc17e07ae14462b03341e4432a907d?pvs=4) contains the formal analysis and discusses the numerical results in detail
    - Python code for figures and calculating the final parameter settings: [flow-internal/analyses/pacemaker_timing/2024-03_Block-timing-update](https://github.com/dapperlabs/flow-internal/tree/master/analyses/pacemaker_timing/2024-03_Block-timing-update) → [timeout-attacks.py](https://github.com/dapperlabs/flow-internal/blob/master/analyses/pacemaker_timing/2024-03_Block-timing-update/timeout-attacks.py)


## Edge Cases

### A node is catching up

When a node is catching up, it observes the blocks significantly later than they were published. In other words, from the perspective
of the node catching up, the blocks are too late. However, as it reaches the most recent blocks, also the observed timing error approaches zero
(assuming approximately correct block publication by the honest supermajority). Nevertheless, due to its biased error observations, the node
catching up could still try to compensate for the network being behind, and publish its proposal as early as possible.

**Assumption:** With only a smaller fraction of nodes being offline or catching up, the effect is expected to be small and easily compensated for by the supermajority of online nodes.

### A node has a misconfigured clock

Cap the maximum deviation from the default delay (limits the general impact of error introduced by the `BlockTimeController`). The node with misconfigured clock will contribute to the error in a limited way, but as long as the majority of nodes have an accurate clock, they will offset this error.

**Assumption:** With only a smaller fraction of nodes having misconfigured clocks, the effect will be small enough to be easily compensated for by the supermajority of correct nodes.

### Near epoch boundaries

We might incorrectly compute high error in the target view rate, if local current view and current epoch are not exactly synchronized. By default, they would not be, because `EpochTransition` events occur upon finalization, and current view is updated as soon as QC/TC is available.

**Solution:** determine epoch locally based on view only, do not use `EpochTransition` event.

### EFM

When the network is in EFM, epoch timing is anyway disrupted. The main thing we want to avoid is that the controller drives consensus into a timeout.
This is largely guaranteed, due to the limits of authority. Beyond that, pretty much any block timing on the happy path is acceptable.
Through, the optimal solution would be a consistent view time throughout normal Epochs as well as EFM.

# Implementation Aspects

## Timing Reference Points

<img src='https://github.com/user-attachments/assets/8c28d4d1-0b6c-4b6f-8413-c0753e7400f9' width='900'>


* Under the hood, the controller outputs the unconstrained view time $\widehat{\tau}[v]= \tau_0 - u[v]$ (of type [`time.Duration`](https://pkg.go.dev/time)),
  which is wrapped into a [`happyPathBlockTime`](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/consensus/hotstuff/cruisectl/proposal_timing.go#L59-L74)
  (👉 [code](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/consensus/hotstuff/cruisectl/block_time_controller.go#L402-L404)).
  The [`happyPathBlockTime`](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/consensus/hotstuff/cruisectl/proposal_timing.go#L59-L74)
  [applies the limits of authority](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/consensus/hotstuff/cruisectl/proposal_timing.go#L94),
  and the resulting `ConstrainedBlockTime` we capture by the metric `Average Target View Time` (blue dotted curve in figure above).
* From taking a look at the [`hotatuff.EventHandler`](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/consensus/hotstuff/eventhandler/event_handler.go#L157-L171),
  we can confirm that the `BlockTimeController` and the metric `Observed View Time` use practically the same reference time to determine view progression:
  ```golang
  func (e *EventHandler) OnReceiveProposal(proposal *model.Proposal) error {
    ⋮
    
    // store the block.
    err := e.forks.AddValidatedBlock(block)
    if err != nil {
      return fmt.Errorf("cannot add proposal to forks (%x): %w", block.BlockID, err)
    }

    _, err = e.paceMaker.ProcessQC(proposal.Block.QC)
    if err != nil {
      return fmt.Errorf("could not process QC for block %x: %w", block.BlockID, err)
    }

    _, err = e.paceMaker.ProcessTC(proposal.LastViewTC)
    if err != nil {
      return fmt.Errorf("could not process TC for block %x: %w", block.BlockID, err)
    }

    ⋮
  ```
    - The call to `forks.AddValidatedBlock` emits the `OnBlockIncorporated` notification for `block` with view $v$, which the `BlockTimeController` uses as [its starting point for the view](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/consensus/hotstuff/cruisectl/block_time_controller.go#L476-L481):
      ```golang
      TimedBlock{Block: block, TimeObserved: time.Now().UTC()}
      ```
      So for the `BlockTimeController`, the start of view `v` is the `TimeObserved` for proposal with view `v`.
    - Right after, the PaceMaker ingests the QC for view `v-1`, which is included in the proposal for view `v`.
      The call to `paceMaker.ProcessQC` updates the metric `consensus_hotstuff_cur_view`, based on which we calculate `Observed Average (10m) View Time [s]` (blue solid curve in figure above).
* As the `TargetPublicationTime` for block `v+1`, the `BlockTimeController` [calculates](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/consensus/hotstuff/cruisectl/proposal_timing.go#L102-L112):
  ```golang
  targetPublicationTime := TimeObserved.Add(ConstrainedBlockTime)
  ``` 
* The [`EventHandler` triggers](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/consensus/hotstuff/eventhandler/event_handler.go#L390-L417) the
  [computation](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/consensus/hotstuff/cruisectl/block_time_controller.go#L249-L262) of the
  `Block Publication Delay` metric (dashed purple curve) right when it hands its proposal for view `v+1` to the [`MessageHub`](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/engine/consensus/message_hub/message_hub.go#L428-L434)
  ```golang
  publicationDelay := time.Until(targetPublicationTime)
  if publicationDelay < 0 {
    publicationDelay = 0 // Controller can only delay publication of proposal. Hence, the delay is lower-bounded by zero.
  }
  metrics.ProposalPublicationDelay(publicationDelay)
  ```
* The [`MessageHub` repeats exactly that computation of `publicationDelay`](https://github.com/onflow/flow-go/blob/d9f7522d6c502d7e148dab69c926279202677cf8/engine/consensus/message_hub/message_hub.go#L433-L437)
  to determine how long it should sleep before broadcasting the proposal.

**Estimator for consensus runtime**:
- The `Observed Average View Time` measurement starts when we see the parent block while `Block Publication Delay` starts to measure when the child block is constructed. 
  `Observed Average View Time` and `Block Publication Delay` use nearly identical temporal reference points to stop their respective measurement (discrepancy is smaller than 1ms based on prior benchmarks of the `EventHandler`).
  Therefore, the following is an estimator for how long it takes for the protocol to complete one view on the happy path (running it fast as it possibly can without any delays):
  ```math
   \texttt{Observed Average View Time} - \texttt{Block Publication Delay}
   ```
- There are some additional computation steps that we haven't accounted for, which could introduce errors. However, the `EventHandler` is very fast with execution times of single-digit milliseconds in practise.
  Hence, the uncertainty of this estimator is minimal (expected order of single milliseconds).  


## Controller's observation of blocks (`OnBlockIncorporated`) must approximately match the time when other replicas observe the proposal

On Testnet (`devnet51`) August 14-18, 2024 we observed the following discrepancy for a consensus committee of 7 nodes:
* There was a significant discrepancy between the `Observed Average View Time` vs the `Average Target View Time`:
  <img src='https://github.com/user-attachments/assets/3820497b-e134-4c50-8f8d-d710997ff646' width='700'>
* This resulted in the controller having reduced limits of authority: the limits of authority are computed based on the `Target View Time`, which the controller 
  set to the largest permitted value of 910ms (higher values were not allowed to prevent the controller from driving consensus into timeouts). However, in reality
  the view progression was notably faster, meaning consensus would have tolerated larger delays without risk of timeouts. 

### Learning

This discrepancy between what the controller was setting vs the networks real-world response was due to a systematic observation bias:
* For **small consensus committee sizes**, a node being selected as **leader for successive rounds** needs to be taken into account. In this scenario, 
  it is especially important that the leader "observes its own proposal" approximately at the same time when the other replicas receive it. 
* Having the leader's cruise control "observes the own proposal" at the time when the proposal is constructed, before adding the controller's delay, would
  introduce a significant observation bias. In this case, only the leader would assume that the view has started much earlier compared to the other replicas.
  Therefore, it assumes it has much less time until other nodes would presumably trigger their timeout. Hence, the node erroneously restricts the possible delay
  it can impose before broadcasting the child block. 
* For larger consensus committee sizes, this systematic observation bias persists, but the probability of is much lower: The probability for a leader to be selected
  as leader decreases with $\frac{1}{n}$ for $n$ the committee size. As the bias only affects the second of the two consecutive views, the overall impact of this 
  bias declines with increasing consensus committee size.  

## Initial Testing

see [Cruise Control: Benchnet Testing Notes](https://www.notion.so/Cruise-Control-Benchnet-Testing-Notes-ea08f49ba9d24ce2a158fca9358966df?pvs=21)
