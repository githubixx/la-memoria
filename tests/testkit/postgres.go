package testkit

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

type PostgreSQL struct {
	ConnectionString string
	container        *postgres.PostgresContainer
}

func StartPostgreSQL(ctx context.Context, test *testing.T) *PostgreSQL {
	test.Helper()
	testcontainers.SkipIfProviderIsNotHealthy(test)

	container, err := postgres.Run(
		ctx,
		"postgres:18.6",
		postgres.WithDatabase("bookmarker"),
		postgres.WithUsername("bookmarker"),
		postgres.WithPassword("bookmarker-test"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		test.Fatalf("start PostgreSQL container: %v", err)
	}

	connectionString, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		test.Cleanup(func() { _ = container.Terminate(ctx) })
		test.Fatalf("get PostgreSQL connection string: %v", err)
	}

	database := &PostgreSQL{
		ConnectionString: connectionString,
		container:        container,
	}
	test.Cleanup(func() {
		if err := database.container.Terminate(ctx); err != nil {
			test.Errorf("terminate PostgreSQL container: %v", err)
		}
	})
	return database
}

func (database *PostgreSQL) ApplyMigration(ctx context.Context, migrationPath string) error {
	statement, err := os.ReadFile(migrationPath)
	if err != nil {
		return fmt.Errorf("read migration %q: %w", migrationPath, err)
	}

	connection, err := pgx.Connect(ctx, database.ConnectionString)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer connection.Close(ctx)

	if _, err := connection.Exec(ctx, string(statement)); err != nil {
		return fmt.Errorf("execute migration %q: %w", migrationPath, err)
	}
	return nil
}

func MigrationPath(fileName string) string {
	return filepath.Join("..", "..", "migrations", fileName)
}

// Connect returns a raw pgx connection, closed automatically on cleanup.
func (database *PostgreSQL) Connect(ctx context.Context, test *testing.T) *pgx.Conn {
	test.Helper()
	connection, err := pgx.Connect(ctx, database.ConnectionString)
	if err != nil {
		test.Fatalf("connect to PostgreSQL: %v", err)
	}
	test.Cleanup(func() { _ = connection.Close(ctx) })
	return connection
}
