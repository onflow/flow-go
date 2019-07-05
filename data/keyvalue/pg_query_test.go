package keyvalue

import (
	"testing"
)

func TestGet(t *testing.T) {
	table := "tableA"
	key := "key"

	q := pgSQLQuery{}
	q.Get(table, key)
	q.MustBuild()
	query, params := q.debug()

	expectString(query, "SELECT value FROM ?0 WHERE key=?1 ; ", t)
	expectStrings(params, []string{table, key}, t)
}

func checkError(err error, t *testing.T) {
	if err != nil {
		t.Error(err)
	}
}

func expectBool(actual, expected bool, t *testing.T) {
	if actual != expected {
		t.Errorf("expected %v to be %v", actual, expected)
	}
}

func expectInt(actual, expected int, t *testing.T) {
	if actual != expected {
		t.Errorf("expected %v to be %v", actual, expected)
	}
}

func expectString(actual, expected string, t *testing.T) {
	if actual != expected {
		t.Errorf("expected %v to be %v", actual, expected)
	}
}

func expectStrings(actual, expected []string, t *testing.T) {
	expectInt(len(actual), len(expected), t)
	for i := range actual {
		expectString(actual[i], expected[i], t)
	}
}
