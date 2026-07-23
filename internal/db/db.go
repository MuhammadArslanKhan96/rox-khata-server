package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// InitPool initializes a PostgreSQL connection pool with optimized configurations.
func InitPool(ctx context.Context, dbURL string) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		return nil, err
	}

	// Tuning connection pool for production throughput
	config.MaxConns = 50
	config.MinConns = 10
	config.MaxConnIdleTime = 15 * time.Minute
	config.MaxConnLifetime = 1 * time.Hour

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}

	return pool, nil
}
