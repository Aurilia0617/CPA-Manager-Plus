package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type Dialect string

const (
	DialectSQLite   Dialect = "sqlite"
	DialectPostgres Dialect = "postgres"
)

func NormalizeDialect(value string) Dialect {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "postgres", "postgresql", "pgx":
		return DialectPostgres
	default:
		return DialectSQLite
	}
}

func Rebind(dialect Dialect, query string) string {
	if dialect != DialectPostgres {
		return query
	}
	var builder strings.Builder
	builder.Grow(len(query) + 8)
	index := 1
	for i := 0; i < len(query); i++ {
		if query[i] == '?' {
			builder.WriteString(fmt.Sprintf("$%d", index))
			index++
			continue
		}
		builder.WriteByte(query[i])
	}
	return builder.String()
}

func QueryContext(ctx context.Context, db *sql.DB, dialect Dialect, query string, args ...any) (*sql.Rows, error) {
	return db.QueryContext(ctx, Rebind(dialect, query), args...)
}

func QueryRowContext(ctx context.Context, db *sql.DB, dialect Dialect, query string, args ...any) *sql.Row {
	return db.QueryRowContext(ctx, Rebind(dialect, query), args...)
}

func ExecContext(ctx context.Context, db *sql.DB, dialect Dialect, query string, args ...any) (sql.Result, error) {
	return db.ExecContext(ctx, Rebind(dialect, query), args...)
}

func PrepareContext(ctx context.Context, tx *sql.Tx, dialect Dialect, query string) (*sql.Stmt, error) {
	return tx.PrepareContext(ctx, Rebind(dialect, query))
}
