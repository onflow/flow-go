package flow_test

import (
	"bufio"
	"log"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// TestTransaction_PerfModifications checks that the sibling source file doesn't
// have errors introduced because of some of the cut/pasted structures.
// It will parse the file, assuming tabs for beginning whitespace, and
// assuming no '/*' style comments that would break the logic.
func TestTransaction_PerfModifications(t *testing.T) {
	
	// open the sibling source file in the same directory
	f, err := os.Open("transaction.go")
	if err != nil {
		log.Fatal(err)
	}
	// remember to close the file at the end of the program
	defer f.Close()

	// read the file line by line using scanner
	scanner := bufio.NewScanner(f)

	nestedBlock := 0

	mapSegments := make(map[string]string)

	// create 2 arrays of arbitrary length '10'.  Only 1st element is used
	
	// keep the segment names, from inside the braces
	segmentNames := make([]string, 10)

	// store the trimmed, de-commented, code segments for comparison
	segments := make([]string, 10)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), ("///%[[")) {
			nestedBlock++

			// clear this string in the array cache
			segments[nestedBlock] = ""

			idxName := strings.Index(scanner.Text(), "{")
			if idxName > -1 {
				idxEnd := strings.Index(scanner.Text(), "}")
				if idxEnd > -1 {
					runes := []rune(scanner.Text())
					segmentNames[nestedBlock] = string(runes[(idxName + 1):idxEnd])
					print("found ")
					println(segmentNames[nestedBlock])
					print("block nest level is ")
					println(nestedBlock)
				}
			}
			continue
		}
		if strings.Contains(scanner.Text(), ("///%]]")) {
			// see if the named segment exists.  Add it if not
			v, found := mapSegments[segmentNames[nestedBlock]]

			if found == false {
				print("adding new segment: ")
				println(segmentNames[nestedBlock])
				mapSegments[segmentNames[nestedBlock]] = segments[nestedBlock]
			} else {
				// compare the segments
				assert.Equal(t, strings.Contains(v, segments[nestedBlock]), true)
			}
			nestedBlock--
			continue
		}
		if nestedBlock > 0 {
			// we're in a segment.  store the trimmed line for later comparison. ignore comments
			if strings.HasPrefix(strings.TrimLeft(scanner.Text(), "\t"), "//") {
				continue
			}

			segments[nestedBlock] = segments[nestedBlock] + strings.TrimLeft(scanner.Text(), "\t")
			print("appending to segment: ")
			println(scanner.Text())
		}
	}
}

func TestTransaction_SignatureOrdering(t *testing.T) {
	tx := flow.NewTransactionBody()

	proposerAddress := unittest.RandomAddressFixture()
	proposerKeyIndex := uint64(1)
	proposerSequenceNumber := uint64(42)
	proposerSignature := []byte{1, 2, 3}

	authorizerAddress := unittest.RandomAddressFixture()
	authorizerKeyIndex := uint64(0)
	authorizerSignature := []byte{4, 5, 6}

	payerAddress := unittest.RandomAddressFixture()
	payerKeyIndex := uint64(0)
	payerSignature := []byte{7, 8, 9}

	tx.SetProposalKey(proposerAddress, proposerKeyIndex, proposerSequenceNumber)
	tx.AddPayloadSignature(proposerAddress, proposerKeyIndex, proposerSignature)

	tx.SetPayer(payerAddress)
	tx.AddEnvelopeSignature(payerAddress, payerKeyIndex, payerSignature)

	tx.AddAuthorizer(authorizerAddress)
	tx.AddPayloadSignature(authorizerAddress, authorizerKeyIndex, authorizerSignature)

	require.Len(t, tx.PayloadSignatures, 2)

	signatureA := tx.PayloadSignatures[0]
	signatureB := tx.PayloadSignatures[1]

	assert.Equal(t, proposerAddress, signatureA.Address)
	assert.Equal(t, authorizerAddress, signatureB.Address)
}

func TestTransaction_Status(t *testing.T) {
	statuses := map[flow.TransactionStatus]string{
		flow.TransactionStatusUnknown:   "UNKNOWN",
		flow.TransactionStatusPending:   "PENDING",
		flow.TransactionStatusFinalized: "FINALIZED",
		flow.TransactionStatusExecuted:  "EXECUTED",
		flow.TransactionStatusSealed:    "SEALED",
		flow.TransactionStatusExpired:   "EXPIRED",
	}

	for status, value := range statuses {
		assert.Equal(t, status.String(), value)
	}
}
