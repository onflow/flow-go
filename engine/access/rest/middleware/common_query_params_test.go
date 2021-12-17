package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
)

// TestCommonQueryParamMiddlewares test the two common query param middleware - expand and select
func TestCommonQueryParamMiddlewares(t *testing.T) {

	testFunc := func(expandList, selectList []string) {

		th := &testHandler{
			t:                  t,
			expectedExpandList: expandList,
			expectedSelectList: selectList,
		}

		// setup the router to use the test handler and the QueryExpandable and QuerySelect
		r := mux.NewRouter()
		r.Handle("/", th.getTestHandler())
		r.Use(QueryExpandable())
		r.Use(QuerySelect())

		// create request
		req := httptest.NewRequest("GET", "/", nil)
		query := req.URL.Query()
		// add query params as per test case
		if len(expandList) > 0 {
			query.Add(ExpandQueryParam, strings.Join(expandList, ","))
		}
		if len(selectList) > 0 {
			query.Add(selectQueryParam, strings.Join(selectList, ","))
		}
		req.URL.RawQuery = query.Encode()
		// fmt.Println(req.URL.String())

		// We create a ResponseRecorder (which satisfies http.ResponseWriter) to record the response.
		rr := httptest.NewRecorder()

		// send the request
		r.ServeHTTP(rr, req)
		if status := rr.Code; status != http.StatusOK {
			t.Errorf("handler returned wrong status code: got %v want %v",
				status, http.StatusOK)
		}
	}

	testcases := []struct {
		expandList []string
		selectList []string
	}{
		{
			expandList: nil,
			selectList: nil,
		},
		{
			expandList: []string{"abcd"},
			selectList: nil,
		},
		{
			expandList: []string{"abcd", "xyz"},
			selectList: nil,
		},
		{
			expandList: nil,
			selectList: []string{"abcd"},
		},
		{
			expandList: nil,
			selectList: []string{"abcd", "xyz"},
		},
		{
			expandList: []string{"abcd"},
			selectList: []string{"abcd"},
		},
		{
			expandList: []string{"abcd", "xyz"},
			selectList: []string{"abcd", "xyz"},
		},
	}
	for _, t := range testcases {
		testFunc(t.expandList, t.selectList)
	}
}

type testHandler struct {
	expectedExpandList []string
	expectedSelectList []string
	t                  *testing.T
}

func (th *testHandler) getTestHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		actualExpandList, expandListPopulated := GetFieldsToExpand(r)
		require.Equal(th.t, len(th.expectedExpandList) != 0, expandListPopulated)
		require.ElementsMatch(th.t, th.expectedExpandList, actualExpandList)

		actualSelectList, selectListPopulated := GetFieldsToSelect(r)
		require.Equal(th.t, len(th.expectedSelectList) != 0, selectListPopulated)
		require.ElementsMatch(th.t, th.expectedSelectList, actualSelectList)

		w.WriteHeader(http.StatusOK)
	})
}
