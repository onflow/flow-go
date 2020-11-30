package ping

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type Suite struct {
	suite.Suite
}

func TestHandler(t *testing.T) {
	suite.Run(t, new(Suite))
}

func TestUnmarshalNodeInfoFromJSONData(t *testing.T) {
	totalNodes := 10
	ids := unittest.IdentifierListFixture(totalNodes)
	testJson := make(map[flow.Identifier]string, totalNodes)
	for i, id := range ids {
		testJson[id] = fmt.Sprintf("Operator%d", i+1)
	}
	jsonAsBytes, err := json.Marshal(testJson)
	require.NoError(t, err)
	content, err := unmarshalNodeInfoFromJSONData(jsonAsBytes)
	require.NoError(t, err)
	require.Len(t, content, totalNodes)
	for k, v := range testJson {
		require.Contains(t, content, k)
		require.Equal(t, content[k], v)
	}
}
