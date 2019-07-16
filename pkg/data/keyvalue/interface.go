// Package keyvalue provides an abstract interface for connecting to DBs and building/executing key-value queries on them.
/*
Some examples:

```go
  dbConn := NewPostgresDB(options...)

  // Get Query
  result, err := dbConn.GetQuery.Execute("foo_collection", "key1")

  // Set Query
  _, err := dbConn.SetQuery.Execute("foo_collection", "key1", "value1")

  // Delete Query
  _, err := dbConn.DeleteQuery.Execute("foo_collection", "key1")

  // Custom query with transaction
  set2Keys := dbConn.NewQueryBuilder().
	  AddSet().
	  AddSet().
	  InTransaction().
	  MustBuild()

  _, err := set2Keys.Execute("foo_collection", "key1", "value1", "bar_collection", "key2", "value2")
```
*/
package keyvalue

// DBConnector abstracts a database connection.
type DBConnector interface {
	// NewQueryBuilder returns an instance of a new QueryBuilder. Intended to be used when building a custom multi statement query
	NewQueryBuilder() QueryBuilder
	// GetQuery returns a pre-built QueryBuilder instance ready to be executed as a get statement
	GetQuery() Query
	// SetQuery returns a pre-built QueryBuilder instance ready to be executed as a get statement
	SetQuery() Query
	// DeleteQuery returns a pre-built QueryBuilder instance ready to be executed as a delete statement
	DeleteQuery() Query
	// MigrateUp performs all the steps required to bring the backing DB into an initialised state
	MigrateUp() error
	// MigrateDown is the inverse of MigrateUp and intended to be used in testing environment to achieve a "clean slate".
	MigrateDown() error
}

// QueryBuilder builds a key value query.
type QueryBuilder interface {
	// InTransaction sets a query to run in a multi statement transaction
	InTransaction() QueryBuilder
	// AddGet adds a get statement
	AddGet() QueryBuilder
	// AddSet adds a set statement
	AddSet() QueryBuilder
	// AddDelete adds a delete statement
	AddDelete() QueryBuilder
	// MustBuild is intended to be called once per query on server startup for performance considerations of some providers.
	MustBuild() Query
}

// Query provides a way to execute a query
type Query interface {
	// Execute runs the query and returns its result
	Execute(params ...string) (result string, err error)
}
