package keyvalue

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-pg/pg"
)

const paramHolder = ""

// pgQueryBuilder implements the QueryBuilder and Query interfaces.
type pgQueryBuilder struct {
	db              *pg.DB
	isTransaction   bool
	builtQuery      string
	params          []string
	statements      []statement
	paramsHolderPos []int
	hasGet          bool // could be derived with getStatementsCount(), but stored for Execute() optimisation
}

type statement interface {
	getTable() string
}

type getStatement string
type setStatement string
type deleteStatement string

func (s getStatement) getTable() string {
	return string(s)
}

func (s setStatement) getTable() string {
	return string(s)
}

func (s deleteStatement) getTable() string {
	return string(s)
}

// InTransaction sets a query to run in a multi-statement transaction.
func (q *pgQueryBuilder) InTransaction() QueryBuilder {
	q.isTransaction = true
	return q
}

// AddGet adds a get statement to the query.
func (q *pgQueryBuilder) AddGet(table string) QueryBuilder {
	q.statements = append(q.statements, getStatement(table))
	return q
}

// AddSet adds a set statement to the query.
func (q *pgQueryBuilder) AddSet(table string) QueryBuilder {
	q.statements = append(q.statements, setStatement(table))
	return q
}

// AddDelete adds a delete statement to the query.
func (q *pgQueryBuilder) AddDelete(table string) QueryBuilder {
	q.statements = append(q.statements, deleteStatement(table))
	return q
}

// MustBuild generates the SQL query and panics if the query is invalid.
//
// For performance reasons, this function is intended to be called once on server initialization.
func (q *pgQueryBuilder) MustBuild() Query {
	errorMessages := []string{}

	getCount := q.getStatementsCount()
	setCount := q.setStatementsCount()
	deleteCount := q.deleteStatementsCount()

	// check if query is supported
	if getCount == 0 && setCount == 0 && deleteCount == 0 {
		errorMessages = append(errorMessages, "Empty query. must have at least one get/set/delete")
	}

	if setCount+deleteCount > 1 && !q.isTransaction {
		errorMessages = append(errorMessages, "Must use a transaction when changing more than one key")
	}

	if len(errorMessages) > 0 {
		panic(errors.New(strings.Join(errorMessages, " ; ")))
	}

	// query is supported- begin building
	query := ""

	for _, statement := range q.statements {
		switch v := statement.(type) {

		case getStatement:
			pos := q.addParams(v.getTable(), paramHolder)
			q.paramsHolderPos = append(q.paramsHolderPos, pos[1])
			query += fmt.Sprintf("SELECT value FROM ?%d WHERE key=?%d ; ", pos[0], pos[1])

		case setStatement:
			pos := q.addParams(v.getTable(), paramHolder, paramHolder)
			q.paramsHolderPos = append(q.paramsHolderPos, pos[1], pos[2])
			query += fmt.Sprintf("INSERT INTO ?%d (key, value) VALUES ('?%d', '?%d') ON CONFLICT (key) DO UPDATE SET value = ?%d ; ", pos[0], pos[1], pos[2], pos[2])

		case deleteStatement:
			pos := q.addParams(v.getTable(), paramHolder)
			q.paramsHolderPos = append(q.paramsHolderPos, pos[1])
			query += fmt.Sprintf(" DELETE FROM ?%d WHERE key=?%d ; ", pos[0], pos[1])

		default:
			panic(errors.New("Not a known get/set/delete statement"))
		}
	}

	if q.isTransaction {
		query = fmt.Sprintf("BEGIN; %s COMMIT;", query)
	}

	q.hasGet = getCount != 0
	q.builtQuery = query
	return q
}

// Execute runs the query and returns its result.
func (q *pgQueryBuilder) Execute(params ...string) (string, error) {
	if q.builtQuery == "" {
		return "", errors.New("Cannot execute unbuilt query, call MustBuild() first")
	}

	err := q.mergeParams(params)
	if err != nil {
		return "", err
	}

	if q.hasGet {
		var value string
		_, err := q.db.QueryOne(&value, q.builtQuery, q.params)
		return value, err
	}

	_, execErr := q.db.Exec(q.builtQuery, q.params)
	return "", execErr
}

func (q *pgQueryBuilder) debug() (string, []string) {
	return q.builtQuery, q.params
}

// addParams adds parameters to the query list and returns their relative position.
func (q *pgQueryBuilder) addParams(params ...string) []int {
	pos := make([]int, 0, len(params))
	startPos := len(q.params)
	for i := range params {
		pos = append(pos, startPos+i)
	}
	q.params = append(q.params, params...)
	return pos
}

// mergesParams merges the execution-time and build-time parameters.
func (q *pgQueryBuilder) mergeParams(params []string) error {

	if len(q.paramsHolderPos) != len(params) {
		return fmt.Errorf("Expected to substituted %d params, but received %d", len(q.paramsHolderPos), len(params))
	}

	subs := 0
	for _, pos := range q.paramsHolderPos {
		q.params[pos] = params[subs]
		subs++
	}

	return nil
}

func (q *pgQueryBuilder) getStatementsCount() int {
	count := 0
	for _, s := range q.statements {
		if _, ok := s.(getStatement); ok {
			count++
		}
	}
	return count
}

func (q *pgQueryBuilder) setStatementsCount() int {
	count := 0
	for _, s := range q.statements {
		if _, ok := s.(setStatement); ok {
			count++
		}
	}
	return count
}

func (q *pgQueryBuilder) deleteStatementsCount() int {
	count := 0
	for _, s := range q.statements {
		if _, ok := s.(deleteStatement); ok {
			count++
		}
	}
	return count
}
