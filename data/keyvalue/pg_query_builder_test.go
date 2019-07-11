package keyvalue

import (
	"testing"

	"github.com/dapperlabs/bamboo-node/utils/unittest"
	. "github.com/onsi/gomega"
)

func TestGet(t *testing.T) {
	RegisterTestingT(t)

	table := "tableA"
	key := "keyA"

	q := (&pgQueryBuilder{}).
		AddGet(table).
		MustBuild()
	err := q.(*pgQueryBuilder).mergeParams([]string{key})
	Expect(err).ToNot(HaveOccurred())
	query, params := q.(*pgQueryBuilder).debug()

	Expect(query).To(Equal("SELECT value FROM ?0 WHERE key=?1 ; "))
	Expect(params).To(Equal([]string{table, key}))
}

func TestSet(t *testing.T) {
	RegisterTestingT(t)

	table := "tableA"
	key := "keyA"
	value := "valueA"

	q := (&pgQueryBuilder{}).
		AddSet(table).
		MustBuild()
	err := q.(*pgQueryBuilder).mergeParams([]string{key, value})
	Expect(err).ToNot(HaveOccurred())
	query, params := q.(*pgQueryBuilder).debug()

	Expect(query).To(Equal("INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?2 ; "))
	Expect(params).To(Equal([]string{table, key, value}))
}

func TestSetWithExtraSetParams(t *testing.T) {
	RegisterTestingT(t)

	table := "tableA"
	key := "keyA"
	value := "valueA"

	q := (&pgQueryBuilder{}).
		AddSet(table).
		MustBuild()
	err := q.(*pgQueryBuilder).mergeParams([]string{key, value, "extra"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(Equal("Expected to substituted 2 params, but received 3"))
}

func TestSetWithMissingSetParams(t *testing.T) {
	RegisterTestingT(t)

	table := "tableA"
	key := "keyA"
	//value := "valueA"

	q := (&pgQueryBuilder{}).
		AddSet(table).
		MustBuild()
	err := q.(*pgQueryBuilder).mergeParams([]string{key})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(Equal("Expected to substituted 2 params, but received 1"))
}

func TestMultiSetNoTx(t *testing.T) {

	table1 := "tableA"
	// key1 := "keyA"
	// value1 := "valueA"

	table2 := "tableB"
	// key2 := "keyB"
	// value2 := "valueB"

	defer unittest.ExpectPanic("Must use a transaction when changing more than one key", t)
	(&pgQueryBuilder{}).
		AddSet(table1).
		AddSet(table2).
		MustBuild()

}

func TestMultiSetWithTx(t *testing.T) {
	RegisterTestingT(t)

	table1 := "tableA"
	key1 := "keyA"
	value1 := "valueA"

	table2 := "tableB"
	key2 := "keyB"
	value2 := "valueB"

	q := (&pgQueryBuilder{}).
		AddSet(table1).
		AddSet(table2).
		InTransaction().
		MustBuild()
	err := q.(*pgQueryBuilder).mergeParams([]string{key1, value1, key2, value2})
	Expect(err).ToNot(HaveOccurred())
	query, params := q.(*pgQueryBuilder).debug()

	Expect(query).To(Equal("BEGIN; INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?2 ; INSERT INTO ?3 (key, value) VALUES ('?4', '?5') ON CONFLICT (key) DO UPDATE SET value = ?5 ;  COMMIT;"))
	Expect(params).To(Equal([]string{table1, key1, value1, table2, key2, value2}))
}

func TestMustBuildWithInvalidQuery(t *testing.T) {

	defer unittest.ExpectPanic("Empty query. must have at least one get/set/delete", t)
	(&pgQueryBuilder{}).
		MustBuild()

}
