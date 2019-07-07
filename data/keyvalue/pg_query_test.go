package keyvalue

import (
	"testing"

	"github.com/dapperlabs/bamboo-node/utils/unittest"
)

func TestGet(t *testing.T) {
	table := "tableA"
	key := "keyA"

	q := pgSQLQuery{}
	q.Get(table, key)
	q.MustBuild()
	query, params := q.debug()

	unittest.ExpectString(query, "SELECT value FROM ?0 WHERE key=?1 ; ", t)
	unittest.ExpectStrings(params, []string{table, key}, t)
}

func TestSet(t *testing.T) {
	table := "tableA"
	key := "keyA"
	value := "valueA"

	q := pgSQLQuery{}
	q.Set(table, key)
	q.MustBuild()
	q.mergeSetParams([]string{value})
	query, params := q.debug()

	unittest.ExpectString(query, "INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?2 ; ", t)
	unittest.ExpectStrings(params, []string{table, key, value}, t)
}

func TestMultiSetNoTx(t *testing.T) {

	defer unittest.ExpectPanic("Must use a transaction when changing more than one key", t)

	table1 := "tableA"
	key1 := "keyA"
	value1 := "valueA"

	table2 := "tableB"
	key2 := "keyB"
	value2 := "valueB"

	q := pgSQLQuery{}
	q.Set(table1, key1)
	q.Set(table2, key2)
	q.MustBuild()
	q.mergeSetParams([]string{value1, value2})

}

func TestMultiSetWithTx(t *testing.T) {

	table1 := "tableA"
	key1 := "keyA"
	value1 := "valueA"

	table2 := "tableB"
	key2 := "keyB"
	value2 := "valueB"

	q := pgSQLQuery{}
	q.Set(table1, key1)
	q.Set(table2, key2)
	q.InTransaction()
	q.MustBuild()
	q.mergeSetParams([]string{value1, value2})
	query, params := q.debug()

	unittest.ExpectString(query, "BEGIN; INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?2 ; INSERT INTO ?3 (key, value) VALUES ('?4', '?5') ON CONFLICT (key) DO UPDATE SET value = ?5 ;  COMMIT;", t)
	unittest.ExpectStrings(params, []string{table1, key1, value1, table2, key2, value2}, t)
}

func TestMustBuildBeforeExecute(t *testing.T) {

	table := "tableA"
	key := "keyA"

	q := pgSQLQuery{}
	q.Get(table, key)
	_, err := q.Execute()
	unittest.ExpectError(err, t)

}

func TestMustBuildWithInvalidQuery(t *testing.T) {

	defer unittest.ExpectPanic("Empty query. must have at least one get/set/delete", t)

	q := pgSQLQuery{}
	q.MustBuild()

}
