package flow

// DKGEndState captures the final state of a completed DKG.
type DKGEndState uint32

const (
	// DKGEndStateUnknown - zero value for this enum, indicates unset value
	DKGEndStateUnknown DKGEndState = iota
	// DKGEndStateSuccess - the DKG completed, this node has a valid beacon key.
	DKGEndStateSuccess
	// DKGEndStateInconsistentKey - the DKG completed, this node has an invalid beacon key.
	DKGEndStateInconsistentKey
	// DKGEndStateNoKey - this node did not store a key, typically caused by a crash mid-DKG.
	DKGEndStateNoKey
	// DKGEndStateDKGFailure - the underlying DKG library reported an error.
	DKGEndStateDKGFailure
)

func (state DKGEndState) String() string {
	switch state {
	case DKGEndStateSuccess:
		return "DKGEndStateSuccess"
	case DKGEndStateInconsistentKey:
		return "DKGEndStateInconsistentKey"
	case DKGEndStateNoKey:
		return "DKGEndStateNoKey"
	case DKGEndStateDKGFailure:
		return "DKGEndStateDKGFailure"
	default:
		return "DKGEndStateUnknown"
	}
}

// DKGIndexMap describes the membership of the DKG committee 𝒟. Flow's random beacon utilizes
// a threshold signature scheme, which requires a Distributed Key Generation [DKG] to generate the
// key shares for each committee member. In the formal cryptographic protocol for DKG with n parties,
// the individual participants are solely identified by indices {0, 1, ..., n-1} and the fact that these
// are non-negative integer values is actively used by the DKG protocol. Accordingly, our implementation
// of the lower-level cryptographic primitives work with these DKG index values.
// On the protocol level, only consensus nodes (identified by their nodeIDs) are allowed to contribute
// random beacon signature shares. Hence, the protocol level needs to map nodeIDs to DKG indices when
// calling into the lower-level cryptographic primitives.
//
// Formal specification:
//   - DKGIndexMap completely describes the DKG committee. If there were n parties authorized to participate
//     in the DKG, DKGIndexMap must contain exactly n elements, i.e. n = len(DKGIndexMap)
//   - The values in DKGIndexMap must form the set {0, 1, …, n-1}.
//
// CAUTION: It is important to cleanly differentiate between the consensus committee 𝒞, the random beacon
// committee ℛ and the DKG committee 𝒟:
//   - For an epoch, the consensus committee 𝒞 contains all nodes that are authorized to vote for blocks. Authority
//     to vote (i.e. membership in the consensus committee) is irrevocably granted for an epoch (though, honest nodes
//     will reject votes and proposals from ejected nodes; nevertheless, ejected nodes formally remain members of
//     the consensus committee).
//   - Only consensus nodes are allowed to contribute to the random beacon. We define the random beacon committee ℛ
//     as the subset of the consensus nodes, which _successfully_ completed the DKG. Hence, ℛ ⊆ 𝒞.
//   - Lastly, there is the DKG committee 𝒟, which is the set of parties that were authorized to
//     participate in the DKG. Mathematically, the DKGIndexMap is an injective function
//     DKGIndexMap: 𝒟 ↦ {0,1,…,n-1}.
//
// The protocol explicitly ALLOWS additional parties outside the current epoch's consensus committee to participate.
// In particular, there can be a key-value pair (d,i) ∈ DKGIndexMap, such that the nodeID d is *not* a consensus
// committee member, i.e. d ∉ 𝒞. In terms of sets, this implies we must consistently work with the relatively
// general assumption that 𝒟 \ 𝒞 ≠ ∅ and 𝒞 \ 𝒟 ≠ ∅.
// Nevertheless, in the vast majority of cases (happy path, roughly 98% of epochs) it will be the case that 𝒟 = 𝒞.
// Therefore, we can optimize for the case 𝒟 = 𝒞, as long as we still support the more general case 𝒟 ≠ 𝒞.
// Broadly, this makes the protocol more robust against temporary disruptions and sudden, large fluctuations in node
// participation.
// Nevertheless, there is an important liveness constraint: the intersection, 𝒟 ∩ 𝒞 = ℛ should be a larger number of
// nodes. Specifically, an honest supermajority of consensus nodes must contain enough successful DKG participants
// (about n/2) to produce a valid group signature for the random beacon [1, 3]. Therefore, we have the approximate
// lower bound that |ℛ| = |𝒟 ∩ 𝒞| ≤ n/2 = |𝒟|/2 = len(DKGIndexMap)/2. Operating close to this lower bound would
// require that every random beacon key-holder r ∈ ℛ remaining in the consensus committee is honest
// (incl. quickly responsive) *all the time*. This is a lower bound, unsuited for decentralized production networks.
// To reject configurations that are vulnerable to liveness failures, the protocol uses the threshold `t_safety`
// (heuristic, see [2]), which is implemented on the smart contract level. In a nutshell, the intersection 𝒟 ∩ 𝒞
// (wrt both sets 𝒟 ∩ 𝒞) should be well above 70%, values in the range 70-62% should be considered for short-term
// recovery cases. Values of 62% or lower (i.e. |ℛ| ≤ 0.62·|𝒟| or |ℛ| ≤ 0.62·|𝒞|) are not recommend for any
// production network, as single-node crashes are already enough to halt consensus.
//
// For further details, see
//   - [1] https://www.notion.so/flowfoundation/Threshold-Signatures-7e26c6dd46ae40f7a83689ba75a785e3?pvs=4
//   - [2] https://www.notion.so/flowfoundation/DKG-contract-success-threshold-86c6bf2b92034855b3c185d7616eb6f1?pvs=4
//   - [3] https://www.notion.so/flowfoundation/Architecture-for-Concurrent-Vote-Processing-41704666bc414a03869b70ba1043605f?pvs=4
type DKGIndexMap map[Identifier]int
