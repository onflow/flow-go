package fixtures

import (
	"github.com/onflow/crypto"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/model/flow/filter"
)

// EpochCommit is the default options factory for [flow.EpochCommit] generation.
var EpochCommit epochCommitFactory

type epochCommitFactory struct{}

type EpochCommitOption func(*EpochCommitGenerator, *flow.EpochCommit)

// WithCounter is an option that sets the `Counter` of the epoch commit.
func (f epochCommitFactory) WithCounter(counter uint64) EpochCommitOption {
	return func(g *EpochCommitGenerator, commit *flow.EpochCommit) {
		commit.Counter = counter
	}
}

// WithClusterQCs is an option that sets the `ClusterQCs` of the epoch commit.
func (f epochCommitFactory) WithClusterQCs(qcs ...flow.ClusterQCVoteData) EpochCommitOption {
	return func(g *EpochCommitGenerator, commit *flow.EpochCommit) {
		commit.ClusterQCs = qcs
	}
}

// WithClusterQCsFromAssignments is an option that sets the `ClusterQCs` of the epoch commit from
// the assignments.
func (f epochCommitFactory) WithClusterQCsFromAssignments(assignments flow.AssignmentList) EpochCommitOption {
	return func(g *EpochCommitGenerator, commit *flow.EpochCommit) {
		qcs := make([]*flow.QuorumCertificateWithSignerIDs, 0, len(assignments))
		for _, nodes := range assignments {
			qc := g.quorumCerts.Fixture(QuorumCertificateWithSignerIDs.WithSignerIDs(nodes))
			qcs = append(qcs, qc)
		}
		commit.ClusterQCs = flow.ClusterQCVoteDatasFromQCs(qcs)
	}
}

// WithDKGFromParticipants is an option that sets the `DKGGroupKey` and `DKGParticipantKeys` of the
// epoch commit from the participants.
func (f epochCommitFactory) WithDKGFromParticipants(participants flow.IdentitySkeletonList) EpochCommitOption {
	return func(g *EpochCommitGenerator, commit *flow.EpochCommit) {
		dkgParticipants := participants.Filter(filter.IsConsensusCommitteeMember).Sort(flow.Canonical[flow.IdentitySkeleton])
		commit.DKGParticipantKeys = nil
		commit.DKGIndexMap = make(flow.DKGIndexMap)
		commit.DKGParticipantKeys = g.cryptoGen.PublicKeys(len(dkgParticipants), crypto.BLSBLS12381)
		for index, nodeID := range dkgParticipants.NodeIDs() {
			commit.DKGIndexMap[nodeID] = index
		}
	}
}

// EpochCommitGenerator generates epoch commit events with consistent randomness.
type EpochCommitGenerator struct {
	epochCommitFactory

	random      *RandomGenerator
	cryptoGen   *CryptoGenerator
	identifiers *IdentifierGenerator
	quorumCerts *QuorumCertificateWithSignerIDsGenerator
}

// NewEpochCommitGenerator creates a new EpochCommitGenerator.
func NewEpochCommitGenerator(
	random *RandomGenerator,
	cryptoGen *CryptoGenerator,
	identifiers *IdentifierGenerator,
	quorumCerts *QuorumCertificateWithSignerIDsGenerator,
) *EpochCommitGenerator {
	return &EpochCommitGenerator{
		random:      random,
		cryptoGen:   cryptoGen,
		identifiers: identifiers,
		quorumCerts: quorumCerts,
	}
}

// Fixture generates a [flow.EpochCommit] with random data based on the provided options.
func (g *EpochCommitGenerator) Fixture(opts ...EpochCommitOption) *flow.EpochCommit {
	commit := &flow.EpochCommit{
		Counter:            uint64(g.random.Uint32()),
		ClusterQCs:         flow.ClusterQCVoteDatasFromQCs(g.quorumCerts.List(1)),
		DKGGroupKey:        g.cryptoGen.PrivateKey(crypto.BLSBLS12381).PublicKey(),
		DKGParticipantKeys: g.cryptoGen.PublicKeys(2, crypto.BLSBLS12381),
	}

	for _, opt := range opts {
		opt(g, commit)
	}

	return commit
}

// List generates a list of [flow.EpochCommit].
func (g *EpochCommitGenerator) List(n int, opts ...EpochCommitOption) []*flow.EpochCommit {
	commits := make([]*flow.EpochCommit, n)
	for i := range n {
		commits[i] = g.Fixture(opts...)
	}
	return commits
}
