package flow_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/onflow/flow-go/model/flow"
)

func TestDomainTags(t *testing.T) {
	assert.Len(t, flow.TransactionDomainTag, flow.DomainTagLength)
}
