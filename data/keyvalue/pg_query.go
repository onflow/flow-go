package keyvalue

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-pg/pg"
)

const setParamHolder = ""

type keyTable struct {
	key   string
	table string
}

// pgSQLQuery ..
type pgSQLQuery struct {
	db            *pg.DB
	isTransaction bool
	builtQuery    string
	params        []string
	getKey        keyTable
	setKeys       []keyTable
	setValuePos   []int
	deleteKeys    []keyTable
}

// InTransaction ..
func (q *pgSQLQuery) InTransaction() {
	q.isTransaction = true
}

// Get ..
func (q *pgSQLQuery) Get(table string, key string) {
	q.getKey = keyTable{key: key, table: table}
}

// Set ..
func (q *pgSQLQuery) Set(table string, key string) {
	q.setKeys = append(q.setKeys, keyTable{key: key, table: table})
}

// Delete ..
func (q *pgSQLQuery) Delete(table string, key string) {
	q.deleteKeys = append(q.deleteKeys, keyTable{key: key, table: table})
}

// MustBuild generates the SQL query. Intended to be called once on server initialisation for performance. This method panics if the Query is not supported.
func (q *pgSQLQuery) MustBuild() {
	errorMessages := []string{}

	if !q.hasGet() && !q.hasSet() && !q.hasDelete() {
		errorMessages = append(errorMessages, "Empty query. must have at least one get/set/delete")
	}

	if len(q.setKeys)+len(q.deleteKeys) > 1 && !q.isTransaction {
		errorMessages = append(errorMessages, "Must use a transaction when changing more than one key")
	}

	if len(errorMessages) > 0 {
		panic(errors.New(strings.Join(errorMessages, " ; ")))
	}

	query := ""

	// set
	for _, setKey := range q.setKeys {
		pos := q.addParams(
			setKey.table,
			setKey.key,
			setParamHolder,
		)
		q.setValuePos = append(q.setValuePos, pos[2])
		query += fmt.Sprintf("INSERT INTO ?%d (key, value) VALUES ('?%d', '?%d') ON CONFLICT (key) DO UPDATE SET value = ?%d ; ",
			pos[0],
			pos[1],
			pos[2],
			pos[2],
		)
	}

	// delete
	for _, deleteKey := range q.deleteKeys {
		pos := q.addParams(deleteKey.table, deleteKey.key)
		query += fmt.Sprintf(" DELETE FROM ?%d WHERE key=?%d ; ", pos[0], pos[1])
	}

	// get
	if q.hasGet() {
		pos := q.addParams(q.getKey.table, q.getKey.key)
		query += fmt.Sprintf("SELECT value FROM ?%d WHERE key=?%d ; ", pos[0], pos[1])
	}

	if q.isTransaction {
		query = fmt.Sprintf("BEGIN; %s COMMIT;", query)
	}

	q.builtQuery = query

}

// Execute ..
func (q *pgSQLQuery) Execute(setParams ...string) (string, error) {
	if q.builtQuery == "" {
		return "", errors.New("Cannot execute unbuilt query, call MustBuild() first")
	}

	err := q.mergeSetParams(setParams)
	if err != nil {
		return "", err
	}

	if q.hasGet() {
		var value string
		_, err := q.db.QueryOne(&value, q.builtQuery, q.params)
		return value, err
	}

	_, execErr := q.db.Exec(q.builtQuery, q.params)
	return "", execErr
}

func (q *pgSQLQuery) debug() (string, []string) {
	return q.builtQuery, q.params
}

// and params to query list and returns their relative position
func (q *pgSQLQuery) addParams(params ...string) []int {
	pos := make([]int, 0, len(params))
	startPos := len(q.params)
	for i := range params {
		pos = append(pos, startPos+i)
	}
	q.params = append(q.params, params...)
	return pos
}

// merges q.params that were set at build time with setParams passed at execution time.
func (q *pgSQLQuery) mergeSetParams(setParams []string) error {

	if len(q.setKeys) != len(setParams) {
		return fmt.Errorf("Expected to substituted %d set params, but received %d", len(q.setKeys), len(setParams))
	}

	subs := 0
	for _, pos := range q.setValuePos {
		q.params[pos] = setParams[subs]
		subs++
	}

	return nil
}

func (q *pgSQLQuery) hasGet() bool {
	return q.getKey.key != ""
}

func (q *pgSQLQuery) hasSet() bool {
	return len(q.setKeys) > 0
}

func (q *pgSQLQuery) hasDelete() bool {
	return len(q.deleteKeys) > 0
}
