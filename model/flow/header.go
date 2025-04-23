package flow

// ProposalHeader is a block header and the proposer's signature for the block.
type ProposalHeader struct {
	Header *Header
	// ProposerSigData is a signature of the proposer over the new block. Not a single cryptographic
	// signature since the data represents cryptographic signatures serialized in some way (concatenation or other)
	ProposerSigData []byte
}

// Header contains all meta-data for a block, as well as a hash representing
// the combined payload of the entire block. It is what consensus nodes agree
// on after validating the contents against the payload hash.
type Header struct {
	// ChainID is a chain-specific value to prevent replay attacks.
	ChainID ChainID
	// ParentID is the ID of this block's parent.
	ParentID Identifier
	// Height is the height of the parent + 1
	Height uint64
	// PayloadHash is a hash of the payload of this block.
	PayloadHash Identifier
	// Timestamp is the time at which this block was proposed, in Unix milliseconds.
	// The proposer can choose any time, so this should not be trusted as accurate.
	Timestamp uint64
	// View number at which this block was proposed.
	View uint64
	// ParentView number at which parent block was proposed.
	ParentView uint64
	// ParentVoterIndices is a bitvector that represents all the voters for the parent block.
	ParentVoterIndices []byte
	// ParentVoterSigData is an aggregated signature over the parent block. Not a single cryptographic
	// signature since the data represents cryptographic signatures serialized in some way (concatenation or other)
	// A quorum certificate can be extracted from the header.
	// This field is the SigData field of the extracted quorum certificate.
	ParentVoterSigData []byte
	// ProposerID is a proposer identifier for the block
	ProposerID Identifier
	// LastViewTC is a timeout certificate for previous view, it can be nil
	// it has to be present if previous round ended with timeout.
	LastViewTC *TimeoutCertificate
}

// QuorumCertificate returns quorum certificate that is incorporated in the block header.
func (h Header) QuorumCertificate() *QuorumCertificate {
	return &QuorumCertificate{
		BlockID:       h.ParentID,
		View:          h.ParentView,
		SignerIndices: h.ParentVoterIndices,
		SigData:       h.ParentVoterSigData,
	}
}

// ID returns a unique ID to singularly identify the header and its block
// within the flow system.
func (h Header) ID() Identifier {
	return MakeID(h)
}

// Checksum returns the checksum of the header.
// TODO(malleability): remove this function
func (h Header) Checksum() Identifier {
	return MakeID(h)
}
