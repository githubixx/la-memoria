package postgres

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/githubixx/la-memoria/bookmarker/model"
)

// DSN builds a PostgreSQL connection string from resolved configuration.
func DSN(configuration model.Configuration) string {
	database := configuration.Database
	values := url.Values{}
	if database.TLSMode != "" {
		values.Set("sslmode", database.TLSMode)
	}
	address := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(database.User, database.Password),
		Host:     fmt.Sprintf("%s:%d", database.Host, database.Port),
		Path:     "/" + database.Name,
		RawQuery: values.Encode(),
	}
	return address.String()
}

// NewPool constructs a connection pool for the given PostgreSQL connection string.
func NewPool(ctx context.Context, connectionString string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, connectionString)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return pool, nil
}

// WithTransaction runs fn inside a PostgreSQL transaction, committing on success
// and rolling back if fn returns an error or panics.
func WithTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// SchemaVersion reports the currently applied migration version, or an empty
// string if no migration has been applied yet.
func SchemaVersion(ctx context.Context, pool *pgxpool.Pool) (string, error) {
	var version string
	err := pool.QueryRow(ctx, "SELECT version FROM schema_migrations").Scan(&version)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) && pgErr.SQLState() == "42P01" { // undefined_table
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read schema version: %w", err)
	}
	return version, nil
}

// RunMigrations applies every "*.up.sql" file in migrationsDir, in lexical
// order, when no migration has been applied yet.
func RunMigrations(ctx context.Context, pool *pgxpool.Pool, migrationsDir string) (string, error) {
	current, err := SchemaVersion(ctx, pool)
	if err != nil {
		return "", err
	}
	if current != "" {
		return current, nil
	}
	entries, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil {
		return "", fmt.Errorf("list migrations: %w", err)
	}
	sort.Strings(entries)
	for _, path := range entries {
		statement, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read migration %q: %w", path, err)
		}
		if _, err := pool.Exec(ctx, string(statement)); err != nil {
			return "", fmt.Errorf("apply migration %q: %w", path, err)
		}
	}
	return SchemaVersion(ctx, pool)
}
