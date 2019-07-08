package keyvalue

import (
	"testing"

	"github.com/dapperlabs/bamboo-node/utils/unittest"
	. "github.com/onsi/gomega"
)

func TestGet(t *testing.T) {
	gomega := NewWithT(t)

	table := "tableA"
	key := "keyA"

	q := (&pgSQLQuery{}).
		Get(table, key).
		MustBuild()
	query, params := q.(*pgSQLQuery).debug()

	gomega.Expect(query).To(Equal("SELECT value FROM ?0 WHERE key=?1 ; "))
	gomega.Expect(params).To(Equal([]string{table, key}))
}

func TestSet(t *testing.T) {
	gomega := NewWithT(t)

	table := "tableA"
	key := "keyA"
	value := "valueA"

	q := (&pgSQLQuery{}).
		Set(table, key).
		MustBuild()
	err := q.(*pgSQLQuery).mergeSetParams([]string{value})
	gomega.Expect(err).ToNot(HaveOccurred())
	query, params := q.(*pgSQLQuery).debug()

	gomega.Expect(query).To(Equal("INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?2 ; "))
	gomega.Expect(params).To(Equal([]string{table, key, value}))
}

func TestSetWithExtraSetParams(t *testing.T) {
	gomega := NewWithT(t)

	table := "tableA"
	key := "keyA"
	value := "valueA"

	q := (&pgSQLQuery{}).
		Set(table, key).
		MustBuild()
	err := q.(*pgSQLQuery).mergeSetParams([]string{value, "extra"})
	gomega.Expect(err).To(HaveOccurred())
	gomega.Expect(err.Error()).To(Equal("Expected to substituted 1 set params, but received 2"))
}

func TestSetWithMissingSetParams(t *testing.T) {
	gomega := NewWithT(t)

	table := "tableA"
	key := "keyA"
	//value := "valueA"

	q := (&pgSQLQuery{}).
		Set(table, key).
		MustBuild()
	err := q.(*pgSQLQuery).mergeSetParams([]string{})
	gomega.Expect(err).To(HaveOccurred())
	gomega.Expect(err.Error()).To(Equal("Expected to substituted 1 set params, but received 0"))
}

func TestMultiSetNoTx(t *testing.T) {

	table1 := "tableA"
	key1 := "keyA"
	// value1 := "valueA"

	table2 := "tableB"
	key2 := "keyB"
	// value2 := "valueB"

	defer unittest.ExpectPanic("Must use a transaction when changing more than one key", t)
	(&pgSQLQuery{}).
		Set(table1, key1).
		Set(table2, key2).
		MustBuild()

}

func TestMultiSetWithTx(t *testing.T) {
	gomega := NewWithT(t)

	table1 := "tableA"
	key1 := "keyA"
	value1 := "valueA"

	table2 := "tableB"
	key2 := "keyB"
	value2 := "valueB"

	q := (&pgSQLQuery{}).
		Set(table1, key1).
		Set(table2, key2).
		InTransaction().
		MustBuild()
	err := q.(*pgSQLQuery).mergeSetParams([]string{value1, value2})
	gomega.Expect(err).ToNot(HaveOccurred())
	query, params := q.(*pgSQLQuery).debug()

	gomega.Expect(query).To(Equal("BEGIN; INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?2 ; INSERT INTO ?3 (key, value) VALUES ('?4', '?5') ON CONFLICT (key) DO UPDATE SET value = ?5 ;  COMMIT;"))
	gomega.Expect(params).To(Equal([]string{table1, key1, value1, table2, key2, value2}))
}

func TestMustBuildBeforeExecute(t *testing.T) {
	gomega := NewWithT(t)

	table := "tableA"
	key := "keyA"

	q := (&pgSQLQuery{}).
		Get(table, key)
	_, err := q.Execute()
	gomega.Expect(err).To(HaveOccurred())

}

func TestMustBuildWithInvalidQuery(t *testing.T) {

	defer unittest.ExpectPanic("Empty query. must have at least one get/set/delete", t)
	(&pgSQLQuery{}).
		MustBuild()

}
