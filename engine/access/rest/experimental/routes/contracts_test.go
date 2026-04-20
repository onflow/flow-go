package routes_test

import (
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	mocktestify "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/onflow/flow/protobuf/go/flow/entities"

	"github.com/onflow/flow-go/access/backends/extended"
	extendedmock "github.com/onflow/flow-go/access/backends/extended/mock"
	"github.com/onflow/flow-go/engine/access/rest/experimental/request"
	"github.com/onflow/flow-go/engine/access/rest/router"
	accessmodel "github.com/onflow/flow-go/model/access"
	"github.com/onflow/flow-go/model/flow"
	"github.com/onflow/flow-go/utils/unittest"
)

// testEncodeContractsCursor encodes a cursor using the same logic as the handler, for test inputs
// and assertions.
func testEncodeContractsCursor(t *testing.T, cursor *accessmodel.ContractDeploymentsCursor) string {
	data, err := request.EncodeContractDeploymentsCursor(cursor)
	require.NoError(t, err)
	return data
}

type contractsListURLParams struct {
	limit        string
	cursor       string
	contractName string
	startBlock   string
	endBlock     string
	expand       string
}

func contractsListURL(t *testing.T, params contractsListURLParams) string {
	u, err := url.ParseRequestURI("/experimental/v1/contracts")
	require.NoError(t, err)
	q := u.Query()
	if params.limit != "" {
		q.Add("limit", params.limit)
	}
	if params.cursor != "" {
		q.Add("cursor", params.cursor)
	}
	if params.contractName != "" {
		q.Add("contract_name", params.contractName)
	}
	if params.startBlock != "" {
		q.Add("start_block", params.startBlock)
	}
	if params.endBlock != "" {
		q.Add("end_block", params.endBlock)
	}
	if params.expand != "" {
		q.Add("expand", params.expand)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func contractURL(t *testing.T, identifier string, params contractsListURLParams) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/experimental/v1/contracts/%s", identifier))
	require.NoError(t, err)
	q := u.Query()
	if params.expand != "" {
		q.Add("expand", params.expand)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func contractDeploymentsURL(t *testing.T, identifier string, params contractsListURLParams) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/experimental/v1/contracts/%s/deployments", identifier))
	require.NoError(t, err)
	q := u.Query()
	if params.limit != "" {
		q.Add("limit", params.limit)
	}
	if params.cursor != "" {
		q.Add("cursor", params.cursor)
	}
	if params.contractName != "" {
		q.Add("contract_name", params.contractName)
	}
	if params.startBlock != "" {
		q.Add("start_block", params.startBlock)
	}
	if params.endBlock != "" {
		q.Add("end_block", params.endBlock)
	}
	if params.expand != "" {
		q.Add("expand", params.expand)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func contractsByAddressURL(t *testing.T, address string, params contractsListURLParams) string {
	u, err := url.ParseRequestURI(fmt.Sprintf("/experimental/v1/accounts/%s/contracts", address))
	require.NoError(t, err)
	q := u.Query()
	if params.limit != "" {
		q.Add("limit", params.limit)
	}
	if params.cursor != "" {
		q.Add("cursor", params.cursor)
	}
	if params.contractName != "" {
		q.Add("contract_name", params.contractName)
	}
	if params.startBlock != "" {
		q.Add("start_block", params.startBlock)
	}
	if params.endBlock != "" {
		q.Add("end_block", params.endBlock)
	}
	if params.expand != "" {
		q.Add("expand", params.expand)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// buildExpectedDeploymentJSON produces the expected JSON object for a ContractDeployment with
// no inline expansions: code, transaction, and result appear as expandable links.
func buildExpectedDeploymentJSON(d *accessmodel.ContractDeployment) string {
	codeHashStr := hex.EncodeToString(d.CodeHash)
	contractID := accessmodel.ContractID(d.Address, d.ContractName)
	txID := d.TransactionID.String()

	return fmt.Sprintf(`{
		"contract_id": %q,
		"address": %q,
		"block_height": "%d",
		"transaction_id": %q,
		"tx_index": "%d",
		"event_index": "%d",
		"code_hash": %q,
		"_expandable": {
			"code": "/experimental/v1/contracts/%s?expand=code",
			"transaction": "/v1/transactions/%s",
			"result": "/v1/transaction_results/%s"
		}
	}`,
		contractID,
		d.Address.Hex(),
		d.BlockHeight,
		txID,
		d.TransactionIndex,
		d.EventIndex,
		codeHashStr,
		contractID,
		txID,
		txID,
	)
}

// buildExpectedDeploymentJSONWithCode produces the expected JSON object for a ContractDeployment
// with code expanded inline (expand=code), and transaction/result as expandable links.
func buildExpectedDeploymentJSONWithCode(d *accessmodel.ContractDeployment) string {
	var codeStr string
	if len(d.Code) > 0 {
		codeStr = base64.StdEncoding.EncodeToString(d.Code)
	}
	codeHashStr := hex.EncodeToString(d.CodeHash)
	txID := d.TransactionID.String()

	return fmt.Sprintf(`{
		"contract_id": %q,
		"address": %q,
		"block_height": "%d",
		"transaction_id": %q,
		"tx_index": "%d",
		"event_index": "%d",
		"code": %q,
		"code_hash": %q,
		"_expandable": {
			"transaction": "/v1/transactions/%s",
			"result": "/v1/transaction_results/%s"
		}
	}`,
		accessmodel.ContractID(d.Address, d.ContractName),
		d.Address.Hex(),
		d.BlockHeight,
		txID,
		d.TransactionIndex,
		d.EventIndex,
		codeStr,
		codeHashStr,
		txID,
		txID,
	)
}

func TestGetContracts(t *testing.T) {
	addr1 := flow.HexToAddress("1234567890abcdef")
	addr2 := flow.HexToAddress("fedcba0987654321")
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()
	code1 := []byte("pub contract First {}")
	code2 := []byte("pub contract Second {}")

	d1 := accessmodel.ContractDeployment{
		Address:          addr1,
		ContractName:     "First",
		BlockHeight:      1000,
		TransactionID:    txID1,
		TransactionIndex: 2,
		EventIndex:       1,
		Code:             code1,
		CodeHash:         accessmodel.CadenceCodeHash(code1),
	}
	d2 := accessmodel.ContractDeployment{
		Address:          addr2,
		ContractName:     "Second",
		BlockHeight:      999,
		TransactionID:    txID2,
		TransactionIndex: 0,
		EventIndex:       0,
		Code:             code2,
		CodeHash:         accessmodel.CadenceCodeHash(code2),
	}

	t.Run("happy path with next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		nextCursorVal := &accessmodel.ContractDeploymentsCursor{
			Address:          d2.Address,
			ContractName:     d2.ContractName,
			BlockHeight:      d2.BlockHeight,
			TransactionIndex: d2.TransactionIndex,
			EventIndex:       d2.EventIndex,
		}
		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{d1, d2},
			NextCursor:  nextCursorVal,
		}

		backend.On("GetContracts",
			mocktestify.Anything,
			uint32(0),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractsListURL(t, contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)

		expectedCursor := testEncodeContractsCursor(t, nextCursorVal)
		expected := fmt.Sprintf(`{
			"contracts": [%s, %s],
			"next_cursor": %q
		}`,
			buildExpectedDeploymentJSON(&d1),
			buildExpectedDeploymentJSON(&d2),
			expectedCursor,
		)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("last page without next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{d1},
			NextCursor:  nil,
		}

		backend.On("GetContracts",
			mocktestify.Anything,
			uint32(10),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractsListURL(t, contractsListURLParams{limit: "10"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotContains(t, rr.Body.String(), "next_cursor")
	})

	t.Run("with cursor parameter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		inputCursor := &accessmodel.ContractDeploymentsCursor{
			Address:          d1.Address,
			ContractName:     d1.ContractName,
			BlockHeight:      d1.BlockHeight,
			TransactionIndex: d1.TransactionIndex,
			EventIndex:       d1.EventIndex,
		}
		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{d2},
		}

		backend.On("GetContracts",
			mocktestify.Anything,
			uint32(0),
			inputCursor,
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractsListURL(t, contractsListURLParams{
			cursor: testEncodeContractsCursor(t, inputCursor),
		}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("with contract_name filter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{},
		}

		backend.On("GetContracts",
			mocktestify.Anything,
			uint32(0),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{ContractName: "First"},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractsListURL(t, contractsListURLParams{contractName: "First"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("with start_block filter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{},
		}

		startBlock := uint64(500)
		backend.On("GetContracts",
			mocktestify.Anything,
			uint32(0),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{StartBlock: &startBlock},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractsListURL(t, contractsListURLParams{startBlock: "500"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid limit", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, contractsListURL(t, contractsListURLParams{limit: "abc"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid limit")
	})

	t.Run("invalid cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, contractsListURL(t, contractsListURLParams{cursor: "!notbase64!"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid cursor encoding")
	})

	t.Run("invalid start_block", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, contractsListURL(t, contractsListURLParams{startBlock: "notanumber"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid start_block")
	})

	t.Run("backend returns failed precondition", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetContracts",
			mocktestify.Anything,
			uint32(0),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.FailedPrecondition, "index not initialized"))

		req, err := http.NewRequest(http.MethodGet, contractsListURL(t, contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Precondition failed")
	})
}

func TestGetContract(t *testing.T) {
	addr := flow.HexToAddress("1234567890abcdef")
	txID := unittest.IdentifierFixture()
	code := []byte("pub contract MyContract {}")
	codeHash := accessmodel.CadenceCodeHash(code)

	contractID := "A.1234567890abcdef.MyContract"

	t.Run("happy path - code in expandable by default", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		d := &accessmodel.ContractDeployment{
			Address:          addr,
			ContractName:     "MyContract",
			BlockHeight:      1234,
			TransactionID:    txID,
			TransactionIndex: 5,
			EventIndex:       3,
			Code:             code,
			CodeHash:         codeHash,
		}

		backend.On("GetContract",
			mocktestify.Anything,
			contractID,
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(d, nil)

		req, err := http.NewRequest(http.MethodGet, contractURL(t, contractID, contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, buildExpectedDeploymentJSON(d), rr.Body.String())
	})

	t.Run("expand=code includes code inline", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		d := &accessmodel.ContractDeployment{
			Address:          addr,
			ContractName:     "MyContract",
			BlockHeight:      1234,
			TransactionID:    txID,
			TransactionIndex: 5,
			EventIndex:       3,
			Code:             code,
			CodeHash:         codeHash,
		}

		backend.On("GetContract",
			mocktestify.Anything,
			contractID,
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{Code: true},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(d, nil)

		req, err := http.NewRequest(http.MethodGet, contractURL(t, contractID, contractsListURLParams{expand: "code"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, buildExpectedDeploymentJSONWithCode(d), rr.Body.String())
	})

	t.Run("happy path - code not loaded by backend", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		d := &accessmodel.ContractDeployment{
			Address:          addr,
			ContractName:     "MyContract",
			BlockHeight:      1234,
			TransactionID:    txID,
			TransactionIndex: 5,
			EventIndex:       3,
			Code:             nil,
			CodeHash:         codeHash,
		}

		backend.On("GetContract",
			mocktestify.Anything,
			contractID,
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(d, nil)

		req, err := http.NewRequest(http.MethodGet, contractURL(t, contractID, contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.JSONEq(t, buildExpectedDeploymentJSON(d), rr.Body.String())
	})

	t.Run("placeholder deployment", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		d := &accessmodel.ContractDeployment{
			Address:       addr,
			ContractName:  "MyContract",
			Code:          code,
			CodeHash:      codeHash,
			IsPlaceholder: true,
			// BlockHeight, TransactionID, TransactionIndex, EventIndex are zero values
		}

		backend.On("GetContract",
			mocktestify.Anything,
			contractID,
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(d, nil)

		req, err := http.NewRequest(http.MethodGet, contractURL(t, contractID, contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		// Placeholder deployments have is_placeholder=true, no transaction/result links, and code is expandable.
		codeHashStr := hex.EncodeToString(codeHash)
		expected := fmt.Sprintf(`{
			"contract_id": %q,
			"address": %q,
			"block_height": "0",
			"transaction_id": "0000000000000000000000000000000000000000000000000000000000000000",
			"tx_index": "0",
			"event_index": "0",
			"code_hash": %q,
			"is_placeholder": true,
			"_expandable": {
				"code": "/experimental/v1/contracts/%s?expand=code"
			}
		}`, contractID, addr.Hex(), codeHashStr, contractID)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("backend returns not found", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetContract",
			mocktestify.Anything,
			contractID,
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.NotFound, "contract %s not found", contractID))

		req, err := http.NewRequest(http.MethodGet, contractURL(t, contractID, contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("backend returns failed precondition", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetContract",
			mocktestify.Anything,
			contractID,
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.FailedPrecondition, "index not initialized"))

		req, err := http.NewRequest(http.MethodGet, contractURL(t, contractID, contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Precondition failed")
	})
}

func TestGetContractDeployments(t *testing.T) {
	addr := flow.HexToAddress("1234567890abcdef")
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()
	code := []byte("pub contract MyContract {}")

	contractID := "A.1234567890abcdef.MyContract"

	d1 := accessmodel.ContractDeployment{
		Address:          addr,
		ContractName:     "MyContract",
		BlockHeight:      2000,
		TransactionID:    txID1,
		TransactionIndex: 1,
		EventIndex:       0,
		Code:             code,
		CodeHash:         accessmodel.CadenceCodeHash(code),
	}
	d2 := accessmodel.ContractDeployment{
		Address:          addr,
		ContractName:     "MyContract",
		BlockHeight:      1000,
		TransactionID:    txID2,
		TransactionIndex: 3,
		EventIndex:       2,
		Code:             code,
		CodeHash:         accessmodel.CadenceCodeHash(code),
	}

	t.Run("happy path with next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		// For DeploymentsByContractID, the contract is identified by the URL; Address and
		// ContractName are populated from the storage key but not used for resumption.
		nextCursorVal := &accessmodel.ContractDeploymentsCursor{
			Address:          d2.Address,
			ContractName:     d2.ContractName,
			BlockHeight:      d2.BlockHeight,
			TransactionIndex: d2.TransactionIndex,
			EventIndex:       d2.EventIndex,
		}
		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{d1, d2},
			NextCursor:  nextCursorVal,
		}

		backend.On("GetContractDeployments",
			mocktestify.Anything,
			contractID,
			uint32(0),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractDeploymentsURL(t, contractID, contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)

		expectedCursor := testEncodeContractsCursor(t, nextCursorVal)
		expected := fmt.Sprintf(`{
			"deployments": [%s, %s],
			"next_cursor": %q
		}`,
			buildExpectedDeploymentJSON(&d1),
			buildExpectedDeploymentJSON(&d2),
			expectedCursor,
		)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("last page without next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{d1},
			NextCursor:  nil,
		}

		backend.On("GetContractDeployments",
			mocktestify.Anything,
			contractID,
			uint32(5),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractDeploymentsURL(t, contractID, contractsListURLParams{limit: "5"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotContains(t, rr.Body.String(), "next_cursor")
	})

	t.Run("with cursor parameter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		// DeploymentsByContractID cursors include Address and ContractName from the storage key.
		inputCursor := &accessmodel.ContractDeploymentsCursor{
			Address:          d1.Address,
			ContractName:     d1.ContractName,
			BlockHeight:      d1.BlockHeight,
			TransactionIndex: d1.TransactionIndex,
			EventIndex:       d1.EventIndex,
		}
		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{d2},
		}

		backend.On("GetContractDeployments",
			mocktestify.Anything,
			contractID,
			uint32(0),
			inputCursor,
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractDeploymentsURL(t, contractID, contractsListURLParams{
			cursor: testEncodeContractsCursor(t, inputCursor),
		}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid limit", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, contractDeploymentsURL(t, contractID, contractsListURLParams{limit: "abc"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid limit")
	})

	t.Run("invalid cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, contractDeploymentsURL(t, contractID, contractsListURLParams{cursor: "badcursor"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid cursor encoding")
	})

	t.Run("backend returns not found", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetContractDeployments",
			mocktestify.Anything,
			contractID,
			uint32(0),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.NotFound, "contract %s not found", contractID))

		req, err := http.NewRequest(http.MethodGet, contractDeploymentsURL(t, contractID, contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusNotFound, rr.Code)
	})

	t.Run("backend returns failed precondition", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetContractDeployments",
			mocktestify.Anything,
			contractID,
			uint32(0),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.FailedPrecondition, "index not initialized"))

		req, err := http.NewRequest(http.MethodGet, contractDeploymentsURL(t, contractID, contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Precondition failed")
	})
}

func TestGetContractsByAddress(t *testing.T) {
	address := unittest.AddressFixture()
	txID1 := unittest.IdentifierFixture()
	txID2 := unittest.IdentifierFixture()
	code := []byte("pub contract MyContract {}")

	d1 := accessmodel.ContractDeployment{
		Address:          address,
		ContractName:     "First",
		BlockHeight:      1500,
		TransactionID:    txID1,
		TransactionIndex: 0,
		EventIndex:       0,
		Code:             code,
		CodeHash:         accessmodel.CadenceCodeHash(code),
	}
	d2 := accessmodel.ContractDeployment{
		Address:          address,
		ContractName:     "Second",
		BlockHeight:      1400,
		TransactionID:    txID2,
		TransactionIndex: 1,
		EventIndex:       0,
		Code:             code,
		CodeHash:         accessmodel.CadenceCodeHash(code),
	}

	t.Run("happy path with next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		nextCursorVal := &accessmodel.ContractDeploymentsCursor{
			Address:          d2.Address,
			ContractName:     d2.ContractName,
			BlockHeight:      d2.BlockHeight,
			TransactionIndex: d2.TransactionIndex,
			EventIndex:       d2.EventIndex,
		}
		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{d1, d2},
			NextCursor:  nextCursorVal,
		}

		backend.On("GetContractsByAddress",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractsByAddressURL(t, address.String(), contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)

		expectedCursor := testEncodeContractsCursor(t, nextCursorVal)
		expected := fmt.Sprintf(`{
			"contracts": [%s, %s],
			"next_cursor": %q
		}`,
			buildExpectedDeploymentJSON(&d1),
			buildExpectedDeploymentJSON(&d2),
			expectedCursor,
		)
		assert.JSONEq(t, expected, rr.Body.String())
	})

	t.Run("last page without next cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{d1},
			NextCursor:  nil,
		}

		backend.On("GetContractsByAddress",
			mocktestify.Anything,
			address,
			uint32(3),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractsByAddressURL(t, address.String(), contractsListURLParams{limit: "3"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.NotContains(t, rr.Body.String(), "next_cursor")
	})

	t.Run("with cursor parameter", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		// ByAddress cursors include Address and ContractName as the resume key.
		inputCursor := &accessmodel.ContractDeploymentsCursor{
			Address:          d1.Address,
			ContractName:     d1.ContractName,
			BlockHeight:      d1.BlockHeight,
			TransactionIndex: d1.TransactionIndex,
			EventIndex:       d1.EventIndex,
		}
		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{d2},
		}

		backend.On("GetContractsByAddress",
			mocktestify.Anything,
			address,
			uint32(0),
			inputCursor,
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractsByAddressURL(t, address.String(), contractsListURLParams{
			cursor: testEncodeContractsCursor(t, inputCursor),
		}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("address with 0x prefix", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		page := &accessmodel.ContractDeploymentPage{
			Deployments: []accessmodel.ContractDeployment{d1},
		}

		backend.On("GetContractsByAddress",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(page, nil)

		req, err := http.NewRequest(http.MethodGet, contractsByAddressURL(t, "0x"+address.String(), contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusOK, rr.Code)
	})

	t.Run("invalid address", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, contractsByAddressURL(t, "invalid", contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid address")
	})

	t.Run("invalid cursor", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, contractsByAddressURL(t, address.String(), contractsListURLParams{cursor: "!notbase64!"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid cursor encoding")
	})

	t.Run("invalid limit", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		req, err := http.NewRequest(http.MethodGet, contractsByAddressURL(t, address.String(), contractsListURLParams{limit: "abc"}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "invalid limit")
	})

	t.Run("backend returns failed precondition", func(t *testing.T) {
		backend := extendedmock.NewAPI(t)

		backend.On("GetContractsByAddress",
			mocktestify.Anything,
			address,
			uint32(0),
			(*accessmodel.ContractDeploymentsCursor)(nil),
			extended.ContractDeploymentFilter{},
			extended.ContractDeploymentExpandOptions{},
			entities.EventEncodingVersion_JSON_CDC_V0,
		).Return(nil, status.Errorf(codes.FailedPrecondition, "index not initialized"))

		req, err := http.NewRequest(http.MethodGet, contractsByAddressURL(t, address.String(), contractsListURLParams{}), nil)
		require.NoError(t, err)

		rr := router.ExecuteExperimentalRequest(req, backend)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
		assert.Contains(t, rr.Body.String(), "Precondition failed")
	})
}
