package mapper

import "github.com/onflow/flow-go/model/flow"

func QuorumCertificate(qc *flow.QuorumCertificate) []byte {
	return makePrefix(codeBlockIDToQuorumCertificate, qc.BlockID)
}

func QuorumCertificateValu(qc *flow.QuorumCertificate) ([]byte, interface{}) {
	return QuorumCertificate(qc), qc
}
