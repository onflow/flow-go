package flow

import (
	"os"
	"strconv"

	"github.com/aristanetworks/goarista/monotime"

	"github.com/onflow/flow-go/crypto"
)

// Attestation confirms correctness of a chunk of an exec result
type Attestation struct {
	BlockID           Identifier // ID of the block included the collection
	ExecutionResultID Identifier // ID of the execution result
	ChunkIndex        uint64     // index of the approved chunk
}

var logfile_resultapproval1 *os.File

// ID generates a unique identifier using attestation
func (a Attestation) ID() Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/temp/makeid-investigation/model/flow/resultapproval1.log")
		logfile_resultapproval1 = newfile
	})
	ts := monotime.Now()
	defer logfile_resultapproval1.WriteString(strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return MakeID(a)
}

// ResultApprovalBody holds body part of a result approval
type ResultApprovalBody struct {
	Attestation
	ApproverID           Identifier       // node id generating this result approval
	AttestationSignature crypto.Signature // signature over attestation, this has been separated for BLS aggregation
	Spock                crypto.Signature // proof of re-computation, one per each chunk
}
var logfile_resultapproval2 *os.File


// PartialID generates a unique identifier using Attestation + ApproverID
func (rab ResultApprovalBody) PartialID() Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/temp/makeid-investigation/model/flow/resultapproval2.log")
		logfile_resultapproval2 = newfile
	})
	ts := monotime.Now()
	defer logfile_resultapproval2.WriteString(strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	data := struct {
		Attestation Attestation
		ApproverID  Identifier
	}{
		Attestation: rab.Attestation,
		ApproverID:  rab.ApproverID,
	}

	return MakeID(data)
}
var logfile_resultapproval3 *os.File


// ID generates a unique identifier using ResultApprovalBody
func (rab ResultApprovalBody) ID() Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/temp/makeid-investigation/model/flow/resultapproval3.log")
		logfile_resultapproval3 = newfile
	})
	ts := monotime.Now()
	defer logfile_resultapproval3.WriteString(strconv.FormatUint(monotime.Now() - ts, 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return MakeID(rab)
}

// ResultApproval includes an approval for a chunk, verified by a verification node
type ResultApproval struct {
	Body              ResultApprovalBody
	VerifierSignature crypto.Signature // signature over all above fields
}

var logfile_resultapproval4 *os.File

// ID generates a unique identifier using result approval body
func (ra ResultApproval) ID() Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/temp/makeid-investigation/model/flow/resultapproval4.log")
		logfile_resultapproval4 = newfile
	})
	ts := monotime.Now()
	defer logfile_resultapproval4.WriteString(strconv.FormatUint(monotime.Now() - ts, 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return MakeID(ra.Body)
}

var logfile_resultapproval5 *os.File

// Checksum generates checksum using the result approval full content
func (ra ResultApproval) Checksum() Identifier {
	once.Do(func() {
		newfile, _ := os.Create("/temp/makeid-investigation/model/flow/resultapproval5.log")
		logfile_resultapproval5 = newfile
	})
	ts := monotime.Now()
	defer logfile_resultapproval5.WriteString(strconv.FormatUint(monotime.Now() - ts, 10) + "," +
		strconv.FormatUint(monotime.Now() - ts, 10) + "\n")

	return MakeID(ra)
}
