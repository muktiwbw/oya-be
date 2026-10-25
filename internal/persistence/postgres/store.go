// Package postgres provides PostgreSQL persistence using pgxpool and sqlc.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"oya-be/internal/persistence/postgres/sqlc"
)

type Store struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

type ConnectionConfig struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
}

func Open(ctx context.Context, connection ConnectionConfig) (*Store, error) {
	cfg, err := pgxpool.ParseConfig("sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("create postgres config: %w", err)
	}
	cfg.ConnConfig.Host = connection.Host
	cfg.ConnConfig.Port = uint16(connection.Port)
	cfg.ConnConfig.Database = connection.Database
	cfg.ConnConfig.User = connection.User
	cfg.ConnConfig.Password = connection.Password

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to postgres: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return &Store{
		pool:    pool,
		queries: sqlc.New(pool),
	}, nil
}

func (s *Store) Close() error {
	s.pool.Close()
	return nil
}

func (s *Store) Ready(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

func (s *Store) Queries() *sqlc.Queries {
	return s.queries
}
