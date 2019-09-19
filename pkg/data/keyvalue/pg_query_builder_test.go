package keyvalue

import (
	"testing"

	"github.com/dapperlabs/flow-go/pkg/utils/unittest"
	. "github.com/onsi/gomega"
)

func TestGet(t *testing.T) {
	RegisterTestingT(t)

	q := (&pgQueryBuilder{}).
		AddGet().
		MustBuild()

	query, err := q.(*pgQuery).debug([]string{"table", "key"})
	Expect(err).ToNot(HaveOccurred())
	Expect(query).To(Equal("SELECT value FROM ?0 WHERE key=?1 ; "))
}

func TestSet(t *testing.T) {
	RegisterTestingT(t)

	q := (&pgQueryBuilder{}).
		AddSet().
		MustBuild()

	query, err := q.(*pgQuery).debug([]string{"table", "key", "value"})
	Expect(err).ToNot(HaveOccurred())
	Expect(query).To(Equal("INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?2 ; "))
}

func TestSetWithExtraSetParams(t *testing.T) {
	RegisterTestingT(t)

	q := (&pgQueryBuilder{}).
		AddSet().
		MustBuild()

	query, err := q.(*pgQuery).debug([]string{"table", "key", "value", "extra"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(Equal("Expected to substituted 3 params, but received 4"))
	Expect(query).To(Equal(""))
}

func TestSetWithMissingSetParams(t *testing.T) {
	RegisterTestingT(t)

	q := (&pgQueryBuilder{}).
		AddSet().
		MustBuild()

	query, err := q.(*pgQuery).debug([]string{"table", "key"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(Equal("Expected to substituted 3 params, but received 2"))
	Expect(query).To(Equal(""))
}

func TestMultiSetNoTx(t *testing.T) {

	defer unittest.ExpectPanic("Must use a transaction when changing more than one key", t)
	(&pgQueryBuilder{}).
		AddSet().
		AddSet().
		MustBuild()

}

func TestMultiSetWithTx(t *testing.T) {
	RegisterTestingT(t)

	q := (&pgQueryBuilder{}).
		AddSet().
		AddSet().
		InTransaction().
		MustBuild()

	query, err := q.(*pgQuery).debug([]string{"table1", "key1", "value1", "table2", "key2", "value2"})
	Expect(err).ToNot(HaveOccurred())
	Expect(query).To(Equal("BEGIN; INSERT INTO ?0 (key, value) VALUES ('?1', '?2') ON CONFLICT (key) DO UPDATE SET value = ?2 ; INSERT INTO ?3 (key, value) VALUES ('?4', '?5') ON CONFLICT (key) DO UPDATE SET value = ?5 ;  COMMIT;"))
}

func TestMustBuildWithInvalidQuery(t *testing.T) {

	defer unittest.ExpectPanic("Empty query. must have at least one get/set/delete", t)
	(&pgQueryBuilder{}).
		MustBuild()

}
