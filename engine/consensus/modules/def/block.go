package def

import (
	"bytes"
	"crypto/sha256"

	"github.com/dapperlabs/flow-go/engine/consensus/modules/utils"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/juju/loggo"
)

var DefinitionsLogger loggo.Logger

type Block struct {
	View       uint64
	Height     uint64
	ChainID    string
	BlockMRH   []byte
	Payload    []byte
	PayloadMRH []byte
	Timestamp  *timestamp.Timestamp
	QC         *QuorumCertificate
}

// Calculate and set BlockMRH
// currently we just concatenate hashes of all necessary fields
func (b *Block) setBlockHash() {
	data := bytes.Join(
		[][]byte{
			// hash of View
			utils.ConvertUintToByte(b.View),
			// hash of ChainID
			[]byte(b.ChainID),
			// hash of Height
			utils.ConvertUintToByte(b.Height),
			// hash of Timestamp
			//TODO:
			//utils.ConvertUintToByte(uint64(b.Timestamp.Seconds)),
			// hash of QC (stored in QC already at construction time)
			b.QC.Hash,
			b.PayloadMRH,
		},
		[]byte{},
	)
	blockMRH := sha256.Sum256(data)

	b.BlockMRH = blockMRH[:]
}

//TODO: check if block is properly formed (contain all fields)
func (b *Block) HasValidStructure() bool {
	return true
}

//TODO: check if block is properly formed (contain all fields)
func (b *Block) HasValidQuorumCertificate() bool {
	return b.QC.IsValid() && (b.View > b.QC.View)
}

// new block should be built on top of current highest QC
func NewBlock(blockPayload []byte, highestQC *QuorumCertificate, view uint64) *Block {
	b := &Block{
		View:    view,
		Payload: blockPayload,
		QC:      highestQC,
		//Timestamp: timestamp,
	}

	b.setBlockHash()

	return b
}
