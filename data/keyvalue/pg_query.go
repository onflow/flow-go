package keyvalue

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-pg/pg"
)

type keyTable struct {
	key   string
	table string
}

type keyValue struct {
	keyTable keyTable
	value    string
}

// pgSQLQuery ..
type pgSQLQuery struct {
	db            *pg.DB
	isTransaction bool
	builtQuery    string
	params        []string
	getKey        keyTable
	setKeyValues  []keyValue
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
func (q *pgSQLQuery) Set(table string, key string, value string) {
	kv := keyValue{
		keyTable: keyTable{key: key, table: table},
		value:    value,
	}
	q.setKeyValues = append(q.setKeyValues, kv)
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

	if len(q.setKeyValues)+len(q.deleteKeys) > 1 && !q.isTransaction {
		errorMessages = append(errorMessages, "Must use a transaction when changing more than one key")
	}

	if len(errorMessages) > 0 {
		panic(errors.New(strings.Join(errorMessages, " ; ")))
	}

	query := ""

	// set
	for _, setKeyValue := range q.setKeyValues {
		pos := q.addParams(
			setKeyValue.keyTable.table,
			setKeyValue.keyTable.key,
			setKeyValue.value,
		)
		query += fmt.Sprintf("INSERT INTO %s (key, value) VALUES ('%s', '%s') ON CONFLICT (key) DO UPDATE SET value = %s ; ",
			pos[0],
			pos[1],
			pos[2],
			pos[2],
		)
	}

	// delete
	for _, deleteKey := range q.deleteKeys {
		pos := q.addParams(deleteKey.table, deleteKey.key)
		query += fmt.Sprintf(" DELETE FROM %s WHERE key=%s ; ", pos[0], pos[1])
	}

	// get
	if q.hasGet() {
		pos := q.addParams(q.getKey.table, q.getKey.key)
		query += fmt.Sprintf("SELECT value FROM %s WHERE key=%s ; ", pos[0], pos[1])
	}

	if q.isTransaction {
		query = fmt.Sprintf("BEGIN; %s COMMIT;", query)
	}

	q.builtQuery = query

}

// Execute ..
func (q *pgSQLQuery) Execute() (string, error) {
	if q.builtQuery == "" {
		return "", errors.New("Cannot execute unbuilt query, call MustBuild() first")
	}

	if q.hasGet() {
		var value string
		_, err := q.db.QueryOne(&value, q.builtQuery, q.params)
		return value, err
	}

	_, err := q.db.Exec(q.builtQuery, q.params)
	return "", err
}

func (q *pgSQLQuery) debug() (string, []string) {
	return q.builtQuery, q.params
}

// returns the relative position of the new added params
func (q *pgSQLQuery) addParams(params ...string) []string {
	pos := make([]string, 0)
	startPos := len(q.params)
	for i := range params {
		pos = append(pos, fmt.Sprintf("?%d", startPos+i))
	}
	q.params = append(q.params, params...)
	return pos
}

func (q *pgSQLQuery) hasGet() bool {
	return q.getKey.key != ""
}

func (q *pgSQLQuery) hasSet() bool {
	return len(q.setKeyValues) > 0
}

func (q *pgSQLQuery) hasDelete() bool {
	return len(q.deleteKeys) > 0
}
