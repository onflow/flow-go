// (c) 2019 Dapper Labs - ALL RIGHTS RESERVED

package flow

import (
	"log"
	"strconv"
	"os"

	"github.com/aristanetworks/goarista/monotime"

	"github.com/onflow/flow-go/crypto"
)

// CollectionGuarantee is a signed hash for a collection, which is used
// to announce collections to consensus nodes.
type CollectionGuarantee struct {
	CollectionID     Identifier       // ID of the collection being guaranteed
	ReferenceBlockID Identifier       // defines expiry of the collection
	SignerIDs        []Identifier     // list of guarantors
	Signature        crypto.Signature // guarantor signatures
}

// ID returns the fingerprint of the collection guarantee.
func (cg *CollectionGuarantee) ID() Identifier {
	return cg.CollectionID
}

var logfile_coll *os.File


// Checksum returns a checksum of the collection guarantee including the
// signatures.
func (cg *CollectionGuarantee) Checksum() Identifier {

	once.Do(func() {
		newfile, err := os.Create("/tmp/makeid-investigation/model/flow/collectionGuarantee.log")
		if err != nil {
			log.Fatal(err)
		}
		logfile_coll = newfile
	})
	ts := monotime.Now()

	defer logfile_coll.WriteString(strconv.FormatUint(monotime.Now(), 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return MakeID(cg)
}
