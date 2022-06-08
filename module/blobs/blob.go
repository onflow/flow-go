package blobs

import blocks "github.com/ipfs/go-block-format"

type Blob = blocks.Block

var NewBlob = blocks.NewBlock
