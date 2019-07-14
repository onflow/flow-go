package keyvalue

import (
	"errors"
	"fmt"
	"strings"

	"github.com/go-pg/pg"
)

type statement int

const (
	getStatement statement = iota
	setStatement
	deleteStatement
)

// pgQueryBuilder implements the QueryBuilder and Query interfaces.
type pgQueryBuilder struct {
	db            *pg.DB
	isTransaction bool
	paramCount    int
	statements    []statement
	hasGet        bool // could be derived with getStatementsCount(), but stored for Execute() optimisation
}

// InTransaction sets a query to run in a multi-statement transaction.
func (q *pgQueryBuilder) InTransaction() QueryBuilder {
	q.isTransaction = true
	return q
}

// AddGet adds a get statement to the query.
func (q *pgQueryBuilder) AddGet() QueryBuilder {
	q.statements = append(q.statements, getStatement)
	return q
}

// AddSet adds a set statement to the query.
func (q *pgQueryBuilder) AddSet() QueryBuilder {
	q.statements = append(q.statements, setStatement)
	return q
}

// AddDelete adds a delete statement to the query.
func (q *pgQueryBuilder) AddDelete() QueryBuilder {
	q.statements = append(q.statements, deleteStatement)
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
		switch statement {

		case getStatement:
			pos := q.addParams(2)
			query += fmt.Sprintf("SELECT value FROM ?%d WHERE key=?%d ; ", pos[0], pos[1])

		case setStatement:
			pos := q.addParams(3)
			query += fmt.Sprintf("INSERT INTO ?%d (key, value) VALUES ('?%d', '?%d') ON CONFLICT (key) DO UPDATE SET value = ?%d ; ", pos[0], pos[1], pos[2], pos[2])

		case deleteStatement:
			pos := q.addParams(2)
			query += fmt.Sprintf(" DELETE FROM ?%d WHERE key=?%d ; ", pos[0], pos[1])

		default:
			panic(errors.New("Not a known get/set/delete statement"))
		}
	}

	if q.isTransaction {
		query = fmt.Sprintf("BEGIN; %s COMMIT;", query)
	}

	return &pgQuery{q.db, query, getCount != 0, q.paramCount}
}

// addParams increases the query's param count and returns the added params relative position.
func (q *pgQueryBuilder) addParams(count int) []int {
	pos := make([]int, 0, count)
	for i := 0; i < count; i++ {
		pos = append(pos, q.paramCount+i)
	}
	q.paramCount += count
	return pos
}

func (q *pgQueryBuilder) getStatementsCount() int {
	count := 0
	for _, s := range q.statements {
		if s == getStatement {
			count++
		}
	}
	return count
}

func (q *pgQueryBuilder) setStatementsCount() int {
	count := 0
	for _, s := range q.statements {
		if s == setStatement {
			count++
		}
	}
	return count
}

func (q *pgQueryBuilder) deleteStatementsCount() int {
	count := 0
	for _, s := range q.statements {
		if s == deleteStatement {
			count++
		}
	}
	return count
}
