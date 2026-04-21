package store

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Postgres struct {
	Pool *pgxpool.Pool
}

type Options struct {
	DatabaseURL          string
	ApplicationName      string
	ConnectTimeout       time.Duration
	MaxConns             int32
	MinConns             int32
	MaxConnLifetime      time.Duration
	MaxConnIdleTime      time.Duration
	HealthCheckPeriod    time.Duration
	DefaultQueryExecMode string
}

func Open(ctx context.Context, options Options) (*Postgres, error) {
	config, err := buildPoolConfig(options)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, config)
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

func buildPoolConfig(options Options) (*pgxpool.Config, error) {
	if strings.TrimSpace(options.DatabaseURL) == "" {
		return nil, errors.New("database url is required")
	}

	config, err := pgxpool.ParseConfig(options.DatabaseURL)
	if err != nil {
		return nil, err
	}

	if options.ApplicationName != "" {
		config.ConnConfig.RuntimeParams["application_name"] = options.ApplicationName
	}

	if options.ConnectTimeout > 0 {
		config.ConnConfig.ConnectTimeout = options.ConnectTimeout
	}

	if options.MaxConns > 0 {
		config.MaxConns = options.MaxConns
	}

	if options.MinConns >= 0 {
		config.MinConns = options.MinConns
	}

	if options.MaxConnLifetime > 0 {
		config.MaxConnLifetime = options.MaxConnLifetime
	}

	if options.MaxConnIdleTime > 0 {
		config.MaxConnIdleTime = options.MaxConnIdleTime
	}

	if options.HealthCheckPeriod > 0 {
		config.HealthCheckPeriod = options.HealthCheckPeriod
	}

	if config.MinConns > config.MaxConns {
		return nil, fmt.Errorf("pool_min_conns cannot exceed pool_max_conns")
	}

	if options.DefaultQueryExecMode != "" {
		config.ConnConfig.DefaultQueryExecMode, err = queryExecMode(options.DefaultQueryExecMode)
		if err != nil {
			return nil, err
		}
	}

	return config, nil
}

func queryExecMode(value string) (pgx.QueryExecMode, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "cache_statement":
		return pgx.QueryExecModeCacheStatement, nil
	case "cache_describe":
		return pgx.QueryExecModeCacheDescribe, nil
	case "describe_exec":
		return pgx.QueryExecModeDescribeExec, nil
	case "exec":
		return pgx.QueryExecModeExec, nil
	case "simple_protocol":
		return pgx.QueryExecModeSimpleProtocol, nil
	default:
		return 0, fmt.Errorf("unsupported query exec mode %q", value)
	}
}
