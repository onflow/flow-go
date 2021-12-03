package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/engine/access/rest/middleware"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

func blockURL(ids []string, start uint64, end uint64, expandPayload bool, expandExecResult bool, heights ...string) string {
	u, _ := url.Parse("/v1/blocks")
	q := u.Query()

	if len(ids) > 0 {
		u, _ = url.Parse(u.String() + "/" + strings.Join(ids, ","))
	}

	if start > 0 {
		q.Add(startHeightQueryParam, fmt.Sprintf("%d", start))
		q.Add(endHeightQueryParam, fmt.Sprintf("%d", end))
	}

	if len(heights) > 0 {
		heightsStr := strings.Join(heights, ",")
		q.Add(heightQueryParam, heightsStr)
	}

	var expands []string
	if expandPayload {
		expands = append(expands, ExpandableFieldPayload)
	}
	if expandExecResult {
		expands = append(expands, ExpandableExecutionResult)
	}
	if len(expands) > 0 {
		expandsStr := strings.Join(expands, ",")
		q.Add(middleware.ExpandQueryParam, expandsStr)
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func TestGetBlocks(t *testing.T) {
	backend := &mock.API{}

	t.Run("get block by ID", func(t *testing.T) {
		block := unittest.BlockFixture()
		executionResult := unittest.ExecutionResultFixture()

		req, err := http.NewRequest("GET", blockURL([]string{block.ID().String()}, 0, 0, true, true), nil)
		require.NoError(t, err)

		backend.Mock.On("GetBlockByID", mocks.Anything, block.ID()).Return(&block, nil)
		backend.Mock.On("GetExecutionResultForBlockID", mocks.Anything, block.ID()).Return(executionResult, nil)

		rr := executeRequest(req, backend)

		expected := `[
	{
		"header": {
			"id": %s,
			"parent_id": "7bf36b6e547aa407cf2dc68977ea5411c58dc6c0ae225b0cd54036e884bcaf57",
			"height": 502607935,
			"timestamp": "2021-12-03T01:53:35.945613Z",
			"parent_voter_signature": "K/EsrL90aTB/te6cQUrPTLzeT5QgBY4yxln7oxZhrIeBQr9jDUCrRUEngWYfzmT8i+2c4PIxfCeZ1V8D3MTCn9oc0LfIho4wFB3KmD4Q1vL3NOn7KJBK5m+Lijzv0abX"
		},
		"payload": {
			"collection_guarantees": [],
			"block_seals": []
		},
		"execution_result": {
			"id": "3658c7cfbd7178bcad8f1ef8036b262f3b3a66a57a9ed2d8fd9c65be92118f2e",
			"block_id": "3b8462c377c110c438500d1697d7635300d745147bd605321186151c846e2288",
			"events": [],
			"_links": {
				"_self": "/v1/execution_results/3658c7cfbd7178bcad8f1ef8036b262f3b3a66a57a9ed2d8fd9c65be92118f2e"
			}
		},
		"_expandable": {},
		"_links": {
			"_self": "/v1/blocks/b129bbae91e40dc435096f99766f29052547a42ec19ff6c343dbc406619e8c68"
		}
	}
]`
		fmt.Println(rr.Body.String())
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, expected, rr.Body.String())

	})
}

func expectedBlockResponseExpanded(block *flow.Block) string {
	id := block.ID().String()
	fmt.Sprintf(`
	{
		"header": {
			"id": %s,
			"parent_id": %s,
			"height": %d,
			"timestamp": "2021-12-03T01:53:35.945613Z",
			"parent_voter_signature": "K/EsrL90aTB/te6cQUrPTLzeT5QgBY4yxln7oxZhrIeBQr9jDUCrRUEngWYfzmT8i+2c4PIxfCeZ1V8D3MTCn9oc0LfIho4wFB3KmD4Q1vL3NOn7KJBK5m+Lijzv0abX"
		},
		"payload": {
			"collection_guarantees": [],
			"block_seals": []
		},
		"execution_result": {
			"id": "3658c7cfbd7178bcad8f1ef8036b262f3b3a66a57a9ed2d8fd9c65be92118f2e",
			"block_id": "3b8462c377c110c438500d1697d7635300d745147bd605321186151c846e2288",
			"events": [],
			"_links": {
				"_self": "/v1/execution_results/3658c7cfbd7178bcad8f1ef8036b262f3b3a66a57a9ed2d8fd9c65be92118f2e"
			}
		},
		"_expandable": {},
		"_links": {
			"_self": "/v1/blocks/b129bbae91e40dc435096f99766f29052547a42ec19ff6c343dbc406619e8c68"
		}
	}`, id,  block.Header.ParentID.String(), block.Header.Height, block.Header.Timestamp.String(), )
}
