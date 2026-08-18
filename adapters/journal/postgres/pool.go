package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Pool wraps a PostgreSQL connection pool.
type Pool struct {
	db     *sql.DB
	config poolConfig
}

type poolConfig struct {
	connString      string
	maxOpenConns    int
	maxIdleConns    int
	connMaxLifetime time.Duration
}

// NewPool creates a new PostgreSQL connection pool.
func NewPool(ctx context.Context, connString string) (*Pool, error) {
	db, err := sql.Open("pgx", connString)
	if err != nil {
		return nil, fmt.Errorf("postgres open: %w", err)
	}

	pool := &Pool{
		db: db,
		config: poolConfig{
			connString:      connString,
			maxOpenConns:    25,
			maxIdleConns:    5,
			connMaxLifetime: 5 * time.Minute,
		},
	}

	db.SetMaxOpenConns(pool.config.maxOpenConns)
	db.SetMaxIdleConns(pool.config.maxIdleConns)
	db.SetConnMaxLifetime(pool.config.connMaxLifetime)

	// Verify connectivity.
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres ping: %w", err)
	}

	return pool, nil
}

// Close closes the underlying database connection.
func (p *Pool) Close() error {
	return p.db.Close()
}

// QueryContext executes a query and returns rows.
func (p *Pool) QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return p.db.QueryContext(ctx, query, args...)
}

// QueryRowContext executes a query and returns a single row.
func (p *Pool) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	return p.db.QueryRowContext(ctx, query, args...)
}

// ExecContext executes a query without returning rows.
func (p *Pool) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return p.db.ExecContext(ctx, query, args...)
}

// BeginTx starts a transaction.
func (p *Pool) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return p.db.BeginTx(ctx, nil)
}
