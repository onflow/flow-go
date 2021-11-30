package rest

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	mocks "github.com/stretchr/testify/mock"

	"github.com/onflow/flow-go/access/mock"
	"github.com/onflow/flow-go/utils/unittest"
)

func blockURL(ids []string, start uint64, end uint64, payload bool) string {
	u, _ := url.Parse("/v1/blocks")
	q := u.Query()

	if len(ids) > 0 {
		u, _ = url.Parse(u.String() + "/" + strings.Join(ids, ","))
	}

	if payload {
		q.Add("expandable", "payload")
	}

	u.RawQuery = q.Encode()
	return u.String()
}

func TestGetBlocks(t *testing.T) {
	t.Skip()
	backend := &mock.API{}


	t.Run("get block by ID", func(t *testing.T) {
		block := unittest.BlockFixture()
		fmt.Println(blockURL([]string{block.ID().String()}, 0, 0, true))
		req, _ := http.NewRequest("GET", blockURL([]string{block.ID().String()}, 0, 0, true), nil)

		backend.Mock.
			On("GetBlockByID", mocks.Anything, block.ID()).
			Return(&block, nil)

		rr := executeRequest(req, backend)

		expected := `{}`
		fmt.Println(rr.Body.String())
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, expected, rr.Body.String())

	})
}
