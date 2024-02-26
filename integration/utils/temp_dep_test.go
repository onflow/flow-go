package utils

import "github.com/btcsuite/btcd/chaincfg/chainhash"

// this is added to resolve the issue with chainhash ambiguous import,
// the code is not used, but it's needed to force go.mod to specify and retain chainhash version
// workaround for issue: https://github.com/golang/go/issues/27899
var _ = chainhash.Hash{}
