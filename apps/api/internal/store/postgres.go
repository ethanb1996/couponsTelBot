package store

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	Pool *pgxpool.Pool
}

func Open(ctx context.Context, databaseURL string) (*Postgres, error) {
	if databaseURL == "" {
		return nil, errors.New("database url is required")
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}

	return &Postgres{Pool: pool}, nil
}

func (p *Postgres) Ping(ctx context.Context) error {
	if p == nil || p.Pool == nil {
		return errors.New("postgres pool is not initialized")
	}

	return p.Pool.Ping(ctx)
}

func (p *Postgres) Close() {
	if p == nil || p.Pool == nil {
		return
	}

	p.Pool.Close()
}
