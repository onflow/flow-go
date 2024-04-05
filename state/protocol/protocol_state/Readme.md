# Dynamic Protocol State in a nutshell

- The Dynamic Protocol State is a framework for storing a snapshot of protocol-defined parameters
  and supplemental protocol-data into each block. Think about it as a key value store in each block. 
- The Flow network uses its Dynamic Protocol State to orchestrate Epoch switchovers and more generally control participation privileges
  for the network (including ejection of misbehaving or compromised nodes).
- Furthermore, the Dynamic Protocol State makes it easily possible update operational protocol parameters on the live network via a governance transaction.
  For example, we could update consensus timing parameters such as `hotstuff-min-timeout`.

These examples from above all use the same primitives provided by the Dynamic Protocol State: 
 - (i) a Key-value store, whose hash-commitment is included at the end of every block,
 - (ii) a set of rules (represented as a state machine) that updates the key-value-store from block to block, and
 - (iii) dedicated `Service Events` originating from the System Smart Contracts (via verification and sealing) are the inputs to the state machines (ii).  

This provides us with a very powerful set of primitives to orchestrate the low-level protocol on the fly with inputs from the System Smart Contracts.
Engineers extending the protocol can add new entries to the Key-Value Store and provide custom state machines for updating their values.
Correct application of this state machine (i.e. correct evolution of data in the store) is guaranteed by the Dynamic Protocol State framework through BFT consensus. 


# Core Concepts

## Orthogonal State Machines

Orthogonality means that state machines can operate completely independently and work on disjoint
sub-states. By convention, they all consume the same inputs (incl. the ordered sequence of
Service Events sealed in one block). In other words, each state machine $S_0, S_1,\ldots$ has full visibility
into the inputs, but each draws their on independent conclusions (maintaining their own exclusive state).
There is no information exchange between the state machines; one state machines cannot read the current state
of another.

We emphasize that this architecture choice does not prevent us of from implementing sequential state
machines for certain use-cases. For example: state machine $A$ provides its output as input to another
state machine $B$. Here, the order of running the state machines matters. This order-dependency is not
supported by the Protocol State, which executes the state machines in an arbitrary order. Therefore,
if we need state machines to be executed in some specific order, we have to bundle them into one composite state
machine (conceptually a processing pipeline) by hand. The composite state machine's execution as a
whole can then be managed by the Protocol State, because the composite state machine is orthogonal
to all other remaining state machines.
Requiring all State Machines to be orthogonal is a deliberate design choice. Thereby the default is
favouring modularity and strong logical independence. This is very beneficial for managing complexity
in the long term.

### Key-Value-Store:
The Flow protocol defines the Key-Value-Store's state $\mathcal{P}$ as the composition of disjoint sub-states
$P_0, P_1, \ldots, P_j$. Formally, we write $\mathcal{P} = P0 \otimes P1 \otimes \ldots \otimes Pj$, where $'\otimes'$ denotes the product state. We
loosely associate each $P_0, P_1,\ldots$ with one specific key-value entry in the store. Correspondingly,
we have conceptually independent state machines $S_0, S_1,\ldots$ operating each on their own respective
sub-state $P_0, P_1, \ldots$ A one-to-one correspondence between key-value-pair and state machine should be the
default, but is not strictly required. However, the strong requirement is that no key-value-pair is operated
on my more than one state machine.

Formally we write:
- The overall protocol state $\mathcal{P}$ is composed of disjoint substates  $\mathcal{P} = P0 \otimes P1 \otimes\ldots\otimes Pj$
- For each state $P_i$, we have a dedicated state machine $S_i$ that exclusively operates on $P_i$
- The state machines can be formalized as orthogonal regions of the composite state machine
  $\mathcal{S} = S_0 \otimes S_1 \otimes \ldots \otimes S_j$. (Technically, we represent the state machine by its state-transition
  function. All other details of the state machine are implicit.)
- The state machine $\mathcal{S}$ being in state $\mathcal{P}$ and observing the input $\xi = x_0\cdot x_1 \cdot x_2 \cdot\ldots\cdot x_z$ will output
  state $\mathcal{P}'$. To emphasize that a certain state machine 𝒮 exclusively operates on state $\mathcal{P}$, we write
  $\mathcal{S}[\mathcal{P}] = S_0[P_0] \otimes S_1[P_1] \otimes\ldots\otimes S_j[P_j]$.
  Observing the events $\xi$, the output state $\mathcal{P}'$ is
  $\mathcal{P}' = \mathcal{S}[\mathcal{P}](\xi) = S_0[P_0](\xi) \otimes S_1[P_1](\xi) \otimes\ldots\otimes S_j[P_j](\xi)$
  $ = P_0' \otimes P_1' \otimes\ldots\otimes P_j'$
  Where each state machine Si individually generated the output state $S_i[P_i](\xi) = P_i'$

Input $\xi$:
Conceptually, the consensus leader first executes these state machines during their block building
process. At this point, the ID of the final block is unknown. Nevertheless, some part of the payload
construction already happened, because at the sealed execution results are used as an input below.
There is a large degree of freedom what part of the block we permit as possible inputs to the state
machines. At the moment, the primary purpose is for the execution environment (with results undergone
verification and sealing) to send Service Events to the protocol layer. Therefore, the current
convention is:
1. At time of state machine construction (for each block), the Protocol State framework provides:
   • candidateView: view of the block currently under construction
   • parentID: parent block's ID (generally used by state machines to read their respective sub-state)
2. The Service Events sealed in the candidate block (under construction)
   are given to each state machine via the `EvolveState(..)` call.
   CAUTION: `EvolveState(..)` MUST be called for all candidate blocks, even if there are no seals
   (or an empty payload).

The Protocol State is the framework, which orchestrates the orthogonal state machines,
feeds them with inputs, post-processes the outputs and overall manages state machines' life-cycle
from block to block. New key-value pairs and corresponding state machines can easily be added
by implementing the following interface (state machine) and adding a new entry to the KV store.
