package keyvalue

import (
	"testing"
)

func TestGet(t *testing.T) {
	table := "tableA"
	key := "keyA"

	q := pgSQLQuery{}
	q.Get(table, key)
	q.MustBuild()
	query, params := q.debug()

	expectString(query, "SELECT value FROM ?0 WHERE key=?1 ; ", t)
	expectStrings(params, []string{table, key}, t)
}

func TestSet(t *testing.T) {
	table := "tableA"
	key := "keyA"
	value := "valueA"

	q := pgSQLQuery{}
	q.Set(table, key, value)
	q.MustBuild()
	query, params := q.debug()

	expectString(query, "INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?3 ; ", t)
	expectStrings(params, []string{table, key, value, value}, t)
}

func TestMultiSetNoTx(t *testing.T) {

	defer expectPanic("Must use a transaction when changing more than one key", t)

	table1 := "tableA"
	key1 := "keyA"
	value1 := "valueA"

	table2 := "tableB"
	key2 := "keyB"
	value2 := "valueB"

	q := pgSQLQuery{}
	q.Set(table1, key1, value1)
	q.Set(table2, key2, value2)
	q.MustBuild()
	query, params := q.debug()

	expectString(query, "INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?3 ; ", t)
	expectStrings(params, []string{table1, key1, value1, value1, table2, key2, value2, value2}, t)
}

func TestMultiSetWithTx(t *testing.T) {

	table1 := "tableA"
	key1 := "keyA"
	value1 := "valueA"

	table2 := "tableB"
	key2 := "keyB"
	value2 := "valueB"

	q := pgSQLQuery{}
	q.Set(table1, key1, value1)
	q.Set(table2, key2, value2)
	q.InTransaction()
	q.MustBuild()
	query, params := q.debug()

	expectString(query, "BEGIN; INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?3 ; INSERT INTO ?4 (key, value) VALUES ('?5', '?6') ON CONFLICT (key) DO UPDATE SET value = ?7 ;  COMMIT;", t)
	expectStrings(params, []string{table1, key1, value1, value1, table2, key2, value2, value2}, t)
}

// -----
// Utils
// -----

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

func expectPanic(expectedMsg string, t *testing.T) {
	if r := recover(); r != nil {
		err := r.(error)
		expectString(err.Error(), expectedMsg, t)
		return
	}
	t.Errorf("Expected to panic with `%s`, but did not panic", expectedMsg)
}
