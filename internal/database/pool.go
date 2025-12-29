package database

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool wraps pgxpool.Pool and provides access to sqlc Queries
type Pool struct {
	*pgxpool.Pool
	queries *Queries
}

// NewPool creates a new database pool from a connection string
func NewPool(ctx context.Context, connString string) (*Pool, error) {
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		return nil, fmt.Errorf("failed to create pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &Pool{
		Pool:    pool,
		queries: New(pool),
	}, nil
}

// Queries returns the sqlc-generated queries interface
func (p *Pool) Queries() *Queries {
	return p.queries
}

// Close closes the database pool
func (p *Pool) Close() {
	p.Pool.Close()
}

// WithTx executes a function within a transaction
func (p *Pool) WithTx(ctx context.Context, fn func(*Queries) error) error {
	tx, err := p.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if err := fn(p.queries.WithTx(tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
